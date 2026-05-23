// V4 GalaxyNFT client — minter side.
//
// Only the platform minter (PLATFORM_PRIVATE_KEY) can call mintGalaxy; the
// NFT contract enforces this. The returned tokenId is parsed from the
// GalaxyMinted event log.
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

// ErrGalaxyNFTNotConfigured is returned when GALAXY_NFT_ADDR is unset.
var ErrGalaxyNFTNotConfigured = errors.New("chain: GALAXY_NFT_ADDR not configured")

const galaxyNFTABI = `[
  {"type":"function","name":"mintGalaxy","stateMutability":"nonpayable","inputs":[
    {"name":"to","type":"address"},
    {"name":"gid","type":"bytes32"},
    {"name":"uri","type":"string"}
  ],"outputs":[{"name":"tokenId","type":"uint256"}]},
  {"type":"function","name":"ownerOf","stateMutability":"view","inputs":[
    {"name":"tokenId","type":"uint256"}
  ],"outputs":[{"name":"","type":"address"}]},
  {"type":"event","name":"GalaxyMinted","anonymous":false,"inputs":[
    {"indexed":true,"name":"tokenId","type":"uint256"},
    {"indexed":true,"name":"galaxyId","type":"bytes32"},
    {"indexed":true,"name":"founder","type":"address"},
    {"indexed":false,"name":"uri","type":"string"}
  ]}
]`

var parsedGalaxyNFTABI abi.ABI

// galaxyMintedTopic is the precomputed topic[0] for fast log filtering.
var galaxyMintedTopic common.Hash

func init() {
	parsed, err := abi.JSON(strings.NewReader(galaxyNFTABI))
	if err != nil {
		panic("invalid galaxyNFTABI: " + err.Error())
	}
	parsedGalaxyNFTABI = parsed
	galaxyMintedTopic = crypto.Keccak256Hash([]byte("GalaxyMinted(uint256,bytes32,address,string)"))
}

// MintGalaxyResult is what the caller persists after a successful mint.
type MintGalaxyResult struct {
	TxHash  string
	TokenID *big.Int // nil if log parsing fell back (tx still succeeded)
}

// MintGalaxy submits a mintGalaxy() tx. Does NOT wait for receipt — the
// caller should poll TransactionReceipt later if it needs the tokenId from
// the event log. For ergonomics MintGalaxyAndWait does the wait + parse.
func MintGalaxy(
	ctx context.Context,
	to common.Address,
	galaxyIDBytes [32]byte,
	uri string,
) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}
	if C.platformKey == nil {
		return "", fmt.Errorf("platform key not configured")
	}
	addrStr := config.Cfg.GalaxyNFTAddr
	if addrStr == "" {
		return "", ErrGalaxyNFTNotConfigured
	}
	contractAddr := common.HexToAddress(addrStr)

	data, err := parsedGalaxyNFTABI.Pack("mintGalaxy", to, galaxyIDBytes, uri)
	if err != nil {
		return "", fmt.Errorf("pack mintGalaxy: %w", err)
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
		gasLimit = 400_000
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
	util.Log.Info("[galaxy-nft] mintGalaxy to=%s gid=%x tx=%s", to.Hex(), galaxyIDBytes[:4], hash)
	return hash, nil
}

// ParseMintedTokenID inspects a receipt's logs and returns the tokenId from
// the first GalaxyMinted event (or nil if none found).
func ParseMintedTokenID(receipt *types.Receipt) *big.Int {
	if receipt == nil {
		return nil
	}
	addrStr := config.Cfg.GalaxyNFTAddr
	if addrStr == "" {
		return nil
	}
	want := common.HexToAddress(addrStr)
	for _, lg := range receipt.Logs {
		if lg.Address != want {
			continue
		}
		if len(lg.Topics) < 2 || lg.Topics[0] != galaxyMintedTopic {
			continue
		}
		// tokenId is topics[1] (indexed uint256).
		return new(big.Int).SetBytes(lg.Topics[1].Bytes())
	}
	return nil
}
