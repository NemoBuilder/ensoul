package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// HolderDashboard handles GET /api/holder/dashboard
// Returns the holder's revenue dashboard.
func HolderDashboard(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	data, err := services.GetHolderDashboard(walletAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, data)
}

// HolderRevenuePeriod handles GET /api/holder/revenue/:period
// Returns detailed revenue for a specific period.
func HolderRevenuePeriod(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	period := c.Param("period")
	if period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period is required"})
		return
	}

	revenues, err := services.GetRevenueForPeriod(walletAddr, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"revenues": revenues})
}
