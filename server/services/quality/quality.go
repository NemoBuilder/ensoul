// Package quality — maturity / diversity / confidence scoring for galaxies.
//
// These scores feed two things:
//   1. The galaxy listing page (sort by "most mature").
//   2. The LaunchReady gate in Phase 3 (fair-launch eligibility).
//
// Phase 1.0 keeps the math intentionally simple and deterministic so the
// scores are auditable. Phase 1.x will introduce embeddings-based diversity
// once pgvector is enabled.
package quality

import (
	"math"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/google/uuid"
)

// Thresholds for LaunchReady. Tuned by feel; revisit after testnet data.
const (
	LaunchMinAtoms        = 200
	LaunchMinContributors = 5
	LaunchMinMaturity     = 60.0 // 0..100
	LaunchMinConfidence   = 0.65
)

// Snapshot bundles the rolled-up numbers we cache on Galaxy.
type Snapshot struct {
	AtomCount       int
	NodeCount       int
	EdgeCount       int
	ContribCount    int
	MaturityScore   float64 // 0..100
	DiversityScore  float64 // 0..100
	ConfidenceAvg   float64 // 0..1
	AntiFarmingPass bool
}

// Recompute reads accepted atoms for one galaxy and writes a fresh Snapshot
// back to the Galaxy row. Returns the snapshot for the caller.
//
// Cheap enough to run inline after each successful distill in Phase 1; in
// Phase 1.x move to a debounced background job per galaxy.
func Recompute(galaxyID uuid.UUID) (Snapshot, error) {
	db := database.DB

	var snap Snapshot

	// Atom totals + average confidence in one pass per kind.
	type agg struct {
		Count int
		Avg   float64
	}
	var na, ea agg
	db.Model(&models.Atom{}).
		Select("COUNT(*) as count, COALESCE(AVG(confidence), 0) as avg").
		Where("galaxy_id = ? AND kind = 'node' AND status = ?", galaxyID, models.AtomStatusAccepted).
		Scan(&na)
	db.Model(&models.Atom{}).
		Select("COUNT(*) as count, COALESCE(AVG(confidence), 0) as avg").
		Where("galaxy_id = ? AND kind = 'edge' AND status = ?", galaxyID, models.AtomStatusAccepted).
		Scan(&ea)
	snap.NodeCount = na.Count
	snap.EdgeCount = ea.Count
	snap.AtomCount = snap.NodeCount + snap.EdgeCount

	if snap.AtomCount > 0 {
		snap.ConfidenceAvg = (na.Avg*float64(na.Count) + ea.Avg*float64(ea.Count)) /
			float64(snap.AtomCount)
	}

	// Distinct contributors on accepted atoms.
	var contribCount int64
	db.Model(&models.Atom{}).
		Where("galaxy_id = ? AND status = ?", galaxyID, models.AtomStatusAccepted).
		Distinct("contrib_id").
		Count(&contribCount)
	snap.ContribCount = int(contribCount)

	// Diversity: distinct node_type / total node_count. Bonus if more than
	// 6 distinct types (rewards multi-faceted galaxies).
	var typeCount int64
	db.Model(&models.Atom{}).
		Where("galaxy_id = ? AND kind = 'node' AND status = ?", galaxyID, models.AtomStatusAccepted).
		Distinct("node_type").
		Count(&typeCount)
	if snap.NodeCount > 0 {
		ratio := float64(typeCount) / math.Max(float64(snap.NodeCount), 1)
		// Map 0..0.5 ratio onto 0..100 with diminishing returns.
		snap.DiversityScore = math.Min(100, ratio*200+float64(typeCount)*2)
	}

	// Maturity: weighted combo of size, confidence, diversity, contributor count.
	sizeScore := math.Min(100, math.Log10(float64(snap.AtomCount)+1)*30) // ~100 at 1000 atoms
	confScore := snap.ConfidenceAvg * 100
	contribScore := math.Min(100, float64(snap.ContribCount)*10)         // 10 contribs = 100
	snap.MaturityScore =
		0.40*sizeScore +
			0.30*confScore +
			0.20*snap.DiversityScore +
			0.10*contribScore

	// Anti-farming sanity: no single contributor produced >70% of accepted atoms.
	snap.AntiFarmingPass = checkAntiFarming(galaxyID, snap.AtomCount)

	// Persist snapshot back to the Galaxy row.
	err := db.Model(&models.Galaxy{}).Where("id = ?", galaxyID).Updates(map[string]interface{}{
		"node_count":        snap.NodeCount,
		"edge_count":        snap.EdgeCount,
		"atom_count":        snap.AtomCount,
		"contrib_count":     snap.ContribCount,
		"maturity_score":    snap.MaturityScore,
		"diversity_score":   snap.DiversityScore,
		"confidence_avg":    snap.ConfidenceAvg,
		"anti_farming_pass": snap.AntiFarmingPass,
	}).Error
	return snap, err
}

func checkAntiFarming(galaxyID uuid.UUID, total int) bool {
	if total < 20 {
		// Too small to judge — give benefit of the doubt.
		return true
	}
	type row struct {
		ContribID uuid.UUID
		Count     int
	}
	var top row
	database.DB.Model(&models.Atom{}).
		Select("contrib_id, COUNT(*) as count").
		Where("galaxy_id = ? AND status = ?", galaxyID, models.AtomStatusAccepted).
		Group("contrib_id").
		Order("count DESC").
		Limit(1).
		Scan(&top)
	return float64(top.Count)/float64(total) <= 0.70
}

// LaunchReady reports whether a galaxy currently passes the fair-launch gate.
func LaunchReady(snap Snapshot) bool {
	return snap.AtomCount >= LaunchMinAtoms &&
		snap.ContribCount >= LaunchMinContributors &&
		snap.MaturityScore >= LaunchMinMaturity &&
		snap.ConfidenceAvg >= LaunchMinConfidence &&
		snap.AntiFarmingPass
}
