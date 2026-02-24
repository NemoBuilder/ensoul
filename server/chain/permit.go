package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/util"
)

// MintPermit holds the data for a signed mint permit.
type MintPermit struct {
	HandleHash string `json:"handle_hash"` // hex-encoded keccak256(lowercase handle)
	Price      string `json:"price"`       // wei string
	Deadline   int64  `json:"deadline"`    // unix timestamp
	Nonce      string `json:"nonce"`       // unique nonce (string to avoid JS precision loss)
	Signature  string `json:"signature"`   // hex-encoded signature
}

// HandleHash computes keccak256(abi.encodePacked(lowercaseHandle)) matching the contract.
func HandleHash(handle string) common.Hash {
	handle = strings.ToLower(handle)
	return crypto.Keccak256Hash([]byte(handle))
}

// SignMintPermit creates a signed mint permit for the EnsoulMinterV2 contract.
// The signature covers: handleHash, price, userAddr, deadline, nonce, chainId, minterAddr
func SignMintPermit(
	handle string,
	priceWei *big.Int,
	userAddr common.Address,
	deadline int64,
	nonce uint64,
) (*MintPermit, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}
	if C.platformKey == nil {
		return nil, fmt.Errorf("platform private key not configured")
	}

	minterAddr := common.HexToAddress(config.Cfg.EnsoulMinterV2Addr)
	if minterAddr == (common.Address{}) {
		return nil, fmt.Errorf("ENSOUL_MINTER_V2_ADDR not configured")
	}

	handleH := HandleHash(handle)

	// Pack the same way as the contract:
	// keccak256(abi.encodePacked(handleHash, price, msg.sender, deadline, nonce, block.chainid, address(this)))
	// NOTE: abi.encodePacked uses 20 bytes for address, 32 bytes for uint256/bytes32
	messageHash := crypto.Keccak256Hash(
		handleH.Bytes(), // bytes32: 32 bytes
		common.LeftPadBytes(priceWei.Bytes(), 32), // uint256: 32 bytes
		userAddr.Bytes(), // address: 20 bytes
		common.LeftPadBytes(big.NewInt(deadline).Bytes(), 32),          // uint256: 32 bytes
		common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32), // uint256: 32 bytes
		common.LeftPadBytes(C.chainID.Bytes(), 32),                     // uint256: 32 bytes
		minterAddr.Bytes(), // address: 20 bytes
	)

	// EIP-191 personal sign prefix
	prefixed := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n32")),
		messageHash.Bytes(),
	)

	sig, err := crypto.Sign(prefixed.Bytes(), C.platformKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign permit: %w", err)
	}

	// Adjust V from 0/1 to 27/28 for Solidity ecrecover
	if sig[64] < 27 {
		sig[64] += 27
	}

	return &MintPermit{
		HandleHash: handleH.Hex(),
		Price:      priceWei.String(),
		Deadline:   deadline,
		Nonce:      fmt.Sprintf("%d", nonce),
		Signature:  "0x" + common.Bytes2Hex(sig),
	}, nil
}

// GetMintPrice returns the mint price in wei based on follower count.
// Tiered pricing as defined in Ensoul-Next.md:
//
//	< 1K      → 0.01 BNB
//	1K-10K    → 0.03 BNB
//	10K-100K  → 0.1  BNB
//	100K-1M   → 0.3  BNB
//	1M-10M    → 1.0  BNB
//	> 10M     → 3.0  BNB
func GetMintPrice(followers int) *big.Int {
	var ethAmount float64
	switch {
	case followers < 1000:
		ethAmount = 0.01
	case followers < 10000:
		ethAmount = 0.03
	case followers < 100000:
		ethAmount = 0.1
	case followers < 1000000:
		ethAmount = 0.3
	case followers < 10000000:
		ethAmount = 1.0
	default:
		ethAmount = 3.0
	}

	// Convert to wei: amount * 1e18
	weiPerEth := new(big.Float).SetFloat64(1e18)
	amountFloat := new(big.Float).SetFloat64(ethAmount)
	weiFloat := new(big.Float).Mul(amountFloat, weiPerEth)

	wei, _ := weiFloat.Int(nil)
	return wei
}

// GetMintPriceTier returns the tier name for display purposes.
func GetMintPriceTier(followers int) string {
	switch {
	case followers < 1000:
		return "micro"
	case followers < 10000:
		return "small"
	case followers < 100000:
		return "medium"
	case followers < 1000000:
		return "large"
	case followers < 10000000:
		return "top"
	default:
		return "super"
	}
}

// GetBNBBalance returns the native BNB balance of an address.
func GetBNBBalance(ctx context.Context, addr string) (*big.Int, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}
	balance, err := C.ethClient.BalanceAt(ctx, common.HexToAddress(addr), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get BNB balance: %w", err)
	}
	return balance, nil
}

// CallMintWithPermit calls EnsoulMinterV2.mint() from a given wallet with a signed permit.
// The caller sends BNB (priceWei) to the minter contract along with the permit data.
// agentURI should be the full EIP-8004 registration file (data:application/json;base64,...)
func CallMintWithPermit(ctx context.Context, callerKey *ecdsa.PrivateKey, handle string, priceWei *big.Int, permit *MintPermit, agentURI string) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}

	minterAddr := common.HexToAddress(config.Cfg.EnsoulMinterV2Addr)
	if minterAddr == (common.Address{}) {
		return "", fmt.Errorf("ENSOUL_MINTER_V2_ADDR not configured")
	}

	callerAddr := crypto.PubkeyToAddress(callerKey.PublicKey)

	// Build the mint function call data
	// mint(string agentURI, bytes32 handleHash, uint256 price, uint256 deadline, uint256 nonce, bytes signature)
	handleHash := HandleHash(handle)

	// Function selector: keccak256("mint(string,bytes32,uint256,uint256,uint256,bytes)")[:4]
	mintSig := crypto.Keccak256([]byte("mint(string,bytes32,uint256,uint256,uint256,bytes)"))[:4]

	sigBytes := common.Hex2Bytes(strings.TrimPrefix(permit.Signature, "0x"))

	permitNonce, _ := new(big.Int).SetString(permit.Nonce, 10)
	data := encodeMintCall(mintSig, agentURI, handleHash, priceWei, big.NewInt(permit.Deadline), permitNonce, sigBytes)

	nonce, err := C.ethClient.PendingNonceAt(ctx, callerAddr)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	gasLimit, err := C.ethClient.EstimateGas(ctx, ethereum.CallMsg{
		From:  callerAddr,
		To:    &minterAddr,
		Value: priceWei,
		Data:  data,
	})
	if err != nil {
		gasLimit = 300000 // fallback
		util.Log.Warn("[permit] Gas estimation failed, using fallback: %v", err)
	}

	tx := types.NewTransaction(nonce, minterAddr, priceWei, gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), callerKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign mint tx: %w", err)
	}

	if err := C.ethClient.SendTransaction(ctx, signedTx); err != nil {
		return "", fmt.Errorf("failed to send mint tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	util.Log.Info("[permit] Mint tx sent for @%s: price=%s BNB, tx=%s", handle, priceWei.String(), txHash)
	return txHash, nil
}

// encodeMintCall ABI-encodes the mint(string,bytes32,uint256,uint256,uint256,bytes) call.
func encodeMintCall(selector []byte, agentURI string, handleHash common.Hash, price, deadline, nonce *big.Int, signature []byte) []byte {
	// ABI encoding with dynamic types (string and bytes) and static types (bytes32, uint256s)
	// Layout: selector + head(6 params) + tail(string data) + tail(bytes data)

	headSize := 6 * 32 // 192 bytes for the 6 parameter slots

	// String data: length + padded content
	uriBytes := []byte(agentURI)
	uriPaddedLen := ((len(uriBytes) + 31) / 32) * 32
	stringData := make([]byte, 32+uriPaddedLen)
	copy(stringData[:32], common.LeftPadBytes(big.NewInt(int64(len(uriBytes))).Bytes(), 32))
	copy(stringData[32:], uriBytes)

	// Bytes data: length + padded content
	sigPaddedLen := ((len(signature) + 31) / 32) * 32
	bytesData := make([]byte, 32+sigPaddedLen)
	copy(bytesData[:32], common.LeftPadBytes(big.NewInt(int64(len(signature))).Bytes(), 32))
	copy(bytesData[32:], signature)

	// Calculate offsets
	stringOffset := big.NewInt(int64(headSize))                  // offset to string data
	bytesOffset := big.NewInt(int64(headSize + len(stringData))) // offset to bytes data

	// Build the full calldata
	result := make([]byte, 0, 4+headSize+len(stringData)+len(bytesData))
	result = append(result, selector...)
	result = append(result, common.LeftPadBytes(stringOffset.Bytes(), 32)...) // param0: string offset (dynamic)
	result = append(result, handleHash.Bytes()...)                            // param1: bytes32 handleHash (static)
	result = append(result, common.LeftPadBytes(price.Bytes(), 32)...)        // param2: uint256 price
	result = append(result, common.LeftPadBytes(deadline.Bytes(), 32)...)     // param3: uint256 deadline
	result = append(result, common.LeftPadBytes(nonce.Bytes(), 32)...)        // param4: uint256 nonce
	result = append(result, common.LeftPadBytes(bytesOffset.Bytes(), 32)...)  // param5: bytes offset (dynamic)
	result = append(result, stringData...)
	result = append(result, bytesData...)

	return result
}

// ParsePrivateKey parses a hex-encoded private key string.
func ParsePrivateKey(hexKey string) (*ecdsa.PrivateKey, error) {
	if hexKey == "" {
		return nil, fmt.Errorf("private key is empty")
	}
	if len(hexKey) > 2 && hexKey[:2] == "0x" {
		hexKey = hexKey[2:]
	}
	return crypto.HexToECDSA(hexKey)
}

// AddressFromKey derives the address from a private key.
func AddressFromKey(key *ecdsa.PrivateKey) common.Address {
	return crypto.PubkeyToAddress(key.PublicKey)
}
