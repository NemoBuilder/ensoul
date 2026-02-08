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

	// Build dimension coverage summary
	var dimCoverage strings.Builder
	currentDims := shell.GetDimensions()
	for dim, data := range currentDims {
		newCount := dimFrags[dim]
		dimCoverage.WriteString(fmt.Sprintf("  %s: score=%d (new fragments: %d)\n", dim, data.Score, newCount))
	}

	prompt := fmt.Sprintf(`You are the Ensouling engine for Ensoul, a decentralized soul construction protocol.
You perform "soul condensation" — merging new verified fragments into an existing soul profile.

=== CURRENT SOUL ===
Handle: @%s
Stage: %s
DNA Version: v%d
Seed Summary: %s

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
		shell.SoulPrompt, dimCoverage.String(),
		len(fragments), fragList.String(),
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
