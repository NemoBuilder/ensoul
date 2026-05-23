// V4 fair-launch HTTP endpoints.
//
// All mutating endpoints are admin-only — fair launches need curator
// approval since they move real BNB. Public reads expose status + launch
// row so the UI can render the raising / claim / refund widget.
package handlers

import (
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services/launch"
	"github.com/gin-gonic/gin"
)

// ─── GET /api/v4/launch/:slug ────────────────────────────────────────────────

// LaunchGet returns the launch row for one galaxy slug (404 if none).
func LaunchGet(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	var g models.Galaxy
	if err := database.DB.Select("id, slug, stage").Where("slug = ?", slug).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}
	var L models.Launch
	if err := database.DB.Where("galaxy_id = ?", g.ID).First(&L).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no launch"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"launch": L, "galaxy_stage": g.Stage})
}

// ─── POST /api/v4/launch/:slug/open  (admin) ─────────────────────────────────

type launchOpenReq struct {
	StartAt      int64  `json:"start_at"`       // unix seconds
	EndAt        int64  `json:"end_at"`         // unix seconds
	MinRaiseWei  string `json:"min_raise_wei"`  // decimal string
	MaxRaiseWei  string `json:"max_raise_wei"`  // "" or "0" = uncapped
	SupplyWei    string `json:"supply_wei"`     // decimal string
	TokenName    string `json:"token_name"`
	TokenSymbol  string `json:"token_symbol"`
}

func parseBig(s string) (*big.Int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0), true
	}
	v, ok := new(big.Int).SetString(s, 10)
	return v, ok
}

// LaunchOpen — admin opens the fair-launch window for one galaxy.
func LaunchOpen(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	var g models.Galaxy
	if err := database.DB.Select("id").Where("slug = ?", slug).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}
	var req launchOpenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	minR, ok1 := parseBig(req.MinRaiseWei)
	maxR, ok2 := parseBig(req.MaxRaiseWei)
	supply, ok3 := parseBig(req.SupplyWei)
	if !ok1 || !ok2 || !ok3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad numeric field"})
		return
	}
	row, err := launch.Open(c.Request.Context(), launch.OpenParams{
		GalaxyID:    g.ID,
		StartAt:     time.Unix(req.StartAt, 0),
		EndAt:       time.Unix(req.EndAt, 0),
		MinRaiseWei: minR,
		MaxRaiseWei: maxR,
		SupplyWei:   supply,
		TokenName:   strings.TrimSpace(req.TokenName),
		TokenSymbol: strings.TrimSpace(req.TokenSymbol),
	})
	if err != nil {
		c.JSON(launchErrStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"launch": row})
}

// ─── POST /api/v4/launch/:slug/token  (admin) ────────────────────────────────

type launchTokenReq struct {
	TokenAddr string `json:"token_addr"`
}

// LaunchSetToken — admin wires a deployed EnsoulCommunityToken address.
func LaunchSetToken(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	var g models.Galaxy
	if err := database.DB.Select("id").Where("slug = ?", slug).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}
	var req launchTokenReq
	if err := c.ShouldBindJSON(&req); err != nil || req.TokenAddr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_addr required"})
		return
	}
	row, err := launch.WireToken(c.Request.Context(), g.ID, req.TokenAddr)
	if err != nil {
		c.JSON(launchErrStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"launch": row})
}

// ─── POST /api/v4/launch/:slug/finalize  (admin) ─────────────────────────────

// LaunchFinalize — admin closes the launch + triggers BNB split / claim phase.
func LaunchFinalize(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	var g models.Galaxy
	if err := database.DB.Select("id").Where("slug = ?", slug).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}
	row, err := launch.Finalize(c.Request.Context(), g.ID)
	if err != nil {
		c.JSON(launchErrStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"launch": row})
}

// launchErrStatus maps service errors to HTTP codes.
func launchErrStatus(err error) int {
	switch err {
	case launch.ErrNotFound:
		return http.StatusNotFound
	case launch.ErrAlreadyExists, launch.ErrTokenAlreadySet, launch.ErrWrongStatus:
		return http.StatusConflict
	case launch.ErrNotLaunchReady, launch.ErrBadWindow, launch.ErrBadAmounts, launch.ErrFounderNoWallet:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
