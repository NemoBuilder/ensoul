package contracts

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// IdentityRegistryABI is the ABI string for the ERC-8004 Identity Registry.
const IdentityRegistryABIJSON = `[
  {"type":"function","name":"register","inputs":[{"name":"agentURI","type":"string"}],"outputs":[{"name":"agentId","type":"uint256"}],"stateMutability":"nonpayable"},
  {"type":"function","name":"setAgentURI","inputs":[{"name":"agentId","type":"uint256"},{"name":"newURI","type":"string"}],"outputs":[],"stateMutability":"nonpayable"},
  {"type":"function","name":"tokenURI","inputs":[{"name":"tokenId","type":"uint256"}],"outputs":[{"name":"","type":"string"}],"stateMutability":"view"},
  {"type":"function","name":"ownerOf","inputs":[{"name":"tokenId","type":"uint256"}],"outputs":[{"name":"","type":"address"}],"stateMutability":"view"},
  {"type":"function","name":"setMetadata","inputs":[{"name":"agentId","type":"uint256"},{"name":"metadataKey","type":"string"},{"name":"metadataValue","type":"bytes"}],"outputs":[],"stateMutability":"nonpayable"},
  {"type":"function","name":"getMetadata","inputs":[{"name":"agentId","type":"uint256"},{"name":"metadataKey","type":"string"}],"outputs":[{"name":"","type":"bytes"}],"stateMutability":"view"},
  {"type":"function","name":"getAgentWallet","inputs":[{"name":"agentId","type":"uint256"}],"outputs":[{"name":"","type":"address"}],"stateMutability":"view"},
  {"type":"function","name":"getVersion","inputs":[],"outputs":[{"name":"","type":"string"}],"stateMutability":"pure"},
  {"type":"event","name":"Registered","inputs":[{"name":"agentId","type":"uint256","indexed":true},{"name":"agentURI","type":"string","indexed":false},{"name":"owner","type":"address","indexed":true}]},
  {"type":"event","name":"MetadataSet","inputs":[{"name":"agentId","type":"uint256","indexed":true},{"name":"indexedMetadataKey","type":"string","indexed":true},{"name":"metadataKey","type":"string","indexed":false},{"name":"metadataValue","type":"bytes","indexed":false}]},
  {"type":"event","name":"URIUpdated","inputs":[{"name":"agentId","type":"uint256","indexed":true},{"name":"newURI","type":"string","indexed":false},{"name":"updatedBy","type":"address","indexed":true}]}
]`

// IdentityRegistry is a Go binding for the ERC-8004 IdentityRegistryUpgradeable contract.
type IdentityRegistry struct {
	ABI      abi.ABI
	contract *bind.BoundContract
	address  common.Address
}

// NewIdentityRegistry creates a new IdentityRegistry binding.
func NewIdentityRegistry(address common.Address, backend bind.ContractBackend) (*IdentityRegistry, error) {
	parsed, err := abi.JSON(strings.NewReader(IdentityRegistryABIJSON))
	if err != nil {
		return nil, err
	}
	contract := bind.NewBoundContract(address, parsed, backend, backend, backend)
	return &IdentityRegistry{
		ABI:      parsed,
		contract: contract,
		address:  address,
	}, nil
}

// Register mints a new agent NFT with the given URI and returns the transaction.
// The agentId is extracted from the Registered event log after mining.
func (ir *IdentityRegistry) Register(opts *bind.TransactOpts, agentURI string) (*types.Transaction, error) {
	return ir.contract.Transact(opts, "register", agentURI)
}

// SetAgentURI updates the agent's tokenURI on-chain.
func (ir *IdentityRegistry) SetAgentURI(opts *bind.TransactOpts, agentId *big.Int, newURI string) (*types.Transaction, error) {
	return ir.contract.Transact(opts, "setAgentURI", agentId, newURI)
}

// SetMetadata sets custom on-chain metadata for an agent.
func (ir *IdentityRegistry) SetMetadata(opts *bind.TransactOpts, agentId *big.Int, key string, value []byte) (*types.Transaction, error) {
	return ir.contract.Transact(opts, "setMetadata", agentId, key, value)
}

// TokenURI reads the agentURI for a given token.
func (ir *IdentityRegistry) TokenURI(opts *bind.CallOpts, tokenId *big.Int) (string, error) {
	var out []interface{}
	err := ir.contract.Call(opts, &out, "tokenURI", tokenId)
	if err != nil {
		return "", err
	}
	return out[0].(string), nil
}

// OwnerOf returns the owner address of the given agent NFT.
func (ir *IdentityRegistry) OwnerOf(opts *bind.CallOpts, tokenId *big.Int) (common.Address, error) {
	var out []interface{}
	err := ir.contract.Call(opts, &out, "ownerOf", tokenId)
	if err != nil {
		return common.Address{}, err
	}
	return out[0].(common.Address), nil
}

// GetMetadata reads on-chain metadata for an agent.
func (ir *IdentityRegistry) GetMetadata(opts *bind.CallOpts, agentId *big.Int, key string) ([]byte, error) {
	var out []interface{}
	err := ir.contract.Call(opts, &out, "getMetadata", agentId, key)
	if err != nil {
		return nil, err
	}
	return out[0].([]byte), nil
}

// GetAgentWallet returns the verified wallet address for an agent.
func (ir *IdentityRegistry) GetAgentWallet(opts *bind.CallOpts, agentId *big.Int) (common.Address, error) {
	var out []interface{}
	err := ir.contract.Call(opts, &out, "getAgentWallet", agentId)
	if err != nil {
		return common.Address{}, err
	}
	return out[0].(common.Address), nil
}

// GetVersion returns the contract version string.
func (ir *IdentityRegistry) GetVersion(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := ir.contract.Call(opts, &out, "getVersion")
	if err != nil {
		return "", err
	}
	return out[0].(string), nil
}

// Address returns the contract address.
func (ir *IdentityRegistry) Address() common.Address {
	return ir.address
}

// ParseRegisteredEvent extracts agentId from a Registered event log.
func (ir *IdentityRegistry) ParseRegisteredEvent(log types.Log) (*big.Int, error) {
	if len(log.Topics) < 2 {
		return nil, nil
	}
	agentId := new(big.Int).SetBytes(log.Topics[1].Bytes())
	return agentId, nil
}
