package chain

// Generic BNB → arbitrary-ERC20 swap used by the V4 buyback worker.
// Mirrors SwapBNBForToken but takes the output token address as an
// argument instead of hard-coding $Ensoul (config.Cfg.EnsoulTokenAddr).

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ensoul-labs/ensoul-server/util"
)

// GetSwapQuoteFor returns the estimated `tokenAddr` output for a given BNB
// input via the WBNB→tokenAddr path on PancakeSwap V2.
func GetSwapQuoteFor(ctx context.Context, bnbAmount *big.Int, tokenAddrHex string) (*big.Int, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}
	if !common.IsHexAddress(tokenAddrHex) {
		return nil, fmt.Errorf("invalid token address: %s", tokenAddrHex)
	}
	path := []common.Address{wbnbAddr, common.HexToAddress(tokenAddrHex)}
	data, err := parsedRouterABI.Pack("getAmountsOut", bnbAmount, path)
	if err != nil {
		return nil, err
	}
	router := routerAddr()
	result, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{To: &router, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("getAmountsOut: %w", err)
	}
	outs, err := parsedRouterABI.Unpack("getAmountsOut", result)
	if err != nil {
		return nil, err
	}
	amounts := outs[0].([]*big.Int)
	if len(amounts) < 2 {
		return nil, fmt.Errorf("getAmountsOut: unexpected length")
	}
	return amounts[1], nil
}

// SwapBNBForArbitraryToken swaps BNB → `tokenAddrHex` via PancakeSwap V2.
// recipient — final ERC-20 destination (typically the buyback wallet itself
// or a treasury). slippageBps — basis points (500 = 5%).
// Returns tx hash and minOut.
func SwapBNBForArbitraryToken(
	ctx context.Context,
	key *ecdsa.PrivateKey,
	bnbAmount *big.Int,
	tokenAddrHex string,
	recipient common.Address,
	slippageBps int64,
) (string, *big.Int, error) {
	if C == nil {
		return "", nil, fmt.Errorf("chain client not initialized")
	}
	if !common.IsHexAddress(tokenAddrHex) {
		return "", nil, fmt.Errorf("invalid token address: %s", tokenAddrHex)
	}
	fromAddr := crypto.PubkeyToAddress(key.PublicKey)
	if recipient == (common.Address{}) {
		recipient = fromAddr
	}

	quote, err := GetSwapQuoteFor(ctx, bnbAmount, tokenAddrHex)
	if err != nil {
		return "", nil, fmt.Errorf("quote: %w", err)
	}
	slippageMul := big.NewInt(10000 - slippageBps)
	amountOutMin := new(big.Int).Mul(quote, slippageMul)
	amountOutMin.Div(amountOutMin, big.NewInt(10000))

	path := []common.Address{wbnbAddr, common.HexToAddress(tokenAddrHex)}
	deadline := big.NewInt(time.Now().Unix() + 300)

	data, err := parsedRouterABI.Pack("swapExactETHForTokens",
		amountOutMin, path, recipient, deadline)
	if err != nil {
		return "", nil, fmt.Errorf("pack: %w", err)
	}

	nonce, err := C.ethClient.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", nil, fmt.Errorf("nonce: %w", err)
	}
	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("gas price: %w", err)
	}
	router := routerAddr()
	gasLimit, err := C.ethClient.EstimateGas(ctx, ethereum.CallMsg{
		From: fromAddr, To: &router, Value: bnbAmount, Data: data,
	})
	if err != nil {
		gasLimit = 350000
	}

	tx := types.NewTransaction(nonce, router, bnbAmount, gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), key)
	if err != nil {
		return "", nil, fmt.Errorf("sign: %w", err)
	}
	if err := C.SwapEthClient().SendTransaction(ctx, signedTx); err != nil {
		return "", nil, fmt.Errorf("send: %w", err)
	}
	hash := signedTx.Hash().Hex()
	util.Log.Info("[swap-generic] BNB→%s sent: bnb=%s minOut=%s tx=%s",
		tokenAddrHex, bnbAmount.String(), amountOutMin.String(), hash)
	return hash, amountOutMin, nil
}
