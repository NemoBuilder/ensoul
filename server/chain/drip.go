package chain

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Gas drip configuration
var (
	// MinGasBalance is the minimum BNB balance a Claw wallet needs (0.0005 BNB).
	// If balance is below this, a drip is triggered.
	MinGasBalance = big.NewInt(500_000_000_000_000) // 0.0005 BNB in wei

	// DripAmount is the BNB sent to a Claw wallet per drip (0.001 BNB).
	// Enough for ~3-5 giveFeedback transactions.
	DripAmount = big.NewInt(1_000_000_000_000_000) // 0.001 BNB in wei
)

// NeedsGasDrip checks if a Claw wallet's BNB balance is below the minimum threshold.
func NeedsGasDrip(ctx context.Context, clawAddr string) (bool, error) {
	if C == nil {
		return false, fmt.Errorf("chain client not initialized")
	}

	addr := common.HexToAddress(clawAddr)
	balance, err := C.ethClient.BalanceAt(ctx, addr, nil)
	if err != nil {
		return false, fmt.Errorf("failed to check balance for %s: %w", clawAddr, err)
	}

	needsDrip := balance.Cmp(MinGasBalance) < 0
	if needsDrip {
		log.Printf("[chain] Claw wallet %s balance is %s wei (below threshold %s), needs drip",
			clawAddr, balance.String(), MinGasBalance.String())
	}

	return needsDrip, nil
}

// DripGas sends a small amount of BNB from the platform wallet to a Claw wallet for gas fees.
// Returns the tx hash on success.
func DripGas(ctx context.Context, clawAddr string) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}
	if C.platformKey == nil {
		return "", fmt.Errorf("platform private key not configured, cannot drip gas")
	}

	toAddr := common.HexToAddress(clawAddr)

	// Get the platform wallet nonce
	nonce, err := C.ethClient.PendingNonceAt(ctx, C.platformAddr)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get suggested gas price
	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Simple BNB transfer: 21000 gas
	gasLimit := uint64(21000)

	// Build the transaction
	tx := types.NewTransaction(nonce, toAddr, DripAmount, gasLimit, gasPrice, nil)

	// Sign with platform key
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), C.platformKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign drip tx: %w", err)
	}

	// Send
	if err := C.ethClient.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("failed to send drip tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	log.Printf("[chain] Gas drip sent to %s: %s BNB, tx=%s",
		clawAddr, "0.001", txHash)

	return txHash, nil
}

// EnsureGasAndDrip checks if a Claw wallet has enough gas, and drips if needed.
// This is the main entry point called before submitting on-chain feedback.
// Returns nil if the wallet has enough gas (either already or after drip).
func EnsureGasAndDrip(ctx context.Context, clawAddr string) error {
	needs, err := NeedsGasDrip(ctx, clawAddr)
	if err != nil {
		return fmt.Errorf("gas check failed: %w", err)
	}

	if !needs {
		return nil // Already has enough gas
	}

	// Send drip
	txHash, err := DripGas(ctx, clawAddr)
	if err != nil {
		return fmt.Errorf("gas drip failed: %w", err)
	}

	log.Printf("[chain] Gas drip successful for %s, waiting for confirmation... tx=%s", clawAddr, txHash)

	// Wait for the drip tx to be mined before proceeding
	// (the Claw needs the BNB in its account before it can send a tx)
	receipt, err := waitForTx(ctx, txHash)
	if err != nil {
		return fmt.Errorf("drip tx not confirmed: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("drip tx reverted: %s", txHash)
	}

	log.Printf("[chain] Gas drip confirmed for %s: tx=%s", clawAddr, txHash)
	return nil
}

// waitForTx polls for a transaction receipt until it's mined or times out.
func waitForTx(ctx context.Context, txHashHex string) (*types.Receipt, error) {
	txHash := common.HexToHash(txHashHex)

	// Set a 60-second timeout for waiting
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		receipt, err := C.ethClient.TransactionReceipt(ctx, txHash)
		if err == nil {
			return receipt, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for tx %s: %w", txHashHex, ctx.Err())
		case <-ticker.C:
			// retry
		}
	}
}

// GetPlatformBalance returns the platform wallet's BNB balance for monitoring.
func GetPlatformBalance(ctx context.Context) (*big.Int, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}
	return C.ethClient.BalanceAt(ctx, C.platformAddr, nil)
}
