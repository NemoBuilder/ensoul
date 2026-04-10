package services

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// MinWithdrawAmount is the minimum $Ensoul withdrawal.
	MinWithdrawAmount = 100.0
	// MinGasBNB is the minimum BNB required for a token transfer (~0.0005 BNB).
	MinGasBNB = 0.0005
	// WithdrawCooldown is the minimum time between withdrawals per Claw.
	WithdrawCooldown = 24 * time.Hour
)

// WithdrawStatus holds the pre-flight check result for a withdrawal.
type WithdrawStatus struct {
	ClawWallet   string  `json:"claw_wallet"`
	UserWallet   string  `json:"user_wallet"`
	TokenBalance float64 `json:"token_balance"` // $Ensoul on-chain
	BNBBalance   float64 `json:"bnb_balance"`   // BNB for gas
	Withdrawable float64 `json:"withdrawable"`  // earnings - withdrawn
	HasGas       bool    `json:"has_gas"`
	MinGas       float64 `json:"min_gas"` // BNB needed
	MinAmount    float64 `json:"min_amount"`
	CanWithdraw  bool    `json:"can_withdraw"`
	Reason       string  `json:"reason,omitempty"` // why can't withdraw
}

// CheckWithdraw performs pre-flight checks for a withdrawal.
func CheckWithdraw(clawID uuid.UUID, userWallet string) (*WithdrawStatus, error) {
	var claw models.Claw
	if err := database.DB.Where("id = ?", clawID).First(&claw).Error; err != nil {
		return nil, fmt.Errorf("claw not found")
	}

	if claw.WalletAddr == "" {
		return nil, fmt.Errorf("claw has no wallet")
	}

	status := &WithdrawStatus{
		ClawWallet:   claw.WalletAddr,
		UserWallet:   userWallet,
		Withdrawable: claw.Earnings - claw.Withdrawn,
		MinGas:       MinGasBNB,
		MinAmount:    MinWithdrawAmount,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Check BNB balance (for gas)
	bnbBal, err := chain.GetBNBBalance(ctx, claw.WalletAddr)
	if err != nil {
		util.Log.Error("[withdraw] Failed to get BNB balance for %s: %v", claw.WalletAddr, err)
	} else {
		// Convert from wei to BNB
		bnbFloat, _ := new(big.Float).Quo(
			new(big.Float).SetInt(bnbBal),
			new(big.Float).SetFloat64(1e18),
		).Float64()
		status.BNBBalance = bnbFloat
	}

	// Check token balance
	tokenBal, err := chain.GetTokenBalance(ctx, claw.WalletAddr)
	if err != nil {
		util.Log.Error("[withdraw] Failed to get token balance for %s: %v", claw.WalletAddr, err)
	} else {
		tokenFloat, _ := new(big.Float).Quo(
			new(big.Float).SetInt(tokenBal),
			new(big.Float).SetFloat64(1e18),
		).Float64()
		status.TokenBalance = tokenFloat
	}

	status.HasGas = status.BNBBalance >= MinGasBNB

	// Determine if withdrawal is possible
	if status.Withdrawable < MinWithdrawAmount {
		status.Reason = fmt.Sprintf("minimum withdrawal is %.0f $Ensoul (available: %.4f)", MinWithdrawAmount, status.Withdrawable)
	} else if !status.HasGas {
		status.Reason = fmt.Sprintf("need at least %.4f BNB for gas (current: %.6f BNB)", MinGasBNB, status.BNBBalance)
	} else if status.TokenBalance < MinWithdrawAmount {
		status.Reason = fmt.Sprintf("on-chain token balance too low (%.4f)", status.TokenBalance)
	} else {
		// Check cooldown
		var lastWithdraw models.WithdrawRecord
		err := database.DB.Where("claw_id = ? AND status IN ?", clawID,
			[]string{models.WithdrawStatusConfirmed, models.WithdrawStatusSent, models.WithdrawStatusPending}).
			Order("created_at DESC").First(&lastWithdraw).Error
		if err == nil && time.Since(lastWithdraw.CreatedAt) < WithdrawCooldown {
			nextAt := lastWithdraw.CreatedAt.Add(WithdrawCooldown)
			status.Reason = fmt.Sprintf("cooldown: next withdrawal after %s", nextAt.Format("15:04 UTC"))
		} else {
			status.CanWithdraw = true
		}
	}

	return status, nil
}

// ExecuteWithdraw initiates a withdrawal from a Claw wallet to the user's wallet.
func ExecuteWithdraw(clawID uuid.UUID, userWallet string, amount float64) (*models.WithdrawRecord, error) {
	// Validate amount
	if amount < MinWithdrawAmount {
		return nil, fmt.Errorf("minimum withdrawal is %.0f $Ensoul", MinWithdrawAmount)
	}

	// Load claw
	var claw models.Claw
	if err := database.DB.Where("id = ?", clawID).First(&claw).Error; err != nil {
		return nil, fmt.Errorf("claw not found")
	}

	// Check withdrawable balance
	withdrawable := claw.Earnings - claw.Withdrawn
	if amount > withdrawable {
		return nil, fmt.Errorf("insufficient withdrawable balance (available: %.4f)", withdrawable)
	}

	// Round amount to 4 decimal places to avoid floating point issues
	amount = math.Floor(amount*10000) / 10000

	// Check cooldown
	var lastWithdraw models.WithdrawRecord
	err := database.DB.Where("claw_id = ? AND status IN ?", clawID,
		[]string{models.WithdrawStatusConfirmed, models.WithdrawStatusSent, models.WithdrawStatusPending}).
		Order("created_at DESC").First(&lastWithdraw).Error
	if err == nil && time.Since(lastWithdraw.CreatedAt) < WithdrawCooldown {
		return nil, fmt.Errorf("withdrawal cooldown: try again later")
	}

	// Create record
	record := &models.WithdrawRecord{
		ClawID:   clawID,
		FromAddr: claw.WalletAddr,
		ToAddr:   userWallet,
		Amount:   amount,
		Status:   models.WithdrawStatusPending,
	}
	if err := database.DB.Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to create withdraw record: %w", err)
	}

	// Mark the amount as withdrawn (optimistic — will be rolled back on failure)
	database.DB.Model(&claw).Update("withdrawn", gorm.Expr("withdrawn + ?", amount))

	// Execute on-chain transfer asynchronously
	go executeWithdrawOnChain(record, &claw)

	return record, nil
}

// executeWithdrawOnChain transfers $Ensoul from Claw wallet to user wallet.
func executeWithdrawOnChain(record *models.WithdrawRecord, claw *models.Claw) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	failWithdraw := func(reason string) {
		database.DB.Model(record).Updates(map[string]interface{}{
			"status":     models.WithdrawStatusFailed,
			"last_error": reason,
		})
		// Roll back the withdrawn amount
		database.DB.Model(&models.Claw{}).Where("id = ?", record.ClawID).
			Update("withdrawn", gorm.Expr("withdrawn - ?", record.Amount))
		util.Log.Error("[withdraw] Failed: %s (claw: %s, amount: %.4f)", reason, claw.Name, record.Amount)
	}

	// Decrypt Claw private key
	clawKey, err := chain.DecryptClawPrivateKey(claw.WalletPKEnc)
	if err != nil {
		failWithdraw(fmt.Sprintf("decrypt key: %v", err))
		return
	}

	// Convert to wei
	amountWei := toWei(record.Amount)

	// Transfer $Ensoul token
	txHash, err := chain.TransferToken(ctx, clawKey, record.ToAddr, amountWei)
	if err != nil {
		failWithdraw(fmt.Sprintf("transfer failed: %v", err))
		return
	}

	database.DB.Model(record).Updates(map[string]interface{}{
		"tx_hash": txHash,
		"status":  models.WithdrawStatusSent,
	})

	// Wait for confirmation
	success, err := chain.WaitForTokenTx(ctx, txHash)
	if err != nil || !success {
		failWithdraw(fmt.Sprintf("tx receipt failed: %v", err))
		return
	}

	database.DB.Model(record).Update("status", models.WithdrawStatusConfirmed)

	util.Log.Info("[withdraw] Success: %.4f $Ensoul from %s to %s (claw: %s), tx=%s",
		record.Amount, record.FromAddr, record.ToAddr, claw.Name, txHash)
}

// GetWithdrawHistory returns withdrawal records for a Claw.
func GetWithdrawHistory(clawID uuid.UUID) ([]models.WithdrawRecord, error) {
	var records []models.WithdrawRecord
	err := database.DB.Where("claw_id = ?", clawID).
		Order("created_at DESC").
		Limit(20).
		Find(&records).Error
	return records, err
}
