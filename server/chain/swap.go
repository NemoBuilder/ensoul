package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/util"
)

// PancakeSwap V2 Router partial ABI
const pancakeRouterABI = `[
	{"inputs":[{"name":"amountOutMin","type":"uint256"},{"name":"path","type":"address[]"},{"name":"to","type":"address"},{"name":"deadline","type":"uint256"}],"name":"swapExactETHForTokens","outputs":[{"name":"amounts","type":"uint256[]"}],"stateMutability":"payable","type":"function"},
	{"inputs":[{"name":"amountIn","type":"uint256"},{"name":"path","type":"address[]"}],"name":"getAmountsOut","outputs":[{"name":"amounts","type":"uint256[]"}],"stateMutability":"view","type":"function"}
]`

// WBNB address on BSC mainnet
var wbnbAddr = common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c")

var parsedRouterABI abi.ABI

func init() {
	var err error
	parsedRouterABI, err = abi.JSON(strings.NewReader(pancakeRouterABI))
	if err != nil {
		panic("failed to parse PancakeSwap Router ABI: " + err.Error())
	}
}

func routerAddr() common.Address {
	return common.HexToAddress(config.Cfg.PancakeRouterAddr)
}

// GetSwapQuote returns the estimated $Ensoul amount for a given BNB input.
func GetSwapQuote(ctx context.Context, bnbAmount *big.Int) (*big.Int, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}

	path := []common.Address{wbnbAddr, tokenAddr()}

	data, err := parsedRouterABI.Pack("getAmountsOut", bnbAmount, path)
	if err != nil {
		return nil, fmt.Errorf("failed to pack getAmountsOut: %w", err)
	}

	router := routerAddr()
	result, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &router,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("getAmountsOut call failed: %w", err)
	}

	outputs, err := parsedRouterABI.Unpack("getAmountsOut", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack getAmountsOut: %w", err)
	}

	amounts := outputs[0].([]*big.Int)
	if len(amounts) < 2 {
		return nil, fmt.Errorf("unexpected getAmountsOut result length")
	}

	return amounts[1], nil
}

// SwapBNBForToken swaps BNB for $Ensoul via PancakeSwap V2 Router.
// slippageBps: slippage tolerance in basis points (e.g., 500 = 5%).
// Returns tx hash and the minimum expected token amount.
func SwapBNBForToken(ctx context.Context, buybackKey *ecdsa.PrivateKey, bnbAmount *big.Int, slippageBps int64) (string, *big.Int, error) {
	if C == nil {
		return "", nil, fmt.Errorf("chain client not initialized")
	}

	fromAddr := crypto.PubkeyToAddress(buybackKey.PublicKey)

	// Get quote first
	quote, err := GetSwapQuote(ctx, bnbAmount)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get swap quote: %w", err)
	}

	// Calculate minimum output with slippage
	// amountOutMin = quote * (10000 - slippageBps) / 10000
	slippageMul := big.NewInt(10000 - slippageBps)
	amountOutMin := new(big.Int).Mul(quote, slippageMul)
	amountOutMin.Div(amountOutMin, big.NewInt(10000))

	path := []common.Address{wbnbAddr, tokenAddr()}
	deadline := big.NewInt(time.Now().Unix() + 300) // 5 minutes

	data, err := parsedRouterABI.Pack("swapExactETHForTokens",
		amountOutMin, path, fromAddr, deadline)
	if err != nil {
		return "", nil, fmt.Errorf("failed to pack swapExactETHForTokens: %w", err)
	}

	nonce, err := C.ethClient.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	router := routerAddr()
	gasLimit, err := C.ethClient.EstimateGas(ctx, ethereum.CallMsg{
		From:  fromAddr,
		To:    &router,
		Value: bnbAmount,
		Data:  data,
	})
	if err != nil {
		gasLimit = 300000 // fallback for swap
	}

	tx := types.NewTransaction(nonce, router, bnbAmount, gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), buybackKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to sign swap tx: %w", err)
	}

	if err := C.ethClient.SendTransaction(ctx, signedTx); err != nil {
		return "", nil, fmt.Errorf("failed to send swap tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	util.Log.Info("[swap] BNB→$Ensoul swap sent: %s BNB, minOut=%s, tx=%s",
		bnbAmount.String(), amountOutMin.String(), txHash)

	return txHash, amountOutMin, nil
}
