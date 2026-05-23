// V4 fair-launch — depositor-facing read endpoints.
//
// LaunchMyDeposit: GET /api/v4/launch/:slug/deposit?addr=0x...
// Returns the caller's recorded deposit row + projected token share if the
// launch has succeeded and a supply_wei is known. Pure read; the actual claim
// is a wallet-signed call against the FairLaunch contract.
package handlers

import (
	"math/big"
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/gin-gonic/gin"
)

func LaunchMyDeposit(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	addr := strings.ToLower(strings.TrimSpace(c.Query("addr")))
	if addr == "" || !strings.HasPrefix(addr, "0x") || len(addr) != 42 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "addr query param required (0x... 42 chars)"})
		return
	}
	var g models.Galaxy
	if err := database.DB.Select("id").Where("slug = ?", slug).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}
	var L models.Launch
	if err := database.DB.Where("galaxy_id = ?", g.ID).First(&L).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no launch"})
		return
	}
	var dep models.LaunchDeposit
	err := database.DB.Where("launch_id = ? AND wallet_addr = ?", L.ID, addr).First(&dep).Error
	out := gin.H{
		"launch_id":          L.ID,
		"wallet_addr":        addr,
		"amount_wei":         "0",
		"claimed":            false,
		"refunded":           false,
		"projected_tokens":   "0",
		"projected_ratio_pp": "0",
	}
	if err == nil {
		out["amount_wei"] = dep.AmountWei
		out["claimed"] = dep.Claimed
		out["refunded"] = dep.Refunded
	}

	// Projection: depositors split (supply * userDeposit / totalRaised) on success.
	if L.Status == models.LaunchStatusFinalSucc {
		userAmt, _ := new(big.Int).SetString(out["amount_wei"].(string), 10)
		totalRaised, _ := new(big.Int).SetString(L.TotalRaisedWei, 10)
		supply, _ := new(big.Int).SetString(L.SupplyWei, 10)
		if userAmt != nil && totalRaised != nil && supply != nil &&
			userAmt.Sign() > 0 && totalRaised.Sign() > 0 {
			share := new(big.Int).Mul(supply, userAmt)
			share.Div(share, totalRaised)
			out["projected_tokens"] = share.String()
			// ratio in basis-points-of-percent (10000 = 100%); avoid float.
			rp := new(big.Int).Mul(userAmt, big.NewInt(10000))
			rp.Div(rp, totalRaised)
			out["projected_ratio_pp"] = rp.String()
		}
	}
	c.JSON(http.StatusOK, out)
}
