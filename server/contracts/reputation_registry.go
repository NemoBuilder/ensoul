package contracts

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ReputationRegistryABIJSON is the ABI string for the ERC-8004 Reputation Registry.
const ReputationRegistryABIJSON = `[
  {"type":"function","name":"giveFeedback","inputs":[{"name":"agentId","type":"uint256"},{"name":"value","type":"int128"},{"name":"valueDecimals","type":"uint8"},{"name":"tag1","type":"string"},{"name":"tag2","type":"string"},{"name":"endpoint","type":"string"},{"name":"feedbackURI","type":"string"},{"name":"feedbackHash","type":"bytes32"}],"outputs":[],"stateMutability":"nonpayable"},
  {"type":"function","name":"readFeedback","inputs":[{"name":"agentId","type":"uint256"},{"name":"clientAddress","type":"address"},{"name":"feedbackIndex","type":"uint64"}],"outputs":[{"name":"value","type":"int128"},{"name":"valueDecimals","type":"uint8"},{"name":"tag1","type":"string"},{"name":"tag2","type":"string"},{"name":"isRevoked","type":"bool"}],"stateMutability":"view"},
  {"type":"function","name":"getSummary","inputs":[{"name":"agentId","type":"uint256"},{"name":"clientAddresses","type":"address[]"},{"name":"tag1","type":"string"},{"name":"tag2","type":"string"}],"outputs":[{"name":"count","type":"uint64"},{"name":"summaryValue","type":"int128"},{"name":"summaryValueDecimals","type":"uint8"}],"stateMutability":"view"},
  {"type":"function","name":"getLastIndex","inputs":[{"name":"agentId","type":"uint256"},{"name":"clientAddress","type":"address"}],"outputs":[{"name":"","type":"uint64"}],"stateMutability":"view"},
  {"type":"function","name":"getClients","inputs":[{"name":"agentId","type":"uint256"}],"outputs":[{"name":"","type":"address[]"}],"stateMutability":"view"},
  {"type":"function","name":"getIdentityRegistry","inputs":[],"outputs":[{"name":"","type":"address"}],"stateMutability":"view"},
  {"type":"function","name":"getVersion","inputs":[],"outputs":[{"name":"","type":"string"}],"stateMutability":"pure"},
  {"type":"event","name":"NewFeedback","inputs":[{"name":"agentId","type":"uint256","indexed":true},{"name":"clientAddress","type":"address","indexed":true},{"name":"feedbackIndex","type":"uint64","indexed":false},{"name":"value","type":"int128","indexed":false},{"name":"valueDecimals","type":"uint8","indexed":false},{"name":"indexedTag1","type":"string","indexed":true},{"name":"tag1","type":"string","indexed":false},{"name":"tag2","type":"string","indexed":false},{"name":"endpoint","type":"string","indexed":false},{"name":"feedbackURI","type":"string","indexed":false},{"name":"feedbackHash","type":"bytes32","indexed":false}]},
  {"type":"event","name":"FeedbackRevoked","inputs":[{"name":"agentId","type":"uint256","indexed":true},{"name":"clientAddress","type":"address","indexed":true},{"name":"feedbackIndex","type":"uint64","indexed":true}]}
]`

// ReputationRegistry is a Go binding for the ERC-8004 ReputationRegistryUpgradeable contract.
type ReputationRegistry struct {
	ABI      abi.ABI
	contract *bind.BoundContract
	address  common.Address
}

// FeedbackResult holds the return values from readFeedback.
type FeedbackResult struct {
	Value         *big.Int
	ValueDecimals uint8
	Tag1          string
	Tag2          string
	IsRevoked     bool
}

// SummaryResult holds the return values from getSummary.
type SummaryResult struct {
	Count                uint64
	SummaryValue         *big.Int
	SummaryValueDecimals uint8
}

// NewReputationRegistry creates a new ReputationRegistry binding.
func NewReputationRegistry(address common.Address, backend bind.ContractBackend) (*ReputationRegistry, error) {
	parsed, err := abi.JSON(strings.NewReader(ReputationRegistryABIJSON))
	if err != nil {
		return nil, err
	}
	contract := bind.NewBoundContract(address, parsed, backend, backend, backend)
	return &ReputationRegistry{
		ABI:      parsed,
		contract: contract,
		address:  address,
	}, nil
}

// GiveFeedback submits feedback for an agent on-chain.
// value is a signed int128 (use big.Int), valueDecimals 0-18, tags/endpoint/feedbackURI/feedbackHash are optional.
func (rr *ReputationRegistry) GiveFeedback(
	opts *bind.TransactOpts,
	agentId *big.Int,
	value *big.Int,
	valueDecimals uint8,
	tag1, tag2, endpoint, feedbackURI string,
	feedbackHash [32]byte,
) (*types.Transaction, error) {
	return rr.contract.Transact(opts, "giveFeedback",
		agentId, value, valueDecimals,
		tag1, tag2, endpoint, feedbackURI, feedbackHash,
	)
}

// ReadFeedback reads a specific feedback entry from the contract.
func (rr *ReputationRegistry) ReadFeedback(
	opts *bind.CallOpts,
	agentId *big.Int,
	clientAddress common.Address,
	feedbackIndex uint64,
) (*FeedbackResult, error) {
	var out []interface{}
	err := rr.contract.Call(opts, &out, "readFeedback", agentId, clientAddress, feedbackIndex)
	if err != nil {
		return nil, err
	}
	return &FeedbackResult{
		Value:         out[0].(*big.Int),
		ValueDecimals: out[1].(uint8),
		Tag1:          out[2].(string),
		Tag2:          out[3].(string),
		IsRevoked:     out[4].(bool),
	}, nil
}

// GetSummary returns an aggregated reputation summary for an agent.
func (rr *ReputationRegistry) GetSummary(
	opts *bind.CallOpts,
	agentId *big.Int,
	clientAddresses []common.Address,
	tag1, tag2 string,
) (*SummaryResult, error) {
	var out []interface{}
	err := rr.contract.Call(opts, &out, "getSummary", agentId, clientAddresses, tag1, tag2)
	if err != nil {
		return nil, err
	}
	return &SummaryResult{
		Count:                out[0].(uint64),
		SummaryValue:         out[1].(*big.Int),
		SummaryValueDecimals: out[2].(uint8),
	}, nil
}

// GetLastIndex returns the last feedback index for a client on an agent.
func (rr *ReputationRegistry) GetLastIndex(opts *bind.CallOpts, agentId *big.Int, clientAddress common.Address) (uint64, error) {
	var out []interface{}
	err := rr.contract.Call(opts, &out, "getLastIndex", agentId, clientAddress)
	if err != nil {
		return 0, err
	}
	return out[0].(uint64), nil
}

// GetClients returns all unique client addresses that have given feedback to an agent.
func (rr *ReputationRegistry) GetClients(opts *bind.CallOpts, agentId *big.Int) ([]common.Address, error) {
	var out []interface{}
	err := rr.contract.Call(opts, &out, "getClients", agentId)
	if err != nil {
		return nil, err
	}
	return out[0].([]common.Address), nil
}

// GetIdentityRegistry returns the linked identity registry address.
func (rr *ReputationRegistry) GetIdentityRegistry(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := rr.contract.Call(opts, &out, "getIdentityRegistry")
	if err != nil {
		return common.Address{}, err
	}
	return out[0].(common.Address), nil
}

// GetVersion returns the contract version string.
func (rr *ReputationRegistry) GetVersion(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := rr.contract.Call(opts, &out, "getVersion")
	if err != nil {
		return "", err
	}
	return out[0].(string), nil
}

// Address returns the contract address.
func (rr *ReputationRegistry) Address() common.Address {
	return rr.address
}
