// V4 EpochRegistry client.
//
// Single responsibility: send recordRoot(galaxyId, index, root, atomCount)
// to the on-chain EnsoulEpochRegistry, return the tx hash.
//
// Reuses the existing platform key + nonce manager + ethClient on C.
// If config.Cfg.EpochRegistryAddr is empty (e.g. local dev with no
// deployed contract), PushEpochRoot returns ErrEpochRegistryNotConfigured
// so callers can degrade gracefully.
package chain

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ErrEpochRegistryNotConfigured is returned when EPOCH_REGISTRY_ADDR is unset.
var ErrEpochRegistryNotConfigured = errors.New("chain: EPOCH_REGISTRY_ADDR not configured")

// Minimal ABI — only the methods + events we actually call/read.
const epochRegistryABI = `[
  {"type":"function","name":"recordRoot","stateMutability":"nonpayable","inputs":[
    {"name":"galaxyId","type":"bytes32"},
    {"name":"index","type":"uint64"},
    {"name":"root","type":"bytes32"},
    {"name":"atomCount","type":"uint64"}
  ],"outputs":[]},
  {"type":"function","name":"nextIndex","stateMutability":"view","inputs":[
    {"name":"","type":"bytes32"}
  ],"outputs":[{"name":"","type":"uint64"}]},
  {"type":"function","name":"epochs","stateMutability":"view","inputs":[
    {"name":"","type":"bytes32"},
    {"name":"","type":"uint64"}
  ],"outputs":[
    {"name":"root","type":"bytes32"},
    {"name":"atomCount","type":"uint64"},
    {"name":"closedAt","type":"uint64"},
    {"name":"writer","type":"address"}
  ]}
]`

var parsedEpochRegistryABI abi.ABI

func init() {
	parsed, err := abi.JSON(strings.NewReader(epochRegistryABI))
	if err != nil {
		panic("invalid epochRegistryABI: " + err.Error())
	}
	parsedEpochRegistryABI = parsed
}

// PushEpochRoot signs and submits a recordRoot tx. Returns the tx hash hex
// (no 0x prefix in DB, but 0x-prefixed in the returned string for ergonomics).
//
// galaxyIDBytes is the 16-byte UUID right-padded into a bytes32 (UUIDs are
// 128-bit; we left-align so the on-chain id matches the off-chain UUID's
// canonical big-endian byte order). Use uuid.UUID.MarshalBinary() upstream.
func PushEpochRoot(
	ctx context.Context,
	galaxyIDBytes [32]byte,
	index uint64,
	rootBytes [32]byte,
	atomCount uint64,
) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}
	if C.platformKey == nil {
		return "", fmt.Errorf("platform key not configured — cannot push epoch root")
	}
	addrStr := config.Cfg.EpochRegistryAddr
	if addrStr == "" {
		return "", ErrEpochRegistryNotConfigured
	}
	contractAddr := common.HexToAddress(addrStr)

	data, err := parsedEpochRegistryABI.Pack("recordRoot",
		galaxyIDBytes, index, rootBytes, atomCount,
	)
	if err != nil {
		return "", fmt.Errorf("pack recordRoot: %w", err)
	}

	nonce, err := C.ethClient.PendingNonceAt(ctx, C.platformAddr)
	if err != nil {
		return "", fmt.Errorf("pending nonce: %w", err)
	}
	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}
	gasLimit, err := C.ethClient.EstimateGas(ctx, ethereum.CallMsg{
		From: C.platformAddr,
		To:   &contractAddr,
		Data: data,
	})
	if err != nil {
		gasLimit = 200_000 // generous fallback for a simple mapping write
	}

	tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), C.platformKey)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	if err := C.ethClient.SendTransaction(ctx, signed); err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	hash := signed.Hash().Hex()
	util.Log.Info("[epoch] recordRoot galaxy=%x index=%d atoms=%d tx=%s",
		galaxyIDBytes[:4], index, atomCount, hash)
	return hash, nil
}

// NextEpochIndex reads the on-chain expected next index for a galaxy. Useful
// as a sanity check before pushing (off-chain DB index must equal this).
func NextEpochIndex(ctx context.Context, galaxyIDBytes [32]byte) (uint64, error) {
	if C == nil {
		return 0, fmt.Errorf("chain client not initialized")
	}
	addrStr := config.Cfg.EpochRegistryAddr
	if addrStr == "" {
		return 0, ErrEpochRegistryNotConfigured
	}
	contractAddr := common.HexToAddress(addrStr)
	data, err := parsedEpochRegistryABI.Pack("nextIndex", galaxyIDBytes)
	if err != nil {
		return 0, err
	}
	out, err := C.ethClient.CallContract(ctx, ethereum.CallMsg{
		To: &contractAddr, Data: data,
	}, nil)
	if err != nil {
		return 0, err
	}
	vals, err := parsedEpochRegistryABI.Unpack("nextIndex", out)
	if err != nil || len(vals) == 0 {
		return 0, fmt.Errorf("unpack nextIndex: %w", err)
	}
	v, _ := vals[0].(uint64)
	if v == 0 {
		v = 1
	}
	return v, nil
}
