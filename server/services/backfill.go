package services

import (
	"context"
	"math/big"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ethereum/go-ethereum/common"
)

// StartAgentIDBackfill launches a background goroutine that periodically scans
// for shells with a mint_tx_hash but missing agent_id, and fills in the agent_id
// by looking up the transaction receipt on-chain.
// This acts as a safety net in case the frontend fails to parse the agentId
// from the Registered event (e.g. network issues, user closes browser early).
func StartAgentIDBackfill(interval time.Duration) {
	go func() {
		// Run once immediately on startup
		backfillAgentIDs()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			backfillAgentIDs()
		}
	}()
	util.Log.Info("[backfill] Agent ID backfill started (interval: %s)", interval)
}

func backfillAgentIDs() {
	if chain.C == nil {
		return
	}

	// Find shells that have a tx hash but no agent_id
	var shells []models.Shell
	result := database.DB.
		Where("mint_tx_hash != '' AND (agent_id IS NULL OR agent_id = 0)").
		Find(&shells)
	if result.Error != nil {
		util.Log.Error("[backfill] Error querying shells: %v", result.Error)
		return
	}
	if len(shells) == 0 {
		return
	}

	util.Log.Debug("[backfill] Found %d shell(s) needing agent_id backfill", len(shells))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, s := range shells {
		txHash := common.HexToHash(s.MintTxHash)
		receipt, err := chain.C.EthClient().TransactionReceipt(ctx, txHash)
		if err != nil {
			util.Log.Warn("[backfill] @%s: failed to get receipt for tx %s: %v", s.Handle, s.MintTxHash, err)
			continue
		}

		// Parse agentId from Registered event
		// Event: Registered(uint256 indexed agentId, string agentURI, address indexed owner)
		var agentID *big.Int
		registeredSig := common.HexToHash("0xca52e62c367d81bb2e328eb795f7c7ba24afb478408a26c0e201d155c449bc4a")

		for _, vLog := range receipt.Logs {
			if len(vLog.Topics) >= 2 && vLog.Topics[0] == registeredSig {
				agentID = new(big.Int).SetBytes(vLog.Topics[1].Bytes())
				break
			}
		}

		if agentID == nil {
			util.Log.Warn("[backfill] @%s: Registered event not found in tx %s", s.Handle, s.MintTxHash)
			continue
		}

		aid := agentID.Uint64()
		err = database.DB.Model(&models.Shell{}).
			Where("id = ?", s.ID).
			Update("agent_id", &aid).Error
		if err != nil {
			util.Log.Error("[backfill] @%s: failed to update agent_id: %v", s.Handle, err)
			continue
		}
		util.Log.Info("[backfill] @%s: agent_id backfilled to %d (tx: %s)", s.Handle, aid, s.MintTxHash)
	}
}
