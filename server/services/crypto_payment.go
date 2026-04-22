package services

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// MaxProMonths bounds N-month bulk purchases.
const MaxProMonths = 24

// NormalizeMonths clamps months into the allowed [1, MaxProMonths] range.
func NormalizeMonths(m int) int {
	if m < 1 {
		return 1
	}
	if m > MaxProMonths {
		return MaxProMonths
	}
	return m
}

// ProCryptoQuote is returned by Quote().
type ProCryptoQuote struct {
	Months          int    `json:"months"`             // 1..24
	PricePerMonthUSDT string `json:"price_per_month_usdt"` // unit price, e.g. "49"
	PriceUSDT       string `json:"price_usdt"`         // total: price_per_month * months
	USDTWei         string `json:"usdt_wei"`           // total wei
	BNBWei          string `json:"bnb_wei"`            // estimated BNB (with buffer)
	BNBHuman        string `json:"bnb_human"`          // formatted "0.0823"
	BufferBPS       int    `json:"buffer_bps"`         // 150 = 1.5%
	RecipientAddr   string `json:"recipient_addr"`
	USDTContract    string `json:"usdt_contract"`
	ChainID         int    `json:"chain_id"`           // 56 BSC mainnet
	DurationDays    int    `json:"duration_days"`      // total days = months * cfg.ProDurationDays
	ExpiresAt       int64  `json:"expires_at"`         // unix sec, quote validity (60s)
}

// ProCryptoQuoteNow returns current pricing + on-chain BNB equivalent for the
// given number of months (1..24, clamped).
func ProCryptoQuoteNow(ctx context.Context, months int) (*ProCryptoQuote, error) {
	cfg := config.Cfg
	if cfg.PaymentRecipient == "" {
		return nil, fmt.Errorf("payment recipient not configured")
	}

	months = NormalizeMonths(months)
	perMonth := big.NewInt(int64(cfg.ProPriceUSDT))
	totalUSDT := new(big.Int).Mul(perMonth, big.NewInt(int64(months)))
	usdtWei := new(big.Int).Mul(totalUSDT, exp10(18))

	q := &ProCryptoQuote{
		Months:            months,
		PricePerMonthUSDT: perMonth.String(),
		PriceUSDT:         totalUSDT.String(),
		USDTWei:           usdtWei.String(),
		BufferBPS:         cfg.ProBNBQuoteBufferBPS,
		RecipientAddr:     cfg.PaymentRecipient,
		USDTContract:      cfg.USDTAddr,
		ChainID:           56,
		DurationDays:      cfg.ProDurationDays * months,
		ExpiresAt:         time.Now().Add(60 * time.Second).Unix(),
	}

	// Quote USDT → BNB then add buffer (so user pays slightly more than spot).
	bnbWei, err := chain.GetUSDTToBNBQuote(ctx, usdtWei)
	if err != nil {
		// Quote unavailable → return without BNB option (frontend can hide BNB).
		util.Log.Warn("[crypto-pay] BNB quote failed: %v", err)
		return q, nil
	}
	// Apply buffer: bnb * (10000 + bps) / 10000
	buf := new(big.Int).Mul(bnbWei, big.NewInt(int64(10000+cfg.ProBNBQuoteBufferBPS)))
	buf.Div(buf, big.NewInt(10000))
	q.BNBWei = buf.String()
	q.BNBHuman = formatWei(buf, 18, 5)
	return q, nil
}

// SubmitCryptoPayment records a pending payment for a user.
// `months` controls how many Pro months this payment grants (1..24).
// Returns the CryptoPayment row (status may be pending/confirmed/rejected after first verify pass).
func SubmitCryptoPayment(ctx context.Context, userID uuid.UUID, txHash, expectedToken string, months int) (*models.CryptoPayment, error) {
	txHash = strings.ToLower(strings.TrimSpace(txHash))
	if !strings.HasPrefix(txHash, "0x") || len(txHash) != 66 {
		return nil, fmt.Errorf("invalid tx_hash format")
	}
	expectedToken = strings.ToUpper(strings.TrimSpace(expectedToken))
	if expectedToken != "USDT" && expectedToken != "BNB" {
		return nil, fmt.Errorf("expected_token must be USDT or BNB")
	}
	months = NormalizeMonths(months)

	// Idempotency: same tx_hash → return existing.
	var existing models.CryptoPayment
	if err := database.DB.First(&existing, "tx_hash = ?", txHash).Error; err == nil {
		return &existing, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("db: %w", err)
	}

	cfg := config.Cfg
	row := &models.CryptoPayment{
		UserID:    userID,
		Chain:     "bsc",
		Token:     expectedToken,
		ToAddr:    strings.ToLower(cfg.PaymentRecipient),
		TxHash:    txHash,
		AmountWei: "0",
		Months:    months,
		Status:    chain.PayStatusPending,
	}
	if expectedToken == "USDT" {
		row.TokenAddr = strings.ToLower(cfg.USDTAddr)
	}
	if err := database.DB.Create(row).Error; err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	// Best-effort first verify pass.
	if err := verifyAndApply(ctx, row); err != nil {
		util.Log.Warn("[crypto-pay] initial verify failed (will retry): %v", err)
	}
	return row, nil
}

// GetCryptoPayment returns a payment row by id, ensuring it belongs to userID.
func GetCryptoPayment(userID, paymentID uuid.UUID) (*models.CryptoPayment, error) {
	var row models.CryptoPayment
	if err := database.DB.First(&row, "id = ? AND user_id = ?", paymentID, userID).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// GetCryptoPaymentRefreshed returns a payment row, re-verifying on-chain if it
// is still pending. This is what the user-facing status poll uses, so
// confirmations propagate within the next polling tick rather than waiting for
// the 5-minute background cron.
func GetCryptoPaymentRefreshed(ctx context.Context, userID, paymentID uuid.UUID) (*models.CryptoPayment, error) {
	row, err := GetCryptoPayment(userID, paymentID)
	if err != nil {
		return nil, err
	}
	if row.Status == chain.PayStatusPending {
		if err := verifyAndApply(ctx, row); err != nil {
			util.Log.Warn("[crypto-pay] status-refresh verify id=%s err=%v", row.ID, err)
		}
	}
	return row, nil
}

// ListCryptoPaymentsForUser returns recent payments for a user, newest first.
func ListCryptoPaymentsForUser(userID uuid.UUID, limit int) ([]models.CryptoPayment, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var rows []models.CryptoPayment
	err := database.DB.Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

// RefreshPendingPayments re-verifies all pending payments. Called by cron.
func RefreshPendingPayments(ctx context.Context) {
	var rows []models.CryptoPayment
	if err := database.DB.Where("status = ?", chain.PayStatusPending).
		Order("created_at ASC").Limit(100).Find(&rows).Error; err != nil {
		util.Log.Warn("[crypto-pay] cron load failed: %v", err)
		return
	}
	for i := range rows {
		row := &rows[i]
		// Time out after 1 hour.
		if time.Since(row.CreatedAt) > time.Hour {
			database.DB.Model(row).Updates(map[string]interface{}{
				"status":        chain.PayStatusRejected,
				"reject_reason": chain.RejectNotFound,
			})
			util.Log.Info("[crypto-pay] timed out payment id=%s tx=%s", row.ID, row.TxHash)
			continue
		}
		if err := verifyAndApply(ctx, row); err != nil {
			util.Log.Warn("[crypto-pay] verify id=%s err=%v", row.ID, err)
		}
	}
}

// StartCryptoPaymentRefresh launches a background ticker that re-verifies
// pending crypto payments at the given interval.
func StartCryptoPaymentRefresh(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		// Initial pass after a short delay so chain client has time to init.
		time.Sleep(30 * time.Second)
		RefreshPendingPayments(context.Background())
		for range ticker.C {
			RefreshPendingPayments(context.Background())
		}
	}()
	util.Log.Info("[crypto-pay] background refresher started (every %s)", interval)
}

// verifyAndApply runs chain.VerifyPayment and applies the result.
// On confirmed, also extends the user's pro_expires_at and writes audit info.
func verifyAndApply(ctx context.Context, row *models.CryptoPayment) error {
	cfg := config.Cfg
	if cfg.PaymentRecipient == "" {
		return fmt.Errorf("recipient not configured")
	}
	months := row.Months
	if months < 1 {
		months = 1 // legacy rows pre-N-month migration
	}
	usdtWei := new(big.Int).Mul(
		big.NewInt(int64(cfg.ProPriceUSDT)*int64(months)),
		exp10(18),
	)

	info, err := chain.VerifyPayment(
		ctx,
		common.HexToHash(row.TxHash),
		common.HexToAddress(cfg.PaymentRecipient),
		row.Token,
		usdtWei,
		cfg.ProBNBVerifyToleranceBPS,
		uint64(cfg.PaymentMinConfirm),
	)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"status":        info.Status,
		"confirmations": int(info.Confirmations),
		"block_number":  info.BlockNumber,
	}
	if info.From != (common.Address{}) {
		updates["from_addr"] = strings.ToLower(info.From.Hex())
	}
	if info.Amount != nil {
		updates["amount_wei"] = info.Amount.String()
	}
	if info.USDTEquivalent != nil {
		// Format with 18 decimals as numeric string.
		updates["paid_usdt_equivalent"] = formatWei(info.USDTEquivalent, 18, 18)
	}
	if info.RejectReason != "" {
		updates["reject_reason"] = info.RejectReason
	}

	if info.Status == chain.PayStatusConfirmed {
		// Grant Pro to the user.
		var user models.User
		if err := database.DB.First(&user, "id = ?", row.UserID).Error; err != nil {
			return fmt.Errorf("user not found: %w", err)
		}
		now := time.Now().UTC()
		base := now
		if user.ProExpiresAt != nil && user.ProExpiresAt.After(now) {
			base = *user.ProExpiresAt
		}
		newExp := base.AddDate(0, 0, cfg.ProDurationDays*months)
		userUpdates := map[string]interface{}{
			"pro_expires_at": &newExp,
			"pro_source":     "crypto",
		}
		// Reset credits to Pro quota if currently below.
		if user.Credits < ProCreditsPerMonth {
			userUpdates["credits"] = ProCreditsPerMonth
			userUpdates["credits_reset"] = time.Now().Truncate(24 * time.Hour).AddDate(0, 1, 0)
		}
		if err := database.DB.Model(&user).Updates(userUpdates).Error; err != nil {
			return fmt.Errorf("grant pro: %w", err)
		}
		updates["pro_granted_until"] = &newExp
		nowVerified := time.Now().UTC()
		updates["verified_at"] = &nowVerified
		util.Log.Info("[crypto-pay] confirmed user=%s token=%s amount=%s tx=%s pro_until=%s",
			row.UserID, row.Token, row.AmountWei, row.TxHash, newExp.Format(time.RFC3339))
	}

	if err := database.DB.Model(row).Updates(updates).Error; err != nil {
		return fmt.Errorf("update payment: %w", err)
	}
	// Refresh in-memory copy.
	database.DB.First(row, "id = ?", row.ID)
	return nil
}

// exp10 returns 10**n as *big.Int.
func exp10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// formatWei renders wei as a decimal string with `decimals` decimal places,
// truncated to `display` digits after the point.
func formatWei(wei *big.Int, decimals, display int) string {
	if wei == nil {
		return "0"
	}
	div := exp10(decimals)
	intPart := new(big.Int).Quo(wei, div)
	frac := new(big.Int).Mod(wei, div)
	fracStr := fmt.Sprintf("%0*s", decimals, frac.String())
	if display < decimals && display >= 0 {
		fracStr = fracStr[:display]
	}
	if display == 0 {
		return intPart.String()
	}
	return intPart.String() + "." + fracStr
}
