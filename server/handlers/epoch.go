// V4 Epoch admin endpoints.
//
// In Phase 2.x these will be triggered by a cron tick; for now we expose a
// manual POST so curators/admins can roll one epoch at a time during
// testnet.
package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services/epoch"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ensoul-labs/ensoul-server/util/merkle"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EpochBuild rolls up pending atoms into a new epoch.
//
// Body: { "galaxy_slug": "optional — empty = global cross-galaxy epoch" }
func EpochBuild(c *gin.Context) {
	var req struct {
		GalaxySlug string `json:"galaxy_slug"`
	}
	_ = c.ShouldBindJSON(&req)

	var galaxyID uuid.UUID
	if req.GalaxySlug != "" {
		var g models.Galaxy
		if err := database.DB.Select("id").Where("slug = ?", req.GalaxySlug).First(&g).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
			return
		}
		galaxyID = g.ID
	}

	res, err := epoch.Build(c.Request.Context(), database.DB, galaxyID)
	if err != nil {
		if errors.Is(err, epoch.ErrNoAtoms) {
			c.JSON(http.StatusOK, gin.H{"skipped": true, "reason": "no pending atoms"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fire-and-forget on-chain push; epoch.PushOnChain handles its own
	// status updates and degrades gracefully if the registry is unconfigured.
	go func(epochID, gID uuid.UUID, idx int64, root merkle.Hash, n int) {
		if err := epoch.PushOnChain(context.Background(), database.DB, epochID, gID, idx, root, n); err != nil {
			util.Log.Warn("[epoch] async chain push: %v", err)
		}
	}(res.EpochID, res.GalaxyID, res.Index, res.Root, res.AtomCount)

	c.JSON(http.StatusOK, gin.H{
		"epoch_id":   res.EpochID,
		"index":      res.Index,
		"root":       res.Root.HashHex(),
		"atom_count": res.AtomCount,
	})
}

// EpochList returns the most recent epochs (global + per-galaxy).
// Public read; useful for the explorer "Latest Roll-ups" widget.
func EpochList(c *gin.Context) {
	var rows []models.Epoch
	q := database.DB.Order("created_at DESC").Limit(50)
	if slug := c.Query("galaxy_slug"); slug != "" {
		var g models.Galaxy
		if err := database.DB.Select("id").Where("slug = ?", slug).First(&g).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
			return
		}
		q = q.Where("galaxy_id = ?", g.ID)
	}
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"epochs": rows})
}

// ─── GET /api/v4/epoch/:id ───────────────────────────────────────────────────
//
// EpochGet returns one epoch row + its atoms (id, kind, label, merkle_leaf).
// Used by the explorer detail page.
func EpochGet(c *gin.Context) {
	epochID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var e models.Epoch
	if err := database.DB.First(&e, "id = ?", epochID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "epoch not found"})
		return
	}
	type leafRow struct {
		ID         uuid.UUID `json:"id"`
		Kind       string    `json:"kind"`
		NodeLabel  string    `json:"node_label"`
		EdgeLabel  string    `json:"edge_label"`
		MerkleLeaf string    `json:"merkle_leaf"`
	}
	var atoms []leafRow
	database.DB.Model(&models.Atom{}).
		Where("epoch_id = ?", epochID).
		Order("created_at ASC, id ASC").
		Find(&atoms)
	c.JSON(http.StatusOK, gin.H{"epoch": e, "atoms": atoms})
}

// ─── GET /api/v4/atom/:id/proof ──────────────────────────────────────────────
//
// AtomProof rebuilds the Merkle tree for the atom's epoch and returns the
// proof path so the frontend can verify locally against the on-chain root.
//
//	{
//	  "epoch": { ... },
//	  "leaf": "0xabc…",
//	  "index": 17,
//	  "path": ["0x…", "0x…", …]   // sibling hashes bottom→top
//	}
func AtomProof(c *gin.Context) {
	atomID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	var atom models.Atom
	if err := database.DB.First(&atom, "id = ?", atomID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "atom not found"})
		return
	}
	if atom.EpochID == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "atom not yet rolled into an epoch"})
		return
	}

	// Fetch the entire epoch's atom set in the same canonical order Build()
	// used, so the rebuilt leaves match the stored root byte-for-byte.
	var ep models.Epoch
	if err := database.DB.First(&ep, "id = ?", *atom.EpochID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "epoch not found"})
		return
	}

	res, err := epoch.RebuildProof(c.Request.Context(), database.DB, ep.ID, atom.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"epoch": ep,
		"leaf":  res.Leaf,
		"index": res.Index,
		"path":  res.Path,
	})
}
