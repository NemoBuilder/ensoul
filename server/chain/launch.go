// V4 EnsoulFairLaunch client.
//
// Only platform-key ops live here:
//   - OpenLaunch   (admin opens the per-galaxy window)
//   - SetToken     (admin wires the deployed community token)
//   - Finalize     (anyone can call after the window closes; platform pays gas)
//
// Depositor flows (deposit / claim / refund) are signed by user wallets
// from the frontend, so they have no Go counterpart.
//
// galaxyId encoding matches chain/epoch.go: UUID's 16 bytes left-aligned
// into bytes32.
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
	"github.com/ethereum/go-ethereum/crypto"
)

// ErrFairLaunchNotConfigured — FAIR_LAUNCH_ADDR env var unset.
var ErrFairLaunchNotConfigured = errors.New("chain: FAIR_LAUNCH_ADDR not configured")

const fairLaunchABI = `[
  {"type":"function","name":"openLaunch","stateMutability":"nonpayable","inputs":[
    {"name":"gid","type":"bytes32"},
    {"name":"founder","type":"address"},
    {"name":"start","type":"uint64"},
    {"name":"end","type":"uint64"},
    {"name":"minRaise","type":"uint128"},
    {"name":"maxRaise","type":"uint128"},
    {"name":"supply","type":"uint256"}
  ],"outputs":[]},
  {"type":"function","name":"setToken","stateMutability":"nonpayable","inputs":[
    {"name":"gid","type":"bytes32"},
    {"name":"token","type":"address"}
  ],"outputs":[]},
  {"type":"function","name":"finalize","stateMutability":"nonpayable","inputs":[
    {"name":"gid","type":"bytes32"}
  ],"outputs":[]},
  {"type":"event","name":"Finalized","anonymous":false,"inputs":[
    {"indexed":true,"name":"gid","type":"bytes32"},
    {"indexed":false,"name":"succeeded","type":"bool"},
    {"indexed":false,"name":"totalRaised","type":"uint128"},
    {"indexed":false,"name":"founderShare","type":"uint256"},
    {"indexed":false,"name":"platformShare","type":"uint256"}
  ]}
]`

var (
	parsedFairLaunchABI abi.ABI
	finalizedTopic      common.Hash
)

func init() {
	parsed, err := abi.JSON(strings.NewReader(fairLaunchABI))
	if err != nil {
		panic("invalid fairLaunchABI: " + err.Error())
	}
	parsedFairLaunchABI = parsed
	finalizedTopic = crypto.Keccak256Hash([]byte("Finalized(bytes32,bool,uint128,uint256,uint256)"))
}

// sendFairLaunchTx packs `method` + args, signs with platform key, sends and
// returns the tx hash. Shared by all three admin entrypoints.
func sendFairLaunchTx(ctx context.Context, method string, gasGuess uint64, args ...interface{}) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}
	if C.platformKey == nil {
		return "", fmt.Errorf("platform key not configured")
	}
	addrStr := config.Cfg.FairLaunchAddr
	if addrStr == "" {
		return "", ErrFairLaunchNotConfigured
	}
	contractAddr := common.HexToAddress(addrStr)

	data, err := parsedFairLaunchABI.Pack(method, args...)
	if err != nil {
		return "", fmt.Errorf("pack %s: %w", method, err)
	}
	nonce, err := C.ethClient.PendingNonceAt(ctx, C.platformAddr)
	if err != nil {
		return "", err
	}
	gasPrice, err := C.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", err
	}
	gasLimit, err := C.ethClient.EstimateGas(ctx, ethereum.CallMsg{
		From: C.platformAddr, To: &contractAddr, Data: data,
	})
	if err != nil {
		gasLimit = gasGuess
	}
	tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), gasLimit, gasPrice, data)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(C.chainID), C.platformKey)
	if err != nil {
		return "", err
	}
	if err := C.ethClient.SendTransaction(ctx, signed); err != nil {
		return "", err
	}
	hash := signed.Hash().Hex()
	util.Log.Info("[fairlaunch] %s tx=%s", method, hash)
	return hash, nil
}

// OpenLaunch — admin opens the per-galaxy raise window.
//   minRaise / maxRaise are uint128 wei (pass nil for maxRaise to mean uncapped, encoded as 0)
//   supply is uint256 wei (token base units).
func OpenLaunch(
	ctx context.Context,
	gid [32]byte,
	founder common.Address,
	start, end uint64,
	minRaise, maxRaise, supply *big.Int,
) (string, error) {
	if maxRaise == nil {
		maxRaise = big.NewInt(0)
	}
	return sendFairLaunchTx(ctx, "openLaunch", 300_000,
		gid, founder, start, end, minRaise, maxRaise, supply)
}

// SetToken — admin wires the deployed EnsoulCommunityToken address.
func SetToken(ctx context.Context, gid [32]byte, token common.Address) (string, error) {
	return sendFairLaunchTx(ctx, "setToken", 80_000, gid, token)
}

// Finalize — closes the launch. Platform pays gas so the UX is "founder taps a button".
func Finalize(ctx context.Context, gid [32]byte) (string, error) {
	return sendFairLaunchTx(ctx, "finalize", 300_000, gid)
}

// FinalizeOutcome is what ParseFinalizedLog returns after a receipt arrives.
type FinalizeOutcome struct {
	Succeeded     bool
	TotalRaised   *big.Int
	FounderShare  *big.Int
	PlatformShare *big.Int
}

// ParseFinalizedLog walks a receipt looking for the FairLaunch Finalized
// event and decodes it. Returns nil if no matching log is present.
func ParseFinalizedLog(receipt *types.Receipt) *FinalizeOutcome {
	if receipt == nil || config.Cfg.FairLaunchAddr == "" {
		return nil
	}
	want := common.HexToAddress(config.Cfg.FairLaunchAddr)
	for _, lg := range receipt.Logs {
		if lg.Address != want || len(lg.Topics) < 1 || lg.Topics[0] != finalizedTopic {
			continue
		}
		vals, err := parsedFairLaunchABI.Unpack("Finalized", lg.Data)
		if err != nil || len(vals) < 4 {
			return nil
		}
		out := &FinalizeOutcome{}
		out.Succeeded, _ = vals[0].(bool)
		if v, ok := vals[1].(*big.Int); ok {
			out.TotalRaised = v
		}
		if v, ok := vals[2].(*big.Int); ok {
			out.FounderShare = v
		}
		if v, ok := vals[3].(*big.Int); ok {
			out.PlatformShare = v
		}
		return out
	}
	return nil
}
