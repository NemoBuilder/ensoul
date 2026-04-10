package handlers

import (
	"context"
	"math/big"
	"net/http"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
)

// EconomyOverviewResponse is the aggregated economy dashboard payload.
type EconomyOverviewResponse struct {
	// High-level stats
	TotalSouls        int64   `json:"total_souls"`
	TotalFragments    int64   `json:"total_fragments"`
	TotalSubscribers  int64   `json:"total_subscribers"`
	TotalMiningPayout float64 `json:"total_mining_payout"`

	// Token info (on-chain)
	TokenInfo TokenInfo `json:"token_info"`

	// Mining pool
	MiningPool MiningPoolSnapshot `json:"mining_pool"`

	// Buyback summary
	Buyback BuybackSummary `json:"buyback"`

	// Revenue pools
	RevenuePools []models.RevenuePool `json:"revenue_pools"`

	// Recent buyback history
	BuybackHistory []models.BuybackRecord `json:"buyback_history"`

	// Revenue split config (for the flow diagram)
	SplitConfig SplitConfig `json:"split_config"`

	// Last buyback amounts (for flow arrows)
	LastBuyback *LastBuybackInfo `json:"last_buyback"`

	// Wallet balances (on-chain, for flow nodes)
	Wallets WalletBalances `json:"wallets"`
}

// TokenInfo holds on-chain token metadata.
type TokenInfo struct {
	TotalSupply  float64 `json:"total_supply"`
	TokenAddress string  `json:"token_address"`
}

// WalletBalances holds live on-chain balances for each wallet in the flywheel.
type WalletBalances struct {
	BuybackBNB       float64 `json:"buyback_bnb"`
	BuybackToken     float64 `json:"buyback_token"`
	BuybackAddr      string  `json:"buyback_addr"`
	MiningPoolToken  float64 `json:"mining_pool_token"`
	MiningPoolAddr   string  `json:"mining_pool_addr"`
	RevenuePoolToken float64 `json:"revenue_pool_token"`
	RevenuePoolAddr  string  `json:"revenue_pool_addr"`
	TreasuryAddr     string  `json:"treasury_addr"`
}

// LastBuybackInfo describes the most recent buyback for flow diagram annotations.
type LastBuybackInfo struct {
	Source      string  `json:"source"`
	BNBAmount   float64 `json:"bnb_amount"`
	TokenAmount float64 `json:"token_amount"`
	CreatedAt   string  `json:"created_at"`
}

// MiningPoolSnapshot is a point-in-time view of the mining pool.
type MiningPoolSnapshot struct {
	Balance        float64 `json:"balance"`
	TotalDeposited float64 `json:"total_deposited"`
	TotalReleased  float64 `json:"total_released"`
	DailyLimit     float64 `json:"daily_limit"`
	DailyReleased  float64 `json:"daily_released"`
	DailyRemaining float64 `json:"daily_remaining"`
	Paused         bool    `json:"paused"`
}

// BuybackSummary aggregates all buyback records.
type BuybackSummary struct {
	TotalBNBSpent    float64 `json:"total_bnb_spent"`
	TotalTokenBought float64 `json:"total_token_bought"`
	TotalOperations  int64   `json:"total_operations"`
	MintRevenueBNB   float64 `json:"mint_revenue_bnb"`
	MintRevenueToken float64 `json:"mint_revenue_token"`
	SubRevenueBNB    float64 `json:"sub_revenue_bnb"`
	SubRevenueToken  float64 `json:"sub_revenue_token"`
}

// SplitConfig describes the flywheel revenue split ratios.
type SplitConfig struct {
	MintBuybackPct     int `json:"mint_buyback_pct"`      // 60
	MintTreasuryPct    int `json:"mint_treasury_pct"`     // 10
	MintRevenuePoolPct int `json:"mint_revenue_pool_pct"` // 30
	SubBuybackPct      int `json:"sub_buyback_pct"`       // 40
	SubTreasuryPct     int `json:"sub_treasury_pct"`      // 10
	SubRevenuePoolPct  int `json:"sub_revenue_pool_pct"`  // 50
}

// weiToFloat18 converts a *big.Int (18 decimals) to float64.
func weiToFloat18(wei *big.Int) float64 {
	if wei == nil {
		return 0
	}
	f := new(big.Float).SetInt(wei)
	divisor := new(big.Float).SetFloat64(1e18)
	result, _ := new(big.Float).Quo(f, divisor).Float64()
	return result
}

// EconomyOverview handles GET /api/economy/overview
// Returns a comprehensive view of the entire economic flywheel.
func EconomyOverview(c *gin.Context) {
	var resp EconomyOverviewResponse
	ctx := context.Background()

	// 1. Total souls (minted shells)
	database.DB.Model(&models.Shell{}).Count(&resp.TotalSouls)

	// 2. Total fragments
	database.DB.Model(&models.Fragment{}).Count(&resp.TotalFragments)

	// 3. Active subscribers
	database.DB.Model(&models.Subscription{}).
		Where("status = ? AND expires_at > ?", models.SubStatusActive, time.Now()).
		Count(&resp.TotalSubscribers)

	// 4. Mining pool snapshot
	poolStatus, err := services.GetPoolStatus()
	if err == nil {
		if v, ok := poolStatus["balance"].(float64); ok {
			resp.MiningPool.Balance = v
		}
		if v, ok := poolStatus["total_deposited"].(float64); ok {
			resp.MiningPool.TotalDeposited = v
		}
		if v, ok := poolStatus["total_released"].(float64); ok {
			resp.MiningPool.TotalReleased = v
		}
		if v, ok := poolStatus["daily_limit"].(float64); ok {
			resp.MiningPool.DailyLimit = v
		}
		if v, ok := poolStatus["daily_released"].(float64); ok {
			resp.MiningPool.DailyReleased = v
		}
		if v, ok := poolStatus["daily_remaining"].(float64); ok {
			resp.MiningPool.DailyRemaining = v
		}
		if v, ok := poolStatus["paused"].(bool); ok {
			resp.MiningPool.Paused = v
		}
	}
	resp.TotalMiningPayout = resp.MiningPool.TotalReleased

	// 5. Buyback aggregates
	database.DB.Model(&models.BuybackRecord{}).Count(&resp.Buyback.TotalOperations)

	var totalAgg struct {
		BNB   float64
		Token float64
	}
	database.DB.Model(&models.BuybackRecord{}).
		Select("COALESCE(SUM(bnb_amount), 0) as bnb, COALESCE(SUM(token_amount), 0) as token").
		Scan(&totalAgg)
	resp.Buyback.TotalBNBSpent = totalAgg.BNB
	resp.Buyback.TotalTokenBought = totalAgg.Token

	var mintAgg struct {
		BNB   float64
		Token float64
	}
	database.DB.Model(&models.BuybackRecord{}).
		Where("source = ?", "mint_revenue").
		Select("COALESCE(SUM(bnb_amount), 0) as bnb, COALESCE(SUM(token_amount), 0) as token").
		Scan(&mintAgg)
	resp.Buyback.MintRevenueBNB = mintAgg.BNB
	resp.Buyback.MintRevenueToken = mintAgg.Token

	var subAgg struct {
		BNB   float64
		Token float64
	}
	database.DB.Model(&models.BuybackRecord{}).
		Where("source = ?", "subscription_revenue").
		Select("COALESCE(SUM(bnb_amount), 0) as bnb, COALESCE(SUM(token_amount), 0) as token").
		Scan(&subAgg)
	resp.Buyback.SubRevenueBNB = subAgg.BNB
	resp.Buyback.SubRevenueToken = subAgg.Token

	// 6. Revenue pools (all months, newest first)
	database.DB.Order("period DESC").Limit(12).Find(&resp.RevenuePools)

	// 7. Recent buyback history
	records, err := services.GetBuybackHistory(20)
	if err == nil {
		resp.BuybackHistory = records
	}

	// 8. Split config
	resp.SplitConfig = SplitConfig{
		MintBuybackPct:     60,
		MintTreasuryPct:    10,
		MintRevenuePoolPct: 30,
		SubBuybackPct:      40,
		SubTreasuryPct:     10,
		SubRevenuePoolPct:  50,
	}

	// 9. Last buyback info (for flow diagram arrows)
	var lastRecord models.BuybackRecord
	if err := database.DB.Order("created_at DESC").First(&lastRecord).Error; err == nil {
		resp.LastBuyback = &LastBuybackInfo{
			Source:      lastRecord.Source,
			BNBAmount:   lastRecord.BNBAmount,
			TokenAmount: lastRecord.TokenAmount,
			CreatedAt:   lastRecord.CreatedAt.Format(time.RFC3339),
		}
	}

	// 10. Token info — on-chain (best-effort, don't fail the whole response)
	resp.TokenInfo.TokenAddress = config.Cfg.EnsoulTokenAddr
	if supply, err := chain.GetTokenTotalSupply(ctx); err == nil {
		resp.TokenInfo.TotalSupply = weiToFloat18(supply)
	} else {
		util.Log.Debug("[economy] Failed to fetch totalSupply: %v", err)
	}

	// 11. Wallet balances — on-chain (best-effort)
	resp.Wallets.TreasuryAddr = config.Cfg.TreasuryAddr

	if config.Cfg.BuybackPrivateKey != "" {
		if buybackKey, err := chain.ParsePrivateKey(config.Cfg.BuybackPrivateKey); err == nil {
			addr := chain.AddressFromKey(buybackKey).Hex()
			resp.Wallets.BuybackAddr = addr
			if bal, err := chain.GetBNBBalance(ctx, addr); err == nil {
				resp.Wallets.BuybackBNB = weiToFloat18(bal)
			}
			if bal, err := chain.GetTokenBalance(ctx, addr); err == nil {
				resp.Wallets.BuybackToken = weiToFloat18(bal)
			}
		}
	}

	if config.Cfg.MiningPoolPrivateKey != "" {
		if key, err := chain.ParsePrivateKey(config.Cfg.MiningPoolPrivateKey); err == nil {
			addr := chain.AddressFromKey(key).Hex()
			resp.Wallets.MiningPoolAddr = addr
			if bal, err := chain.GetTokenBalance(ctx, addr); err == nil {
				resp.Wallets.MiningPoolToken = weiToFloat18(bal)
			}
		}
	}

	if config.Cfg.RevenuePoolPrivateKey != "" {
		if key, err := chain.ParsePrivateKey(config.Cfg.RevenuePoolPrivateKey); err == nil {
			addr := chain.AddressFromKey(key).Hex()
			resp.Wallets.RevenuePoolAddr = addr
			if bal, err := chain.GetTokenBalance(ctx, addr); err == nil {
				resp.Wallets.RevenuePoolToken = weiToFloat18(bal)
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}
