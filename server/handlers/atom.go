// V4 curator endpoints — dispute & resolve atoms.
//
// Flow:
//   1. Any logged-in user can flag an Atom via POST /api/v4/atom/:id/dispute
//      → Atom.Status flips from "accepted" to "disputed".
//   2. A curator (admin) resolves via POST /api/v4/atom/:id/resolve with
//      body {action: "accept" | "reject", note}.
//      → "accept" puts it back to AtomStatusAccepted (and keeps galaxy
//        counters intact);
//      → "reject" flips to AtomStatusRejected and decrements counters.
//
// We do NOT delete rows — audit trail matters for the on-chain Merkle
// roll-up in Phase 2.
package handlers

import (
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─── POST /api/v4/atom/:id/dispute ───────────────────────────────────────────

type atomDisputeReq struct {
	Reason string `json:"reason"`
}

// AtomDispute lets a logged-in user flag an accepted atom for curator review.
// Idempotent: flagging an already-disputed atom is a no-op.
func AtomDispute(c *gin.Context) {
	user := resolveV4User(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}
	atomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req atomDisputeReq
	_ = c.ShouldBindJSON(&req) // reason is optional

	var atom models.Atom
	if err := database.DB.First(&atom, "id = ?", atomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "atom not found"})
		return
	}
	if atom.Status == models.AtomStatusDisputed {
		c.JSON(http.StatusOK, gin.H{"atom": atom, "already_disputed": true})
		return
	}
	if atom.Status != models.AtomStatusAccepted {
		c.JSON(http.StatusConflict, gin.H{"error": "atom is " + atom.Status + ", cannot dispute"})
		return
	}

	atom.Status = models.AtomStatusDisputed
	if err := database.DB.Save(&atom).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// NOTE: we intentionally do NOT decrement Galaxy.AtomCount/EdgeCount on
	// dispute — only on a curator's final "reject". A disputed atom is
	// quarantined, not deleted.
	c.JSON(http.StatusOK, gin.H{"atom": atom})
}

// ─── POST /api/v4/atom/:id/resolve  (admin) ──────────────────────────────────

type atomResolveReq struct {
	Action string `json:"action" binding:"required"` // "accept" | "reject"
	Note   string `json:"note"`
}

// AtomResolve is the curator decision endpoint. Wrapped in AuthAdmin() at
// the router layer — handler trusts that.
func AtomResolve(c *gin.Context) {
	atomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req atomResolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "accept" && action != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be accept|reject"})
		return
	}

	var atom models.Atom
	if err := database.DB.First(&atom, "id = ?", atomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "atom not found"})
		return
	}
	if atom.Status != models.AtomStatusDisputed {
		c.JSON(http.StatusConflict, gin.H{"error": "atom not in disputed state"})
		return
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if action == "accept" {
			atom.Status = models.AtomStatusAccepted
			return tx.Save(&atom).Error
		}
		// reject: flip + decrement counters on the galaxy.
		atom.Status = models.AtomStatusRejected
		if err := tx.Save(&atom).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"atom_count": gorm.Expr("GREATEST(atom_count - 1, 0)"),
		}
		switch atom.Kind {
		case "node":
			updates["node_count"] = gorm.Expr("GREATEST(node_count - 1, 0)")
		case "edge":
			updates["edge_count"] = gorm.Expr("GREATEST(edge_count - 1, 0)")
		}
		return tx.Model(&models.Galaxy{}).
			Where("id = ?", atom.GalaxyID).
			Updates(updates).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"atom": atom})
}

// ─── GET /api/v4/credits/me ──────────────────────────────────────────────────

// CreditsMe returns the current user's credit balance + recent ledger rows.
// Reuses V3 services.GetCreditsInfo so the monthly auto-reset still kicks in.
func CreditsMe(c *gin.Context) {
	user := resolveV4User(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}
	var ledger []models.CreditLedger
	database.DB.Where("user_id = ?", user.ID).
		Order("created_at DESC").
		Limit(50).
		Find(&ledger)

	// Fresh balance read (avoids serving stale User.Credits if a concurrent
	// debit just landed).
	var fresh models.User
	database.DB.Select("credits", "credits_reset", "pro_expires_at").
		First(&fresh, "id = ?", user.ID)

	c.JSON(http.StatusOK, gin.H{
		"credits":        fresh.Credits,
		"credits_reset":  fresh.CreditsReset,
		"pro_expires_at": fresh.ProExpiresAt,
		"ledger":         ledger,
	})
}
