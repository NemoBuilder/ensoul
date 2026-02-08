package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ensoul-labs/ensoul-server/util"
)

// AgentRegistrationFile is the ERC-8004 registration file format.
// This is stored at the agentURI and follows the spec's recommended shape.
type AgentRegistrationFile struct {
	Type          string                 `json:"type"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Image         string                 `json:"image"`
	Services      []AgentService         `json:"services,omitempty"`
	Registrations []AgentRegistration    `json:"registrations,omitempty"`
	Ensoul        map[string]interface{} `json:"ensoul,omitempty"` // Custom Ensoul-specific metadata
}

// AgentService describes an agent's service endpoint.
type AgentService struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Protocol string `json:"protocol,omitempty"`
}

// AgentRegistration binds the registration file back to on-chain identity.
type AgentRegistration struct {
	AgentRegistry string `json:"agentRegistry"`
	AgentID       string `json:"agentId"`
}

// MintSoul registers a new Soul as an ERC-8004 agent on-chain.
// Returns the agentId (tokenId) and the transaction hash.
func MintSoul(ctx context.Context, handle, ownerAddr, avatarURL, seedSummary string, dnaVersion int) (*big.Int, string, error) {
	if C == nil {
		return nil, "", fmt.Errorf("chain client not initialized")
	}
	if !C.HasPlatformKey() {
		util.Log.Debug("[chain] Skipping on-chain minting: no platform key configured")
		return nil, "", nil
	}

	// Build the ERC-8004 registration file
	regFile := AgentRegistrationFile{
		Type:        "https://eips.ethereum.org/EIPS/eip-8004#registration-v1",
		Name:        fmt.Sprintf("@%s Soul", handle),
		Description: seedSummary,
		Image:       avatarURL,
		Services: []AgentService{
			{
				Name:     "web",
				URL:      fmt.Sprintf("https://ensoul.ac/soul/%s", handle),
				Protocol: "https",
			},
			{
				Name:     "chat",
				URL:      fmt.Sprintf("https://ensoul.ac/soul/%s/chat", handle),
				Protocol: "https",
			},
		},
		Ensoul: map[string]interface{}{
			"handle":     handle,
			"stage":      "embryo",
			"dnaVersion": dnaVersion,
		},
	}

	// Serialize to JSON for data URI (fully on-chain)
	regJSON, err := json.Marshal(regFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to serialize registration file: %w", err)
	}

	// Use data URI for fully on-chain metadata
	agentURI := "data:application/json;base64," + encodeBase64(regJSON)

	// Create transaction opts
	opts, err := C.PlatformTransactOpts(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create transaction opts: %w", err)
	}

	// Call register(agentURI) on the Identity Registry
	tx, err := C.identityRegistry.Register(opts, agentURI)
	if err != nil {
		return nil, "", fmt.Errorf("register() call failed: %w", err)
	}

	util.Log.Debug("[chain] Soul registration tx sent: %s (handle: @%s)", tx.Hash().Hex(), handle)

	// Wait for transaction receipt
	receipt, err := bind.WaitMined(ctx, C.ethClient, tx)
	if err != nil {
		return nil, tx.Hash().Hex(), fmt.Errorf("waiting for tx receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return nil, tx.Hash().Hex(), fmt.Errorf("register() tx reverted (status=%d)", receipt.Status)
	}

	// Extract agentId from the Registered event
	agentId, err := extractAgentIdFromReceipt(receipt)
	if err != nil {
		return nil, tx.Hash().Hex(), fmt.Errorf("failed to extract agentId from receipt: %w", err)
	}

	util.Log.Info("[chain] Soul registered on-chain: @%s -> agentId=%s, tx=%s", handle, agentId.String(), tx.Hash().Hex())

	// Set additional metadata: handle and stage
	go func() {
		setCtx := context.Background()
		setOpts, err := C.PlatformTransactOpts(setCtx)
		if err != nil {
			util.Log.Error("[chain] Failed to create opts for setMetadata: %v", err)
			return
		}

		// Store the handle as on-chain metadata
		_, err = C.identityRegistry.SetMetadata(setOpts, agentId, "ensoul:handle", []byte(handle))
		if err != nil {
			util.Log.Error("[chain] Failed to set handle metadata: %v", err)
		} else {
			util.Log.Debug("[chain] Handle metadata set for agentId=%s", agentId.String())
		}
	}()

	return agentId, tx.Hash().Hex(), nil
}

// UpdateSoulURI updates the agentURI on-chain after an ensouling event.
func UpdateSoulURI(ctx context.Context, agentId *big.Int, handle, avatarURL, seedSummary, stage string, dnaVersion int) (string, error) {
	if C == nil || !C.HasPlatformKey() {
		util.Log.Debug("[chain] Skipping URI update: chain client not configured")
		return "", nil
	}

	// Build updated registration file
	regFile := AgentRegistrationFile{
		Type:        "https://eips.ethereum.org/EIPS/eip-8004#registration-v1",
		Name:        fmt.Sprintf("@%s Soul", handle),
		Description: seedSummary,
		Image:       avatarURL,
		Services: []AgentService{
			{
				Name:     "web",
				URL:      fmt.Sprintf("https://ensoul.ac/soul/%s", handle),
				Protocol: "https",
			},
			{
				Name:     "chat",
				URL:      fmt.Sprintf("https://ensoul.ac/soul/%s/chat", handle),
				Protocol: "https",
			},
		},
		Ensoul: map[string]interface{}{
			"handle":     handle,
			"stage":      stage,
			"dnaVersion": dnaVersion,
		},
	}

	regJSON, err := json.Marshal(regFile)
	if err != nil {
		return "", fmt.Errorf("failed to serialize registration file: %w", err)
	}

	agentURI := "data:application/json;base64," + encodeBase64(regJSON)

	opts, err := C.PlatformTransactOpts(ctx)
	if err != nil {
		return "", err
	}

	tx, err := C.identityRegistry.SetAgentURI(opts, agentId, agentURI)
	if err != nil {
		return "", fmt.Errorf("setAgentURI() call failed: %w", err)
	}

	util.Log.Debug("[chain] Soul URI update tx sent: %s (agentId=%s, dna v%d)", tx.Hash().Hex(), agentId.String(), dnaVersion)

	// Wait for receipt
	receipt, err := bind.WaitMined(ctx, C.ethClient, tx)
	if err != nil {
		return tx.Hash().Hex(), fmt.Errorf("waiting for setAgentURI receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return tx.Hash().Hex(), fmt.Errorf("setAgentURI() tx reverted")
	}

	util.Log.Info("[chain] Soul URI updated on-chain: agentId=%s, tx=%s", agentId.String(), tx.Hash().Hex())
	return tx.Hash().Hex(), nil
}

// ReadSoulURI reads the current agentURI from the chain.
func ReadSoulURI(ctx context.Context, agentId *big.Int) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}
	return C.identityRegistry.TokenURI(&bind.CallOpts{Context: ctx}, agentId)
}

// ReadSoulOwner reads the owner address of a soul NFT.
func ReadSoulOwner(ctx context.Context, agentId *big.Int) (common.Address, error) {
	if C == nil {
		return common.Address{}, fmt.Errorf("chain client not initialized")
	}
	return C.identityRegistry.OwnerOf(&bind.CallOpts{Context: ctx}, agentId)
}

// extractAgentIdFromReceipt extracts the agentId from the Registered event in a transaction receipt.
func extractAgentIdFromReceipt(receipt *types.Receipt) (*big.Int, error) {
	// The Registered event signature: Registered(uint256 indexed agentId, string agentURI, address indexed owner)
	registeredEventSig := common.HexToHash("0xca52e62c367d81bb2e328eb795f7c7ba24afb478408a26c0e201d155c449bc4a")

	for _, vLog := range receipt.Logs {
		if len(vLog.Topics) >= 2 && vLog.Topics[0] == registeredEventSig {
			agentId := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
			return agentId, nil
		}
	}

	// Fallback: try to find any event with 2+ topics from the identity registry
	// The first topic matching is the event sig, second is indexed agentId
	for _, vLog := range receipt.Logs {
		if vLog.Address == C.identityRegistry.Address() && len(vLog.Topics) >= 2 {
			agentId := new(big.Int).SetBytes(vLog.Topics[1].Bytes())
			return agentId, nil
		}
	}

	return nil, fmt.Errorf("Registered event not found in receipt")
}
