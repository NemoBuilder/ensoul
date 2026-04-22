package chain

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ensoul-labs/ensoul-server/config"
)

// Reject reason codes (mirrored to frontend i18n).
const (
	RejectNotFound    = "NOT_FOUND"
	RejectWrongChain  = "WRONG_CHAIN"
	RejectWrongToken  = "WRONG_TOKEN"
	RejectWrongAddr   = "WRONG_ADDR"
	RejectAmountLow   = "AMOUNT_LOW"
	RejectTxFailed    = "TX_FAILED"
	RejectPriceOutlier = "PRICE_OUTLIER"
)

// Payment status returned by VerifyPayment.
const (
	PayStatusPending   = "pending"
	PayStatusConfirmed = "confirmed"
	PayStatusRejected  = "rejected"
)

// PaymentInfo is the normalized result of a successful payment lookup.
type PaymentInfo struct {
	Status         string         // pending | confirmed | rejected
	RejectReason   string         // populated when Status == rejected
	From           common.Address
	To             common.Address
	Token          string         // "USDT" | "BNB"
	Amount         *big.Int       // wei (USDT 18d / BNB 18d)
	USDTEquivalent *big.Int       // wei, 18 decimals
	BlockNumber    uint64
	Confirmations  uint64
}

// transferTopic = keccak256("Transfer(address,address,uint256)")
var transferTopic = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

// VerifyPayment fetches a tx by hash and validates that it transfers
// `expectedToken` (USDT or BNB) to `recipient` with USDT-equivalent value
// >= `minUSDTWei` (within toleranceBPS for BNB).
//
// Returns PaymentInfo with Status set to one of pending/confirmed/rejected.
// Errors are returned only for transport-level failures (RPC down, etc).
func VerifyPayment(
	ctx context.Context,
	txHash common.Hash,
	recipient common.Address,
	expectedToken string,
	minUSDTWei *big.Int,
	toleranceBPS int,
	minConfirmations uint64,
) (*PaymentInfo, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}

	// 1. Fetch receipt. If not yet mined, the tx is still pending — do NOT reject.
	// Real "never mined" txs are reaped by the cron's 1-hour timeout.
	receipt, err := C.ethClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		if errors.Is(err, ethereum.NotFound) {
			return &PaymentInfo{Status: PayStatusPending}, nil
		}
		return nil, fmt.Errorf("fetch receipt: %w", err)
	}

	if receipt.Status != 1 {
		return &PaymentInfo{Status: PayStatusRejected, RejectReason: RejectTxFailed}, nil
	}

	// 2. Confirmation depth.
	head, err := C.ethClient.BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch head: %w", err)
	}
	if head < receipt.BlockNumber.Uint64() {
		return &PaymentInfo{Status: PayStatusPending}, nil
	}
	confirmations := head - receipt.BlockNumber.Uint64() + 1
	if confirmations < minConfirmations {
		return &PaymentInfo{
			Status:        PayStatusPending,
			BlockNumber:   receipt.BlockNumber.Uint64(),
			Confirmations: confirmations,
		}, nil
	}

	expectedToken = strings.ToUpper(expectedToken)

	switch expectedToken {
	case "USDT":
		usdt := common.HexToAddress(config.Cfg.USDTAddr)
		// Find the matching Transfer log: token contract = USDT, topic2 = recipient.
		for _, lg := range receipt.Logs {
			if lg.Address != usdt {
				continue
			}
			if len(lg.Topics) != 3 || lg.Topics[0] != transferTopic {
				continue
			}
			toAddr := common.BytesToAddress(lg.Topics[2].Bytes())
			if toAddr != recipient {
				continue
			}
			amount := new(big.Int).SetBytes(lg.Data)
			if amount.Cmp(minUSDTWei) < 0 {
				return &PaymentInfo{
					Status:        PayStatusRejected,
					RejectReason:  RejectAmountLow,
					Token:         "USDT",
					Amount:        amount,
					BlockNumber:   receipt.BlockNumber.Uint64(),
					Confirmations: confirmations,
				}, nil
			}
			fromAddr := common.BytesToAddress(lg.Topics[1].Bytes())
			return &PaymentInfo{
				Status:         PayStatusConfirmed,
				From:           fromAddr,
				To:             toAddr,
				Token:          "USDT",
				Amount:         amount,
				USDTEquivalent: new(big.Int).Set(amount),
				BlockNumber:    receipt.BlockNumber.Uint64(),
				Confirmations:  confirmations,
			}, nil
		}
		// No matching Transfer log found.
		return &PaymentInfo{Status: PayStatusRejected, RejectReason: RejectWrongToken}, nil

	case "BNB":
		tx, _, err := C.ethClient.TransactionByHash(ctx, txHash)
		if err != nil {
			return nil, fmt.Errorf("fetch tx: %w", err)
		}
		if tx.To() == nil || *tx.To() != recipient {
			return &PaymentInfo{Status: PayStatusRejected, RejectReason: RejectWrongAddr}, nil
		}
		if tx.Value().Sign() == 0 {
			return &PaymentInfo{Status: PayStatusRejected, RejectReason: RejectWrongToken}, nil
		}
		// Convert BNB amount to USDT equivalent via PancakeSwap.
		usdtEquiv, err := GetBNBToUSDTQuote(ctx, tx.Value())
		if err != nil {
			return nil, fmt.Errorf("bnb->usdt quote: %w", err)
		}
		// Sanity check: BNB price between 100 and 2000 USDT.
		// 1 BNB (1e18 wei) → usdtEquivPer1 = quote * 1e18 / tx.Value()
		// We compare (usdtEquiv * 1e18) vs (tx.Value() * 100) and (tx.Value() * 2000).
		one := big.NewInt(1)
		if tx.Value().Cmp(one) > 0 {
			lower := new(big.Int).Mul(tx.Value(), big.NewInt(100))
			upper := new(big.Int).Mul(tx.Value(), big.NewInt(2000))
			if usdtEquiv.Cmp(lower) < 0 || usdtEquiv.Cmp(upper) > 0 {
				return &PaymentInfo{Status: PayStatusRejected, RejectReason: RejectPriceOutlier}, nil
			}
		}
		// Apply tolerance: usdtEquiv * 10000 >= minUSDT * (10000 - tolBPS)
		threshold := new(big.Int).Mul(minUSDTWei, big.NewInt(int64(10000-toleranceBPS)))
		lhs := new(big.Int).Mul(usdtEquiv, big.NewInt(10000))
		if lhs.Cmp(threshold) < 0 {
			return &PaymentInfo{
				Status:         PayStatusRejected,
				RejectReason:   RejectAmountLow,
				Token:          "BNB",
				Amount:         tx.Value(),
				USDTEquivalent: usdtEquiv,
				BlockNumber:    receipt.BlockNumber.Uint64(),
				Confirmations:  confirmations,
			}, nil
		}
		// Recover from address.
		fromAddr, err := types.Sender(types.LatestSignerForChainID(C.chainID), tx)
		if err != nil {
			return nil, fmt.Errorf("recover from addr: %w", err)
		}
		return &PaymentInfo{
			Status:         PayStatusConfirmed,
			From:           fromAddr,
			To:             *tx.To(),
			Token:          "BNB",
			Amount:         tx.Value(),
			USDTEquivalent: usdtEquiv,
			BlockNumber:    receipt.BlockNumber.Uint64(),
			Confirmations:  confirmations,
		}, nil

	default:
		return &PaymentInfo{Status: PayStatusRejected, RejectReason: RejectWrongToken}, nil
	}
}

// GetBNBToUSDTQuote returns estimated USDT (wei, 18d) for given BNB amount (wei).
// Path: [WBNB, USDT].
func GetBNBToUSDTQuote(ctx context.Context, bnbAmount *big.Int) (*big.Int, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}
	usdt := common.HexToAddress(config.Cfg.USDTAddr)
	path := []common.Address{wbnbAddr, usdt}

	data, err := parsedRouterABI.Pack("getAmountsOut", bnbAmount, path)
	if err != nil {
		return nil, fmt.Errorf("pack: %w", err)
	}

	router := routerAddr()
	result, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &router,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}

	outputs, err := parsedRouterABI.Unpack("getAmountsOut", result)
	if err != nil {
		return nil, fmt.Errorf("unpack: %w", err)
	}
	amounts := outputs[0].([]*big.Int)
	if len(amounts) < 2 {
		return nil, fmt.Errorf("unexpected result length")
	}
	return amounts[1], nil
}
