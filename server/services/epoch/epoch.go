// Package epoch — V4 epoch roll-up.
//
// Every N hours we pack the day's accepted atoms (that don't yet have an
// epoch_id) into a Merkle tree, write the root to Postgres, and (in Phase
// 2.1) push the root on-chain. The on-chain write is intentionally NOT
// implemented here — that lives behind a chain client interface so tests
// can stub it.
//
// Phase 1.x ships only the off-chain assembly + DB roll-up so the UI can
// already show "epoch #42 — 372 atoms — root 0xabc…".
package epoch

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ensoul-labs/ensoul-server/util/merkle"
)

// canonicalAtom is the stable JSON shape that we hash for a Merkle leaf.
// Field order is fixed by the struct layout; keep it stable across releases
// or the chain root will diverge from off-chain verifiers.
type canonicalAtom struct {
	ID         string  `json:"id"`
	GalaxyID   string  `json:"galaxy_id"`
	SourceID   string  `json:"source_id"`
	ContribID  string  `json:"contrib_id"`
	Kind       string  `json:"kind"`
	NodeLabel  string  `json:"node_label,omitempty"`
	NodeType   string  `json:"node_type,omitempty"`
	HeadNodeID string  `json:"head_node_id,omitempty"`
	TailNodeID string  `json:"tail_node_id,omitempty"`
	EdgeLabel  string  `json:"edge_label,omitempty"`
	Confidence float64 `json:"confidence"`
	CreatedAt  int64   `json:"created_at"` // unix seconds
}

func canonical(a models.Atom) []byte {
	c := canonicalAtom{
		ID:         a.ID.String(),
		GalaxyID:   a.GalaxyID.String(),
		SourceID:   a.SourceID.String(),
		ContribID:  a.ContribID.String(),
		Kind:       a.Kind,
		NodeLabel:  a.NodeLabel,
		NodeType:   a.NodeType,
		EdgeLabel:  a.EdgeLabel,
		Confidence: a.Confidence,
		CreatedAt:  a.CreatedAt.Unix(),
	}
	if a.HeadNodeID != nil {
		c.HeadNodeID = a.HeadNodeID.String()
	}
	if a.TailNodeID != nil {
		c.TailNodeID = a.TailNodeID.String()
	}
	b, _ := json.Marshal(c)
	return b
}

// Result describes one successful epoch.
type Result struct {
	EpochID    uuid.UUID
	Index      int64
	Root       merkle.Hash
	AtomCount  int
	GalaxyID   uuid.UUID // zero = global (cross-galaxy) epoch
}

// Build assembles one epoch for the given galaxy (or pass uuid.Nil for a
// global epoch across all galaxies). Atoms are selected as: status=accepted
// AND epoch_id IS NULL. Returns ErrNoAtoms if there is nothing to roll up.
//
// Side effects on success:
//   1. Insert one models.Epoch row with the root.
//   2. Update each Atom: set epoch_id + merkle_leaf.
//
// On-chain push is the caller's responsibility (chain/epoch.go in Phase 2.1).
func Build(ctx context.Context, db *gorm.DB, galaxyID uuid.UUID) (*Result, error) {
	q := db.WithContext(ctx).
		Model(&models.Atom{}).
		Where("status = ? AND epoch_id IS NULL", models.AtomStatusAccepted).
		Order("created_at ASC, id ASC")
	if galaxyID != uuid.Nil {
		q = q.Where("galaxy_id = ?", galaxyID)
	}

	var atoms []models.Atom
	if err := q.Find(&atoms).Error; err != nil {
		return nil, fmt.Errorf("epoch.Build: load atoms: %w", err)
	}
	if len(atoms) == 0 {
		return nil, ErrNoAtoms
	}

	// Deterministic ordering: sort by created_at then id so the off-chain
	// rebuild produces the same root even if DB rows came back unordered.
	sort.SliceStable(atoms, func(i, j int) bool {
		if atoms[i].CreatedAt.Equal(atoms[j].CreatedAt) {
			return atoms[i].ID.String() < atoms[j].ID.String()
		}
		return atoms[i].CreatedAt.Before(atoms[j].CreatedAt)
	})

	leaves := make([]merkle.Hash, len(atoms))
	for i, a := range atoms {
		leaves[i] = merkle.LeafFromBytes(canonical(a))
	}
	root := merkle.Root(leaves)

	// Next sequential index per galaxy (or globally when galaxyID = nil).
	var nextIdx int64
	idxQ := db.Model(&models.Epoch{})
	if galaxyID != uuid.Nil {
		idxQ = idxQ.Where("galaxy_id = ?", galaxyID)
	} else {
		idxQ = idxQ.Where("galaxy_id IS NULL")
	}
	idxQ.Select("COALESCE(MAX(index), 0) + 1").Scan(&nextIdx)
	if nextIdx == 0 {
		nextIdx = 1
	}

	epochRow := models.Epoch{
		Index:     nextIdx,
		Root:      root.HashHex(),
		AtomCount: len(atoms),
		ClosedAt:  time.Now(),
	}
	if galaxyID != uuid.Nil {
		epochRow.GalaxyID = &galaxyID
	}

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&epochRow).Error; err != nil {
			return err
		}
		// Bulk-update atoms with their epoch_id + per-atom merkle_leaf hex.
		// Done one-by-one in this MVP; switch to a single CASE-WHEN UPDATE if
		// epochs ever exceed ~10k atoms.
		for i, a := range atoms {
			if err := tx.Model(&models.Atom{}).Where("id = ?", a.ID).
				Updates(map[string]interface{}{
					"epoch_id":    epochRow.ID,
					"merkle_leaf": leaves[i].HashHex(),
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &Result{
		EpochID:   epochRow.ID,
		Index:     epochRow.Index,
		Root:      root,
		AtomCount: len(atoms),
		GalaxyID:  galaxyID,
	}, nil
}

// ErrNoAtoms is returned by Build when there are no pending atoms.
var ErrNoAtoms = fmt.Errorf("epoch: no pending atoms")

// ProofResult is what RebuildProof returns — leaf + path hex strings, plus
// the atom's 0-based index inside the canonical leaf list.
type ProofResult struct {
	Leaf  string   `json:"leaf"`
	Index int      `json:"index"`
	Path  []string `json:"path"`
}

// RebuildProof reconstructs the Merkle tree for a closed epoch using the
// same canonical ordering Build() used, then returns the proof path for the
// requested atom. Errors if the atom isn't in this epoch.
func RebuildProof(ctx context.Context, db *gorm.DB, epochID uuid.UUID, atomID uuid.UUID) (*ProofResult, error) {
	var atoms []models.Atom
	if err := db.WithContext(ctx).
		Where("epoch_id = ?", epochID).
		Order("created_at ASC, id ASC").
		Find(&atoms).Error; err != nil {
		return nil, fmt.Errorf("load atoms: %w", err)
	}
	if len(atoms) == 0 {
		return nil, fmt.Errorf("epoch has no atoms")
	}

	// Reapply Build's deterministic sort (created_at then id-string).
	sort.SliceStable(atoms, func(i, j int) bool {
		if atoms[i].CreatedAt.Equal(atoms[j].CreatedAt) {
			return atoms[i].ID.String() < atoms[j].ID.String()
		}
		return atoms[i].CreatedAt.Before(atoms[j].CreatedAt)
	})

	idx := -1
	leaves := make([]merkle.Hash, len(atoms))
	for i, a := range atoms {
		leaves[i] = merkle.LeafFromBytes(canonical(a))
		if a.ID == atomID {
			idx = i
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("atom %s not in epoch %s", atomID, epochID)
	}

	path := merkle.Proof(leaves, idx)
	out := &ProofResult{
		Leaf:  leaves[idx].HashHex(),
		Index: idx,
		Path:  make([]string, len(path)),
	}
	for i, h := range path {
		out.Path[i] = h.HashHex()
	}
	return out, nil
}

// PushOnChain submits the epoch root to EnsoulEpochRegistry and updates
// the Epoch row's chain_tx_hash / chain_status / pushed_at. Safe to call
// in a goroutine after Build returns.
//
// galaxyID = uuid.Nil maps to bytes32(0) on-chain (the global epoch stream).
// If the registry address isn't configured we log + mark "skipped" instead
// of failing — local dev shouldn't need a deployed contract.
func PushOnChain(ctx context.Context, db *gorm.DB, epochID uuid.UUID, galaxyID uuid.UUID, index int64, root merkle.Hash, atomCount int) error {
	var gid [32]byte
	if galaxyID != uuid.Nil {
		b, _ := galaxyID.MarshalBinary() // 16 bytes
		copy(gid[0:16], b)               // left-aligned so on-chain id matches UUID byte order
	}
	var rootB [32]byte
	copy(rootB[:], root[:])

	tx, err := chain.PushEpochRoot(ctx, gid, uint64(index), rootB, uint64(atomCount))
	if err != nil {
		// Not configured? mark and move on.
		if err == chain.ErrEpochRegistryNotConfigured {
			util.Log.Warn("[epoch] skip on-chain push (registry not configured) epoch=%s", epochID)
			return db.WithContext(ctx).Model(&models.Epoch{}).
				Where("id = ?", epochID).
				Update("chain_status", "skipped").Error
		}
		util.Log.Error("[epoch] on-chain push failed epoch=%s: %v", epochID, err)
		_ = db.WithContext(ctx).Model(&models.Epoch{}).
			Where("id = ?", epochID).
			Update("chain_status", "failed").Error
		return err
	}

	now := time.Now()
	return db.WithContext(ctx).Model(&models.Epoch{}).
		Where("id = ?", epochID).
		Updates(map[string]interface{}{
			"chain_tx_hash": tx,
			"chain_status":  "confirmed", // optimistic — receipt watcher can downgrade
			"pushed_at":     &now,
		}).Error
}
