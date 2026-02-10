package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// EnsoulingResult holds the LLM output for a soul condensation.
type EnsoulingResult struct {
	NewPrompt   string                          `json:"new_prompt"`
	Dimensions  map[string]models.DimensionData `json:"dimensions"`
	SummaryDiff string                          `json:"summary_diff"`
}

// TriggerEnsouling performs the soul condensation process.
// Merges new accepted fragments into the soul prompt and updates the DNA.
func TriggerEnsouling(shell *models.Shell) {
	// Get unmerged accepted fragments
	var fragments []models.Fragment
	database.DB.Where("shell_id = ? AND status = ? AND ensouling_id IS NULL",
		shell.ID, models.FragStatusAccepted).
		Order("created_at ASC").
		Find(&fragments)

	if len(fragments) == 0 {
		return
	}

	// Create ensouling record
	ensouling := &models.Ensouling{
		ShellID:     shell.ID,
		VersionFrom: shell.DNAVersion,
		VersionTo:   shell.DNAVersion + 1,
		FragsMerged: len(fragments),
	}

	// Perform ensouling via LLM or fallback
	var result *EnsoulingResult
	var err error

	if config.Cfg.LLMAPIKey != "" {
		result, err = ensoulWithLLM(shell, fragments)
		if err != nil {
			util.Log.Warn("[ensouling] LLM ensouling failed, using fallback: %v", err)
			result = ensoulFallback(shell, fragments)
		}
	} else {
		result = ensoulFallback(shell, fragments)
	}

	ensouling.NewPrompt = result.NewPrompt
	ensouling.SummaryDiff = result.SummaryDiff

	if err := database.DB.Create(ensouling).Error; err != nil {
		util.Log.Error("[ensouling] Failed to create ensouling record: %v", err)
		return
	}

	// Mark fragments as merged
	fragIDs := make([]interface{}, len(fragments))
	for i, f := range fragments {
		fragIDs[i] = f.ID
	}
	database.DB.Model(&models.Fragment{}).
		Where("id IN ?", fragIDs).
		Update("ensouling_id", ensouling.ID)

	// Update shell
	shell.DNAVersion++
	shell.SoulPrompt = result.NewPrompt

	updateFields := map[string]interface{}{
		"dna_version": shell.DNAVersion,
		"soul_prompt": result.NewPrompt,
	}

	// Update dimensions if provided by LLM
	if result.Dimensions != nil {
		dimsJSON, _ := json.Marshal(result.Dimensions)
		var dimsMap models.JSON
		json.Unmarshal(dimsJSON, &dimsMap)
		shell.Dimensions = dimsMap
		updateFields["dimensions"] = dimsMap
	}

	database.DB.Model(shell).Updates(updateFields)

	// Update stage
	UpdateShellStage(shell)

	// Update agentURI on-chain if this shell is linked to an on-chain agent
	if shell.AgentID != nil {
		go func() {
			ctx := context.Background()
			agentId := new(big.Int).SetUint64(*shell.AgentID)
			txHash, err := chain.UpdateSoulURI(
				ctx, agentId, shell.Handle, shell.AvatarURL,
				shell.SeedSummary, shell.Stage, shell.DNAVersion,
			)
			if err != nil {
				util.Log.Error("[ensouling] Failed to update agentURI on-chain for @%s: %v", shell.Handle, err)
				return
			}
			if txHash != "" {
				database.DB.Model(ensouling).Update("tx_hash", txHash)
				util.Log.Debug("[ensouling] On-chain URI updated for @%s: tx=%s", shell.Handle, txHash)
			}
		}()
	}

	util.Log.Info("[ensouling] Completed for @%s: v%d -> v%d, merged %d fragments",
		shell.Handle, ensouling.VersionFrom, ensouling.VersionTo, len(fragments))
}

// ensoulWithLLM performs soul condensation using the LLM.
func ensoulWithLLM(shell *models.Shell, fragments []models.Fragment) (*EnsoulingResult, error) {
	// Build fragment list text
	var fragList strings.Builder
	dimFrags := make(map[string]int)
	for i, f := range fragments {
		fragList.WriteString(fmt.Sprintf("[%d] Dimension: %s | Confidence: %.2f\n%s\n\n",
			i+1, f.Dimension, f.Confidence, f.Content))
		dimFrags[f.Dimension]++
	}

	// Build dimension coverage summary with actual fragment counts
	var dimCoverage strings.Builder
	currentDims := shell.GetDimensions()
	allDimensions := []string{"personality", "knowledge", "stance", "style", "relationship", "timeline"}
	for _, dim := range allDimensions {
		data := currentDims[dim]
		newCount := dimFrags[dim]

		// Count total accepted fragments for this dimension
		var totalAccepted int64
		database.DB.Model(&models.Fragment{}).
			Where("shell_id = ? AND dimension = ? AND status = ?",
				shell.ID, dim, models.FragStatusAccepted).
			Count(&totalAccepted)

		dimCoverage.WriteString(fmt.Sprintf("  %s: current_score=%d, total_accepted_fragments=%d, new_fragments_this_batch=%d\n",
			dim, data.Score, totalAccepted, newCount))
	}

	// Determine depth tier based on follower count
	followers := getFollowers(*shell)
	var depthTier, scoringGuide string
	switch {
	case followers >= 1_000_000:
		depthTier = fmt.Sprintf("MEGA (%d followers) — extremely rich public data, needs 80+ fragments per dimension to reach high scores", followers)
		scoringGuide = `  0-5:   Almost no data (0-2 fragments, only seed info)
  5-12:  Minimal data (3-8 fragments, surface-level)
  12-25: Basic coverage (9-20 fragments, some evidence)
  25-40: Moderate coverage (21-40 fragments, multiple angles)
  40-55: Good coverage (41-60 fragments, detailed with citations)
  55-70: Strong coverage (61-80 fragments, comprehensive)
  70-85: Excellent coverage (81-120 fragments, deep multi-source)
  85-100: Near-complete (120+ fragments, exhaustive — rarely achievable)`
	case followers >= 100_000:
		depthTier = fmt.Sprintf("LARGE (%d followers) — rich public data, needs 50+ fragments per dimension for high scores", followers)
		scoringGuide = `  0-8:   Almost no data (0-2 fragments, only seed info)
  8-18:  Minimal data (3-6 fragments, surface-level)
  18-30: Basic coverage (7-15 fragments, some evidence)
  30-45: Moderate coverage (16-30 fragments, multiple angles)
  45-60: Good coverage (31-50 fragments, detailed with citations)
  60-75: Strong coverage (51-70 fragments, comprehensive)
  75-90: Excellent coverage (70+ fragments, deep multi-source)
  90-100: Near-complete (100+ fragments, exhaustive — rarely achievable)`
	case followers >= 10_000:
		depthTier = fmt.Sprintf("MEDIUM (%d followers) — moderate public data, needs 30+ fragments per dimension for high scores", followers)
		scoringGuide = `  0-10:  Almost no data (0-2 fragments, only seed info)
  10-20: Minimal data (3-5 fragments, surface-level)
  20-35: Basic coverage (6-12 fragments, some evidence)
  35-50: Moderate coverage (13-25 fragments, multiple angles)
  50-65: Good coverage (26-40 fragments, detailed with citations)
  65-80: Strong coverage (41-55 fragments, comprehensive)
  80-90: Excellent coverage (55+ fragments, deep analysis)
  90-100: Near-complete (70+ fragments, exhaustive — rarely achievable)`
	case followers >= 1_000:
		depthTier = fmt.Sprintf("SMALL (%d followers) — limited public data, needs 15+ fragments per dimension for high scores", followers)
		scoringGuide = `  0-12:  Almost no data (0-2 fragments, only seed info)
  12-25: Minimal data (3-4 fragments, surface-level)
  25-40: Basic coverage (5-8 fragments, some evidence)
  40-55: Moderate coverage (9-15 fragments, multiple angles)
  55-70: Good coverage (16-25 fragments, detailed)
  70-85: Strong coverage (26-35 fragments, comprehensive)
  85-95: Excellent coverage (35+ fragments, deep analysis)
  95-100: Near-complete (50+ fragments, exhaustive)`
	default:
		depthTier = fmt.Sprintf("MICRO (%d followers) — very limited public data, needs 8+ fragments per dimension for high scores", followers)
		scoringGuide = `  0-15:  Almost no data (0-1 fragments, only seed info)
  15-30: Minimal data (2-3 fragments, surface-level)
  30-50: Basic coverage (4-6 fragments, some evidence)
  50-65: Moderate coverage (7-10 fragments, multiple angles)
  65-80: Good coverage (11-15 fragments, detailed)
  80-90: Strong coverage (16-20 fragments, comprehensive)
  90-95: Excellent coverage (20+ fragments, thorough analysis)
  95-100: Near-complete (30+ fragments, exhaustive)`
	}

	prompt := fmt.Sprintf(`You are the Ensouling engine for Ensoul, a decentralized soul construction protocol.
You perform "soul condensation" — merging new verified fragments into an existing soul profile.

=== CURRENT SOUL ===
Handle: @%s
Stage: %s
DNA Version: v%d
Seed Summary: %s
Depth Tier: %s

=== CURRENT SYSTEM PROMPT ===
%s

=== CURRENT DIMENSION SCORES ===
%s

=== NEW FRAGMENTS TO MERGE (total: %d) ===
%s

=== YOUR TASK ===
1. Carefully analyze each new fragment
2. Integrate the insights into the existing soul profile
3. Produce an UPDATED System Prompt that incorporates the new knowledge
4. Update the dimension scores (each dimension: 0-100)
5. Write a brief summary of what changed

=== DIMENSION SCORING RULES (CRITICAL) ===
The score measures OUR DATA COVERAGE — how thoroughly we have mapped this person's soul.
It does NOT measure the person's trait strength or fame.

This soul's depth tier is: %s
Use the scoring guide below that matches this tier:
%s

IMPORTANT:
- Never increase a score by more than 15 points in a single ensouling.
- If a dimension received 0 new fragments, its score should stay the same or decrease slightly.
- If the current score is HIGHER than what the scoring guide suggests for the actual fragment count,
  you MUST correct it downward.

The System Prompt should:
- Maintain the soul's voice and personality
- Incorporate new insights naturally (not just appending bullet points)
- Be structured as a character prompt suitable for LLM conversation
- Begin with "You are the digital soul of @%s."
- Include personality traits, knowledge areas, opinions, and communication style
- Be comprehensive but concise (aim for 500-1000 words)

Respond in JSON format ONLY:
{
  "new_prompt": "You are the digital soul of @%s...",
  "dimensions": {
    "personality": {"score": 25, "summary": "..."},
    "knowledge": {"score": 18, "summary": "..."},
    "stance": {"score": 30, "summary": "..."},
    "style": {"score": 20, "summary": "..."},
    "relationship": {"score": 12, "summary": "..."},
    "timeline": {"score": 8, "summary": "..."}
  },
  "summary_diff": "Brief description of what changed in this version..."
}`,
		shell.Handle, shell.Stage, shell.DNAVersion, shell.SeedSummary,
		depthTier,
		shell.SoulPrompt, dimCoverage.String(),
		len(fragments), fragList.String(),
		depthTier, scoringGuide,
		shell.Handle, shell.Handle)

	var result EnsoulingResult
	err := CallLLMJSON([]ChatMessage{
		{Role: "system", Content: "You are a precise soul construction engine. Output valid JSON only, no markdown."},
		{Role: "user", Content: prompt},
	}, 4000, 0.4, &result)

	if err != nil {
		return nil, err
	}

	util.Log.Debug("[ensouling] LLM ensouling for @%s: %s", shell.Handle, result.SummaryDiff)
	return &result, nil
}

// ensoulFallback creates an updated soul prompt by simple concatenation when LLM is unavailable.
func ensoulFallback(shell *models.Shell, fragments []models.Fragment) *EnsoulingResult {
	prompt := shell.SoulPrompt + "\n\n--- Updated Knowledge (DNA v" +
		fmt.Sprintf("%d", shell.DNAVersion+1) + ") ---\n\n"

	// Group fragments by dimension
	dimFrags := make(map[string][]string)
	for _, f := range fragments {
		dimFrags[f.Dimension] = append(dimFrags[f.Dimension], f.Content)
	}

	for dim, contents := range dimFrags {
		prompt += fmt.Sprintf("[%s]\n", dim)
		for _, content := range contents {
			prompt += fmt.Sprintf("- %s\n", content)
		}
		prompt += "\n"
	}

	return &EnsoulingResult{
		NewPrompt:   prompt,
		SummaryDiff: fmt.Sprintf("Merged %d new fragments across %d dimensions. DNA upgraded from v%d to v%d.", len(fragments), len(dimFrags), shell.DNAVersion, shell.DNAVersion+1),
	}
}
