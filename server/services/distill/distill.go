// Package distill — the heart of V4.
//
// Pipeline:
//   1. ingest    — turn a raw file/url into normalised UTF-8 text segments
//   2. extract   — ask LLM to emit nodes + edges (JSON schema below)
//   3. confidence — derive a 0..1 score per atom from the LLM signal +
//                   source quality + cross-corroboration
//   4. align     — call alignment.Align() to dedupe entities
//   5. persist   — write Atom rows (caller-supplied tx)
//
// Phase 1.0 ships a minimal in-process, synchronous orchestrator suitable
// for local debugging. Phase 1.x swaps to an async worker pool with progress
// pushed over WebSocket. The Run() signature is stable across both.
package distill

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services/alignment"
)

// Job is the input contract for one distillation request.
type Job struct {
	GalaxyID   uuid.UUID
	SourceID   uuid.UUID
	ContribID  uuid.UUID
	Text       string // already-normalised UTF-8 (caller did ingest)
	Hints      Hints
}

// Hints are optional knobs that a Founder/Curator can set per-galaxy to bias
// extraction (e.g. force person-only nodes, set preferred edge vocabulary).
type Hints struct {
	NodeTypes   []string // e.g. ["person","work","concept"]
	EdgeVocab   []string // e.g. ["authored","cites","influenced_by"]
	MaxNodes    int      // 0 = no cap
}

// Result is what one Job emits before persistence.
type Result struct {
	Nodes []NodeDraft
	Edges []EdgeDraft
}

// NodeDraft is a candidate node returned by the extractor — not yet aligned
// or persisted.
type NodeDraft struct {
	Label      string
	NodeType   string
	Summary    string
	Confidence float64
	Provenance string // short quoted span supporting this node
}

// EdgeDraft references nodes BY INDEX into the Result.Nodes slice,
// so the orchestrator can rewire to persisted IDs after alignment.
type EdgeDraft struct {
	HeadIdx    int
	TailIdx    int
	Label      string
	Dir        string  // "directed" | "undirected"
	Confidence float64
	Provenance string
}

// Run executes the full pipeline for one job: extract via LLM → align
// nodes against existing canonical atoms → persist Nodes (then Edges so
// edge head/tail can reference real Atom IDs) inside the supplied tx.
//
// Returns the extraction result (with NodeDrafts updated to carry their
// new/aligned Atom ID via NodeDraft.Label kept as-is + a parallel idMap
// returned via Result.persistedIDs — kept implicit; the caller usually
// only cares that things landed).
func Run(ctx context.Context, db *gorm.DB, job Job) (*Result, error) {
	if job.Text == "" {
		return nil, errors.New("distill.Run: empty text")
	}
	res, err := extract(ctx, job)
	if err != nil {
		return nil, err
	}

	// Persist: nodes first → record their resulting Atom IDs in order →
	// then edges using those IDs.
	nodeIDs := make([]uuid.UUID, len(res.Nodes))
	err = db.Transaction(func(tx *gorm.DB) error {
		for i, n := range res.Nodes {
			al, _ := alignment.Align(ctx, tx, job.GalaxyID, n.Label, n.NodeType)
			atom := models.Atom{
				GalaxyID:    job.GalaxyID,
				SourceID:    job.SourceID,
				ContribID:   job.ContribID,
				Kind:        "node",
				NodeLabel:   n.Label,
				NodeType:    n.NodeType,
				NodeSummary: n.Summary,
				Confidence:  n.Confidence,
				Status:      models.AtomStatusAccepted,
			}
			if al.CanonicalID != nil {
				atom.AlignedTo = al.CanonicalID
				atom.Ambiguous = al.Ambiguous
			}
			if err := tx.Create(&atom).Error; err != nil {
				return fmt.Errorf("persist node %d: %w", i, err)
			}
			nodeIDs[i] = atom.ID
		}
		for i, e := range res.Edges {
			head := nodeIDs[e.HeadIdx]
			tail := nodeIDs[e.TailIdx]
			atom := models.Atom{
				GalaxyID:   job.GalaxyID,
				SourceID:   job.SourceID,
				ContribID:  job.ContribID,
				Kind:       "edge",
				HeadNodeID: &head,
				TailNodeID: &tail,
				EdgeLabel:  e.Label,
				EdgeDir:    e.Dir,
				Confidence: e.Confidence,
				Status:     models.AtomStatusAccepted,
			}
			if err := tx.Create(&atom).Error; err != nil {
				return fmt.Errorf("persist edge %d: %w", i, err)
			}
		}
		// Roll counters on the Galaxy row.
		return tx.Model(&models.Galaxy{}).
			Where("id = ?", job.GalaxyID).
			Updates(map[string]interface{}{
				"node_count": gorm.Expr("node_count + ?", len(res.Nodes)),
				"edge_count": gorm.Expr("edge_count + ?", len(res.Edges)),
				"atom_count": gorm.Expr("atom_count + ?", len(res.Nodes)+len(res.Edges)),
			}).Error
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
