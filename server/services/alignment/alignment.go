// Package alignment — cross-galaxy entity alignment.
//
// When a new Atom (node) is extracted, we ask: does this entity already exist
// in this galaxy (canonical merge) or in a different galaxy (cross-link)?
//
// Phase 1.0: text-similarity (exact + normalised label match) only. pgvector
// + LLM arbitration land in Phase 1.x once Postgres extension is enabled.
package alignment

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/ensoul-labs/ensoul-server/models"
)

// Result describes the alignment outcome for one incoming node.
type Result struct {
	// CanonicalID — the existing atom this one should merge into.
	// Nil means "create as new".
	CanonicalID *uuid.UUID
	// Ambiguous — true when multiple candidates were close enough that a
	// curator should review. The caller still creates the atom but flags it.
	Ambiguous bool
	// Score 0..1 — confidence of the alignment decision.
	Score float64
}

// Align inspects the atom's NodeLabel + NodeType and looks for an existing
// canonical atom in the same galaxy. Phase 1.0 implementation: case-insensitive
// label match within the same NodeType bucket.
func Align(_ context.Context, db *gorm.DB, galaxyID uuid.UUID, label, nodeType string) (Result, error) {
	label = strings.TrimSpace(strings.ToLower(label))
	if label == "" {
		return Result{}, nil
	}
	var candidates []models.Atom
	q := db.Where(
		"galaxy_id = ? AND kind = ? AND status = ? AND lower(node_label) = ?",
		galaxyID, "node", models.AtomStatusAccepted, label,
	)
	if nodeType != "" {
		q = q.Where("node_type = ?", nodeType)
	}
	if err := q.Limit(5).Find(&candidates).Error; err != nil {
		return Result{}, err
	}
	switch len(candidates) {
	case 0:
		return Result{}, nil
	case 1:
		id := candidates[0].ID
		return Result{CanonicalID: &id, Score: 1.0}, nil
	default:
		// Multiple exact matches → flag for curator. Pick the highest-confidence
		// canonical as the merge target so the graph stays connected.
		best := candidates[0]
		for _, a := range candidates[1:] {
			if a.Confidence > best.Confidence {
				best = a
			}
		}
		id := best.ID
		return Result{CanonicalID: &id, Ambiguous: true, Score: 0.7}, nil
	}
}
