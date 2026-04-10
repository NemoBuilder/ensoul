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
	{"inputs":[{"name":"amountIn","type":"uint256"},{"name":"amountOutMin","type":"uint256"},{"name":"path","type":"address[]"},{"name":"to","type":"address"},{"name":"deadline","type":"uint256"}],"name":"swapExactTokensForETH","outputs":[{"name":"amounts","type":"uint256[]"}],"stateMutability":"nonpayable","type":"function"},
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

	// Send via private RPC to prevent sandwich attacks
	if err := C.SwapEthClient().SendTransaction(ctx, signedTx); err != nil {
		return "", nil, fmt.Errorf("failed to send swap tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	util.Log.Info("[swap] BNB→$Ensoul swap sent (private RPC): %s BNB, minOut=%s, tx=%s",
		bnbAmount.String(), amountOutMin.String(), txHash)

	return txHash, amountOutMin, nil
}

// usdtAddr returns the BSC USDT contract address.
func usdtAddr() common.Address {
	return common.HexToAddress(config.Cfg.USDTAddr)
}

// GetUSDTToBNBQuote returns estimated BNB output for a given USDT input.
func GetUSDTToBNBQuote(ctx context.Context, usdtAmount *big.Int) (*big.Int, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}

	path := []common.Address{usdtAddr(), wbnbAddr}

	data, err := parsedRouterABI.Pack("getAmountsOut", usdtAmount, path)
	if err != nil {
		return nil, fmt.Errorf("failed to pack getAmountsOut: %w", err)
	}

	router := routerAddr()
	result, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &router,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("getAmountsOut USDT→BNB call failed: %w", err)
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

// SwapUSDTForBNB swaps USDT for BNB via PancakeSwap V2 Router.
// Requires prior ERC-20 approval of USDT to the Router.
// slippageBps: slippage tolerance in basis points (e.g., 500 = 5%).
// Returns tx hash and the minimum expected BNB amount.
func SwapUSDTForBNB(ctx context.Context, buybackKey *ecdsa.PrivateKey, usdtAmount *big.Int, slippageBps int64) (string, *big.Int, error) {
	if C == nil {
		return "", nil, fmt.Errorf("chain client not initialized")
	}

	fromAddr := crypto.PubkeyToAddress(buybackKey.PublicKey)

	// Check & set USDT allowance for the Router
	if err := ensureAllowance(ctx, buybackKey, usdtAddr(), routerAddr(), usdtAmount); err != nil {
		return "", nil, fmt.Errorf("USDT approval failed: %w", err)
	}

	// Get quote
	quote, err := GetUSDTToBNBQuote(ctx, usdtAmount)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get USDT→BNB quote: %w", err)
	}

	// Calculate minimum output with slippage
	slippageMul := big.NewInt(10000 - slippageBps)
	amountOutMin := new(big.Int).Mul(quote, slippageMul)
	amountOutMin.Div(amountOutMin, big.NewInt(10000))

	path := []common.Address{usdtAddr(), wbnbAddr}
	deadline := big.NewInt(time.Now().Unix() + 300)

	data, err := parsedRouterABI.Pack("swapExactTokensForETH",
		usdtAmount, amountOutMin, path, fromAddr, deadline)
	if err != nil {
		return "", nil, fmt.Errorf("failed to pack swapExactTokensForETH: %w", err)
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
		From: fromAddr,
		To:   &router,
		Data: data,
	})
	if err != nil {
		gasLimit = 350000 // fallback for token-to-ETH swap
	}

	tx := types.NewTransaction(nonce, router, big.NewInt(0), gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), buybackKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to sign USDT swap tx: %w", err)
	}

	// Send via private RPC to prevent sandwich attacks
	if err := C.SwapEthClient().SendTransaction(ctx, signedTx); err != nil {
		return "", nil, fmt.Errorf("failed to send USDT swap tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	util.Log.Info("[swap] USDT→BNB swap sent (private RPC): %s USDT, minOut=%s BNB, tx=%s",
		usdtAmount.String(), amountOutMin.String(), txHash)

	return txHash, amountOutMin, nil
}

// GetBNBPriceInUSDT returns the current BNB price in USDT using PancakeSwap V2.
// It queries getAmountsOut for 1 WBNB → USDT.
func GetBNBPriceInUSDT(ctx context.Context) (float64, error) {
	if C == nil {
		return 0, fmt.Errorf("chain client not initialized")
	}

	// 1 BNB = 1e18 wei
	oneBNB := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	path := []common.Address{wbnbAddr, usdtAddr()}

	data, err := parsedRouterABI.Pack("getAmountsOut", oneBNB, path)
	if err != nil {
		return 0, fmt.Errorf("failed to pack getAmountsOut: %w", err)
	}

	router := routerAddr()
	result, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &router,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("getAmountsOut WBNB→USDT call failed: %w", err)
	}

	outputs, err := parsedRouterABI.Unpack("getAmountsOut", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack getAmountsOut: %w", err)
	}

	amounts := outputs[0].([]*big.Int)
	if len(amounts) < 2 {
		return 0, fmt.Errorf("unexpected getAmountsOut result length")
	}

	// USDT has 18 decimals on BSC
	usdtWei := amounts[1]
	// Convert to float: divide by 1e18
	f := new(big.Float).SetInt(usdtWei)
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	price, _ := new(big.Float).Quo(f, divisor).Float64()

	return price, nil
}

// ensureAllowance checks the ERC-20 allowance and approves if needed.
func ensureAllowance(ctx context.Context, ownerKey *ecdsa.PrivateKey, token, spender common.Address, amount *big.Int) error {
	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)

	// Check current allowance
	data, err := parsedERC20ABI.Pack("allowance", ownerAddr, spender)
	if err != nil {
		return fmt.Errorf("failed to pack allowance: %w", err)
	}
	result, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: data,
	}, nil)
	if err != nil {
		return fmt.Errorf("allowance call failed: %w", err)
	}
	outputs, err := parsedERC20ABI.Unpack("allowance", result)
	if err != nil {
		return fmt.Errorf("failed to unpack allowance: %w", err)
	}
	currentAllowance := outputs[0].(*big.Int)

	if currentAllowance.Cmp(amount) >= 0 {
		return nil // already approved
	}

	// Approve max uint256 to avoid repeated approvals
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	approveData, err := parsedERC20ABI.Pack("approve", spender, maxUint256)
	if err != nil {
		return fmt.Errorf("failed to pack approve: %w", err)
	}

	nonce, err := C.ethClient.PendingNonceAt(ctx, ownerAddr)
	if err != nil {
		return fmt.Errorf("failed to get nonce for approval: %w", err)
	}

	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to get gas price for approval: %w", err)
	}

	tx := types.NewTransaction(nonce, token, big.NewInt(0), 100000, gasPrice, approveData)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), ownerKey)
	if err != nil {
		return fmt.Errorf("failed to sign approval tx: %w", err)
	}

	if err := C.ethClient.SendTransaction(ctx, signedTx); err != nil {
		return fmt.Errorf("failed to send approval tx: %w", err)
	}

	// Wait for approval confirmation
	success, err := WaitForTokenTx(ctx, signedTx.Hash().Hex())
	if err != nil || !success {
		return fmt.Errorf("approval tx failed: %s", signedTx.Hash().Hex())
	}

	util.Log.Info("[swap] Approved %s to spend token %s", spender.Hex(), token.Hex())
	return nil
}
