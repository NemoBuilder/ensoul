package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/util"
)

// Minimal ERC-20 ABI for transfer and balanceOf
const erc20ABI = `[
	{"constant":true,"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"},
	{"constant":false,"inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"type":"function"},
	{"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"},
	{"constant":true,"inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"}
]`

var parsedERC20ABI abi.ABI

func init() {
	var err error
	parsedERC20ABI, err = abi.JSON(strings.NewReader(erc20ABI))
	if err != nil {
		panic("failed to parse ERC-20 ABI: " + err.Error())
	}
}

// tokenAddr returns the $Ensoul token contract address.
func tokenAddr() common.Address {
	return common.HexToAddress(config.Cfg.EnsoulTokenAddr)
}

// GetTokenBalance returns the $Ensoul balance of an address.
func GetTokenBalance(ctx context.Context, addr string) (*big.Int, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}

	data, err := parsedERC20ABI.Pack("balanceOf", common.HexToAddress(addr))
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf: %w", err)
	}

	token := tokenAddr()
	result, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("balanceOf call failed: %w", err)
	}

	outputs, err := parsedERC20ABI.Unpack("balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack balanceOf: %w", err)
	}

	return outputs[0].(*big.Int), nil
}

// TransferToken sends $Ensoul tokens from a wallet to a recipient.
// Returns the transaction hash.
func TransferToken(ctx context.Context, fromKey *ecdsa.PrivateKey, to string, amount *big.Int) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}

	fromAddr := crypto.PubkeyToAddress(fromKey.PublicKey)
	toAddr := common.HexToAddress(to)

	data, err := parsedERC20ABI.Pack("transfer", toAddr, amount)
	if err != nil {
		return "", fmt.Errorf("failed to pack transfer: %w", err)
	}

	nonce, err := C.ethClient.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	token := tokenAddr()
	gasLimit, err := C.ethClient.EstimateGas(ctx, ethereum.CallMsg{
		From: fromAddr,
		To:   &token,
		Data: data,
	})
	if err != nil {
		gasLimit = 100000 // fallback
	}

	tx := types.NewTransaction(nonce, token, big.NewInt(0), gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), fromKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign tx: %w", err)
	}

	if err := C.ethClient.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("failed to send tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	util.Log.Debug("[token] Transfer %s $Ensoul from %s to %s, tx=%s",
		amount.String(), fromAddr.Hex(), to, txHash)

	return txHash, nil
}

// ApproveToken approves a spender to spend $Ensoul tokens on behalf of the owner.
func ApproveToken(ctx context.Context, ownerKey *ecdsa.PrivateKey, spender string, amount *big.Int) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}

	ownerAddr := crypto.PubkeyToAddress(ownerKey.PublicKey)
	spenderAddr := common.HexToAddress(spender)

	data, err := parsedERC20ABI.Pack("approve", spenderAddr, amount)
	if err != nil {
		return "", fmt.Errorf("failed to pack approve: %w", err)
	}

	nonce, err := C.ethClient.PendingNonceAt(ctx, ownerAddr)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	token := tokenAddr()
	tx := types.NewTransaction(nonce, token, big.NewInt(0), 100000, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), ownerKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign tx: %w", err)
	}

	if err := C.ethClient.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("failed to send tx: %w", err)
	}

	return signedTx.Hash().Hex(), nil
}

// TransferBNB sends native BNB from a wallet to a recipient.
// Returns the transaction hash.
func TransferBNB(ctx context.Context, fromKey *ecdsa.PrivateKey, to string, amount *big.Int) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}

	fromAddr := crypto.PubkeyToAddress(fromKey.PublicKey)
	toAddr := common.HexToAddress(to)

	nonce, err := C.ethClient.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Native BNB transfer uses a fixed 21000 gas
	gasLimit := uint64(21000)

	tx := types.NewTransaction(nonce, toAddr, amount, gasLimit, gasPrice, nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), fromKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign tx: %w", err)
	}

	if err := C.ethClient.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("failed to send tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	util.Log.Debug("[chain] Transfer %s BNB from %s to %s, tx=%s",
		amount.String(), fromAddr.Hex(), to, txHash)

	return txHash, nil
}

// WaitForTokenTx waits for a token transaction to be mined and returns success status.
func WaitForTokenTx(ctx context.Context, txHash string) (bool, error) {
	if C == nil {
		return false, fmt.Errorf("chain client not initialized")
	}

	receipt, err := waitForTx(ctx, txHash)
	if err != nil {
		return false, err
	}

	return receipt.Status == types.ReceiptStatusSuccessful, nil
}

// VerifyPaymentTx checks that a transaction exists, was successful, and was sent by
// the expected sender. Returns the value transferred (in wei) and the recipient.
// This is used to verify subscription or mint payments on-chain.
func VerifyPaymentTx(ctx context.Context, txHashHex, expectedSender string) (value *big.Int, to string, err error) {
	if C == nil {
		return nil, "", fmt.Errorf("chain client not initialized")
	}

	txHash := common.HexToHash(txHashHex)

	// Get receipt to check status
	receipt, err := C.ethClient.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, "", fmt.Errorf("tx not found or not yet mined: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, "", fmt.Errorf("tx %s failed (status=0)", txHashHex)
	}

	// Get the full transaction to check sender and value
	tx, _, err := C.ethClient.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get tx details: %w", err)
	}

	// Recover sender from the transaction
	signer := types.LatestSignerForChainID(C.chainID)
	sender, err := types.Sender(signer, tx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to recover sender: %w", err)
	}

	// Verify sender matches
	if !strings.EqualFold(sender.Hex(), expectedSender) {
		return nil, "", fmt.Errorf("sender mismatch: tx from %s, expected %s", sender.Hex(), expectedSender)
	}

	toAddr := ""
	if tx.To() != nil {
		toAddr = tx.To().Hex()
	}

	return tx.Value(), toAddr, nil
}
