package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ensoul-labs/ensoul-server/util"
)

// nonceEntry tracks the current nonce and serializes transactions for one address.
type nonceEntry struct {
	mu      sync.Mutex
	nonce   uint64
	initted bool
}

// nonceMgr is the global per-address nonce manager.
// It prevents "replacement transaction underpriced" errors by ensuring
// only one transaction is in-flight per wallet at a time, with a
// locally-incremented nonce.
var nonceMgr = struct {
	mu      sync.Mutex
	entries map[common.Address]*nonceEntry
}{
	entries: make(map[common.Address]*nonceEntry),
}

// getEntry returns (or creates) the nonce entry for an address.
func getEntry(addr common.Address) *nonceEntry {
	nonceMgr.mu.Lock()
	defer nonceMgr.mu.Unlock()
	e, ok := nonceMgr.entries[addr]
	if !ok {
		e = &nonceEntry{}
		nonceMgr.entries[addr] = e
	}
	return e
}

// AcquireNonce locks the wallet and returns the next nonce to use.
// The caller MUST call ReleaseNonce when the transaction is sent (success or fail).
// On success pass `true` to advance the local counter; on failure pass `false`
// to re-sync from the chain on the next call.
func AcquireNonce(ctx context.Context, addr common.Address) (uint64, error) {
	if C == nil {
		return 0, fmt.Errorf("chain client not initialized")
	}

	e := getEntry(addr)
	e.mu.Lock() // held until ReleaseNonce

	if !e.initted {
		// First call — seed from chain
		pending, err := C.ethClient.PendingNonceAt(ctx, addr)
		if err != nil {
			e.mu.Unlock()
			return 0, fmt.Errorf("failed to get pending nonce for %s: %w", addr.Hex(), err)
		}
		e.nonce = pending
		e.initted = true
		util.Log.Debug("[nonce] Initialized nonce for %s: %d", addr.Hex(), pending)
	}

	return e.nonce, nil
}

// ReleaseNonce unlocks the wallet after a transaction attempt.
//   - success=true  → increment local nonce (tx was accepted by the mempool)
//   - success=false → reset to re-sync from chain next time
func ReleaseNonce(addr common.Address, success bool) {
	e := getEntry(addr)
	if success {
		e.nonce++
	} else {
		// Force re-sync on next AcquireNonce
		e.initted = false
	}
	e.mu.Unlock()
}
