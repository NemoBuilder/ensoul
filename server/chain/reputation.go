package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// SubmitFeedback sends reputation feedback from a Claw's wallet to the Reputation Registry.
// The Claw's own wallet address is the msg.sender, making each feedback independently verifiable.
// value: the feedback score (e.g., 85 for 85% quality). valueDecimals: typically 0.
// tag1/tag2: optional categorization tags (e.g., "personality", "knowledge").
// endpoint: the agent's service endpoint URL.
// feedbackURI: link to the detailed feedback content.
// feedbackHash: keccak256 hash of the feedback content for integrity verification.
func SubmitFeedback(
	ctx context.Context,
	clawKey *ecdsa.PrivateKey,
	agentId *big.Int,
	value int64,
	tag1, tag2 string,
	endpoint, feedbackURI string,
	feedbackHash [32]byte,
) (string, error) {
	if C == nil {
		return "", fmt.Errorf("chain client not initialized")
	}

	// Create transaction opts from the Claw's key
	opts, err := C.TransactOptsFromKey(ctx, clawKey)
	if err != nil {
		return "", fmt.Errorf("failed to create transactor: %w", err)
	}

	// Prepare feedback parameters
	feedbackValue := big.NewInt(value)

	tx, err := C.reputationRegistry.GiveFeedback(
		opts,
		agentId,
		feedbackValue,
		0,           // valueDecimals = 0 (whole number)
		tag1,        // dimension/category tag
		tag2,        // sub-category tag
		endpoint,    // agent soul page URL
		feedbackURI, // link to fragment detail
		feedbackHash,
	)
	if err != nil {
		return "", fmt.Errorf("giveFeedback() call failed: %w", err)
	}

	log.Printf("[chain] Reputation feedback tx sent: %s (agentId=%s, value=%d, tag1=%s)",
		tx.Hash().Hex(), agentId.String(), value, tag1)

	// Wait for confirmation
	receipt, err := bind.WaitMined(ctx, C.ethClient, tx)
	if err != nil {
		return tx.Hash().Hex(), fmt.Errorf("waiting for feedback receipt: %w", err)
	}

	if receipt.Status != types.ReceiptStatusSuccessful {
		return tx.Hash().Hex(), fmt.Errorf("giveFeedback() tx reverted")
	}

	log.Printf("[chain] Reputation feedback confirmed: agentId=%s, value=%d, tx=%s",
		agentId.String(), value, tx.Hash().Hex())

	return tx.Hash().Hex(), nil
}

// ReadReputationSummary reads the aggregated reputation for a soul from the chain.
// clientAddresses should be the list of known Claw wallet addresses.
func ReadReputationSummary(
	ctx context.Context,
	agentId *big.Int,
	clientAddresses []common.Address,
) (uint64, *big.Int, uint8, error) {
	if C == nil {
		return 0, nil, 0, fmt.Errorf("chain client not initialized")
	}

	summary, err := C.reputationRegistry.GetSummary(
		&bind.CallOpts{Context: ctx},
		agentId,
		clientAddresses,
		"", "", // No tag filtering
	)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("getSummary() call failed: %w", err)
	}

	return summary.Count, summary.SummaryValue, summary.SummaryValueDecimals, nil
}

// ReadFeedbackForClaw reads the latest feedback a specific Claw gave to a soul.
func ReadFeedbackForClaw(
	ctx context.Context,
	agentId *big.Int,
	clawAddr common.Address,
) (*big.Int, string, string, error) {
	if C == nil {
		return nil, "", "", fmt.Errorf("chain client not initialized")
	}

	// Get the last feedback index
	lastIndex, err := C.reputationRegistry.GetLastIndex(
		&bind.CallOpts{Context: ctx},
		agentId,
		clawAddr,
	)
	if err != nil {
		return nil, "", "", fmt.Errorf("getLastIndex() call failed: %w", err)
	}

	if lastIndex == 0 {
		return nil, "", "", nil // No feedback given
	}

	// Read the latest feedback
	feedback, err := C.reputationRegistry.ReadFeedback(
		&bind.CallOpts{Context: ctx},
		agentId,
		clawAddr,
		lastIndex,
	)
	if err != nil {
		return nil, "", "", fmt.Errorf("readFeedback() call failed: %w", err)
	}

	return feedback.Value, feedback.Tag1, feedback.Tag2, nil
}

// GetReputationClients returns all addresses that have given feedback to an agent.
func GetReputationClients(ctx context.Context, agentId *big.Int) ([]common.Address, error) {
	if C == nil {
		return nil, fmt.Errorf("chain client not initialized")
	}

	return C.reputationRegistry.GetClients(
		&bind.CallOpts{Context: ctx},
		agentId,
	)
}
