package services

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/google/uuid"
)

// SubmitFragment processes a new fragment submission from a Claw.
func SubmitFragment(claw *models.Claw, handle, dimension, content string) (*models.Fragment, error) {
	// Find the target shell
	var shell models.Shell
	if err := database.DB.Where("handle = ?", handle).First(&shell).Error; err != nil {
		return nil, fmt.Errorf("soul @%s not found", handle)
	}

	// Create the fragment
	fragment := &models.Fragment{
		ShellID:   shell.ID,
		ClawID:    claw.ID,
		Dimension: dimension,
		Content:   content,
		Status:    models.FragStatusPending,
	}

	if err := database.DB.Create(fragment).Error; err != nil {
		return nil, fmt.Errorf("failed to create fragment: %w", err)
	}

	// Update claw submission count
	database.DB.Model(claw).Update("total_submitted", claw.TotalSubmitted+1)

	// Update shell total fragments count
	database.DB.Model(&shell).Update("total_frags", shell.TotalFrags+1)

	// Run curator review (async in production, sync for MVP)
	go func() {
		ReviewFragment(fragment, &shell)
	}()

	return fragment, nil
}

// ReviewFragment runs the Curator AI to review a fragment using LLM analysis.
func ReviewFragment(fragment *models.Fragment, shell *models.Shell) {
	// Fetch existing accepted fragments for this shell+dimension to check for duplicates
	var existingFrags []models.Fragment
	database.DB.Where("shell_id = ? AND dimension = ? AND status = ? AND id != ?",
		shell.ID, fragment.Dimension, models.FragStatusAccepted, fragment.ID).
		Order("created_at DESC").Limit(20).Find(&existingFrags)

	// If LLM is not configured, auto-accept with default confidence
	if config.Cfg.LLMAPIKey == "" {
		log.Println("[curator] LLM not configured, auto-accepting fragment")
		acceptFragment(fragment, shell, 0.75)
		return
	}

	// Build the existing fragments context
	var existingCtx string
	if len(existingFrags) > 0 {
		var sb strings.Builder
		for i, f := range existingFrags {
			sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, f.Content))
		}
		existingCtx = sb.String()
	} else {
		existingCtx = "(No existing fragments for this dimension yet)"
	}

	curatorPrompt := fmt.Sprintf(`You are the Curator for Ensoul, a decentralized soul construction protocol.
Your job is to review fragment submissions that claim to describe aspects of @%s's personality/behavior.

=== SOUL ===
Handle: @%s
Stage: %s
Seed Summary: %s

=== DIMENSION ===
%s

=== EXISTING ACCEPTED FRAGMENTS (same dimension) ===
%s

=== NEW FRAGMENT TO REVIEW ===
%s

=== REVIEW CRITERIA ===
1. SUBSTANCE: Does this fragment contain genuine insight or analysis (not just copy-pasted facts)?
2. UNIQUENESS: Is it semantically distinct from existing accepted fragments?
3. RELEVANCE: Does it belong to the "%s" dimension?
4. QUALITY: Is it well-articulated and specific enough to be useful?

Respond in JSON format ONLY:
{
  "accept": true/false,
  "confidence": 0.0-1.0,
  "reason": "Brief explanation of your decision"
}`,
		shell.Handle, shell.Handle, shell.Stage, shell.SeedSummary,
		fragment.Dimension, existingCtx, fragment.Content, fragment.Dimension)

	var result struct {
		Accept     bool    `json:"accept"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}

	err := CallLLMJSON([]ChatMessage{
		{Role: "system", Content: "You are a strict but fair content curator. Output valid JSON only."},
		{Role: "user", Content: curatorPrompt},
	}, 500, 0.2, &result)

	if err != nil {
		log.Printf("[curator] LLM review failed, auto-accepting: %v", err)
		acceptFragment(fragment, shell, 0.70)
		return
	}

	log.Printf("[curator] Review for @%s/%s: accept=%v, confidence=%.2f, reason=%s",
		shell.Handle, fragment.Dimension, result.Accept, result.Confidence, result.Reason)

	if result.Accept {
		acceptFragment(fragment, shell, result.Confidence)
	} else {
		rejectFragment(fragment, result.Confidence, result.Reason)
	}
}

// acceptFragment marks a fragment as accepted and triggers downstream effects.
func acceptFragment(fragment *models.Fragment, shell *models.Shell, confidence float64) {
	fragment.Status = models.FragStatusAccepted
	fragment.Confidence = confidence
	database.DB.Save(fragment)

	// Update shell accepted count
	database.DB.Model(shell).UpdateColumn("accepted_frags", shell.AcceptedFrags+1)

	// Update claw accepted count
	database.DB.Model(&models.Claw{}).Where("id = ?", fragment.ClawID).
		UpdateColumn("total_accepted", database.DB.Raw("total_accepted + 1"))

	// Update unique claws count for this shell
	var uniqueClaws int64
	database.DB.Model(&models.Fragment{}).
		Where("shell_id = ? AND status = ?", shell.ID, models.FragStatusAccepted).
		Distinct("claw_id").Count(&uniqueClaws)
	database.DB.Model(shell).Update("total_claws", uniqueClaws)

	// Update shell stage
	shell.AcceptedFrags++
	UpdateShellStage(shell)

	// Check if ensouling threshold is reached
	CheckEnsoulingThreshold(shell)

	// Submit reputation feedback on-chain via Claw's independent wallet
	submitOnChainFeedback(fragment, shell)
}

// rejectFragment marks a fragment as rejected.
func rejectFragment(fragment *models.Fragment, confidence float64, reason string) {
	fragment.Status = models.FragStatusRejected
	fragment.Confidence = confidence
	fragment.RejectReason = reason
	database.DB.Save(fragment)
}

// submitOnChainFeedback submits reputation feedback for an accepted fragment.
func submitOnChainFeedback(fragment *models.Fragment, shell *models.Shell) {
	if shell.AgentID == nil {
		return
	}

	go func() {
		// Load the Claw to get its encrypted private key
		var claw models.Claw
		if err := database.DB.First(&claw, "id = ?", fragment.ClawID).Error; err != nil {
			log.Printf("[services] Failed to load claw for feedback: %v", err)
			return
		}
		if claw.WalletPKEnc == "" {
			log.Printf("[services] Claw %s has no wallet key, skipping on-chain feedback", claw.Name)
			return
		}

		clawKey, err := chain.DecryptClawPrivateKey(claw.WalletPKEnc)
		if err != nil {
			log.Printf("[services] Failed to decrypt claw key: %v", err)
			return
		}

		ctx := context.Background()
		agentId := new(big.Int).SetUint64(*shell.AgentID)
		// Map confidence (0.0-1.0) to feedback value (0-100)
		feedbackValue := int64(fragment.Confidence * 100)
		txHash, err := chain.SubmitFeedback(ctx, clawKey, agentId, feedbackValue, fragment.Dimension, "fragment")
		if err != nil {
			log.Printf("[services] On-chain feedback failed for @%s by claw %s: %v", shell.Handle, claw.Name, err)
			return
		}
		// Store the feedback tx hash on the fragment
		database.DB.Model(fragment).Update("tx_hash", txHash)
		log.Printf("[services] On-chain feedback submitted for @%s: value=%d, tx=%s", shell.Handle, feedbackValue, txHash)
	}()
}

// ListFragments returns fragments with optional filters.
func ListFragments(handle, status, dimension, pageStr, limitStr string) (map[string]interface{}, error) {
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Fragment{}).Preload("Claw").Preload("Shell")

	// Apply filters
	if handle != "" {
		var shell models.Shell
		if err := database.DB.Where("handle = ?", handle).First(&shell).Error; err == nil {
			query = query.Where("shell_id = ?", shell.ID)
		}
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if dimension != "" {
		query = query.Where("dimension = ?", dimension)
	}

	// Count total
	var total int64
	query.Count(&total)

	// Fetch results
	var fragments []models.Fragment
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&fragments).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"fragments": fragments,
		"total":     total,
		"page":      page,
		"limit":     limit,
	}, nil
}

// GetFragmentByID returns a single fragment by its ID.
func GetFragmentByID(id string) (*models.Fragment, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid fragment ID")
	}

	var fragment models.Fragment
	if err := database.DB.Preload("Shell").Preload("Claw").Where("id = ?", uid).First(&fragment).Error; err != nil {
		return nil, err
	}

	return &fragment, nil
}

// CheckEnsoulingThreshold checks if a shell has enough new fragments to trigger ensouling.
func CheckEnsoulingThreshold(shell *models.Shell) {
	// Count accepted fragments since last ensouling
	var lastEnsouling models.Ensouling
	hasLastEnsouling := database.DB.Where("shell_id = ?", shell.ID).
		Order("created_at DESC").First(&lastEnsouling).Error == nil

	query := database.DB.Model(&models.Fragment{}).
		Where("shell_id = ? AND status = ?", shell.ID, models.FragStatusAccepted)

	if hasLastEnsouling {
		query = query.Where("created_at > ?", lastEnsouling.CreatedAt)
	}

	var newAccepted int64
	query.Where("ensouling_id IS NULL").Count(&newAccepted)

	// Threshold: 10 new accepted fragments
	if newAccepted >= 10 {
		TriggerEnsouling(shell)
	}
}
