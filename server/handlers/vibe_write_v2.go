package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ══════════════════════════════════════════════════════════════════════════════
// Vibe Write 2.0 Handlers — Tag-based Feed + Snipe
// ══════════════════════════════════════════════════════════════════════════════

// ──────────────────────────────────────────────────────────────────────────────
// Tags
// ──────────────────────────────────────────────────────────────────────────────

// VibeWriteGetTags handles GET /api/vibe-write/tags
// Returns all active tags with their accounts. Public endpoint.
func VibeWriteGetTags(c *gin.Context) {
	tags, defaults, err := services.GetAllTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tags":     tags,
		"defaults": defaults,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Feed
// ──────────────────────────────────────────────────────────────────────────────

// VibeWriteGetFeed handles GET /api/vibe-write/feed
// Returns aggregated tweets from selected tags. Public endpoint.
func VibeWriteGetFeed(c *gin.Context) {
	tagIDsStr := c.Query("tag_ids")
	if tagIDsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_ids parameter is required"})
		return
	}

	tagIDs := strings.Split(tagIDsStr, ",")
	cursor := c.Query("cursor")

	count := 20
	if countStr := c.Query("count"); countStr != "" {
		if n, err := strconv.Atoi(countStr); err == nil {
			count = n
		}
	}

	// Collect muted handles
	var mutedHandles []string
	if mutedStr := c.Query("muted"); mutedStr != "" {
		mutedHandles = strings.Split(mutedStr, ",")
	}

	// If user is logged in, auto-inject their muted list
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr != "" {
		userMuted, _ := services.GetUserMutedHandles(walletAddr)
		mutedHandles = append(mutedHandles, userMuted...)
	}

	result, err := services.BuildFeed(tagIDs, mutedHandles, cursor, count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// VibeWriteFeedRefresh handles GET /api/vibe-write/feed/refresh
// Forces a cache refresh for the specified tags. Public endpoint.
func VibeWriteFeedRefresh(c *gin.Context) {
	tagIDsStr := c.Query("tag_ids")
	if tagIDsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_ids parameter is required"})
		return
	}

	tagIDs := strings.Split(tagIDsStr, ",")

	newCount, err := services.RefreshTagFeed(tagIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"refreshed_tags": tagIDs,
		"new_tweets":     newCount,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// SSE Stream
// ──────────────────────────────────────────────────────────────────────────────

// VibeWriteFeedStream handles GET /api/vibe-write/feed/stream
// SSE endpoint for real-time tweet push. Public endpoint.
func VibeWriteFeedStream(c *gin.Context) {
	tagIDsStr := c.Query("tag_ids")
	if tagIDsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_ids parameter is required"})
		return
	}

	tagIDs := strings.Split(tagIDsStr, ",")
	if len(tagIDs) > 15 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 15 tags per SSE connection"})
		return
	}

	hub := services.GetSSEHub()
	if hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSE not available"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	ch, unsub := hub.Subscribe(tagIDs)
	defer unsub()

	// Heartbeat ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Max connection timer (2 hours)
	maxTimer := time.NewTimer(2 * time.Hour)
	defer maxTimer.Stop()

	clientGone := c.Request.Context().Done()
	flusher, _ := c.Writer.(http.Flusher)

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return // channel closed
			}
			c.SSEvent(event.Type, event.Data)
			if flusher != nil {
				flusher.Flush()
			}

		case <-ticker.C:
			c.SSEvent("heartbeat", gin.H{"ts": time.Now().Unix()})
			if flusher != nil {
				flusher.Flush()
			}

		case <-maxTimer.C:
			c.SSEvent("error", gin.H{"message": "max connection time reached, please reconnect"})
			if flusher != nil {
				flusher.Flush()
			}
			return

		case <-clientGone:
			return // client disconnected
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// User Tag Preferences
// ──────────────────────────────────────────────────────────────────────────────

// VibeWriteGetUserTags handles GET /api/vibe-write/user/tags
// Returns the user's selected tags. Requires session.
func VibeWriteGetUserTags(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	tagIDs, err := services.GetUserSelectedTags(walletAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tag_ids": tagIDs})
}

// VibeWriteUpdateUserTags handles PUT /api/vibe-write/user/tags
// Updates the user's selected tags. Requires session.
func VibeWriteUpdateUserTags(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		TagIDs []string `json:"tag_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_ids array is required"})
		return
	}

	if err := services.UpdateUserSelectedTags(walletAddr, req.TagIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "tag_ids": req.TagIDs})
}

// ──────────────────────────────────────────────────────────────────────────────
// User Muted Accounts
// ──────────────────────────────────────────────────────────────────────────────

// VibeWriteGetMuted handles GET /api/vibe-write/user/muted
// Returns the user's muted accounts. Requires session.
func VibeWriteGetMuted(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	handles, err := services.GetUserMutedHandles(walletAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"muted": handles})
}

// VibeWriteMuteAccount handles POST /api/vibe-write/user/muted
// Mutes an account for the user. Requires session.
func VibeWriteMuteAccount(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Handle string `json:"handle" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	if err := services.MuteAccount(walletAddr, req.Handle); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "muted", "handle": req.Handle})
}

// VibeWriteUnmuteAccount handles DELETE /api/vibe-write/user/muted/:handle
// Unmutes an account. Requires session.
func VibeWriteUnmuteAccount(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	handle := c.Param("handle")
	if handle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	if err := services.UnmuteAccount(walletAddr, handle); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "unmuted", "handle": handle})
}

// ──────────────────────────────────────────────────────────────────────────────
// Snipe (new version of reply generation)
// ──────────────────────────────────────────────────────────────────────────────

// VibeWriteSnipe handles POST /api/vibe-write/snipe
// Generates reply suggestions for a tweet. Requires Pro subscription.
func VibeWriteSnipe(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		TweetID      string `json:"tweet_id" binding:"required"`
		TweetText    string `json:"tweet_text" binding:"required"`
		AuthorHandle string `json:"author_handle" binding:"required"`
		TagID        string `json:"tag_id"`
		Language     string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tweet_id, tweet_text, and author_handle are required"})
		return
	}

	reply, err := services.Snipe(walletAddr, req.AuthorHandle, req.TweetID, req.TweetText, req.TagID, req.Language)
	if err != nil {
		// Distinguish between auth errors and other errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "subscription required") {
			c.JSON(http.StatusForbidden, gin.H{"error": errMsg})
			return
		}
		if strings.Contains(errMsg, "limit reached") {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, reply)
}

// ══════════════════════════════════════════════════════════════════════════════
// Admin: Vibe Write Tag CRUD
// ══════════════════════════════════════════════════════════════════════════════

// AdminVibeWriteListTags handles GET /api/admin/vibe-write/tags
// Returns all tags (including inactive) with their accounts.
func AdminVibeWriteListTags(c *gin.Context) {
	tags, err := services.AdminListTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// AdminVibeWriteCreateTag handles POST /api/admin/vibe-write/tags
// Creates a new tag.
func AdminVibeWriteCreateTag(c *gin.Context) {
	var req struct {
		ID          string `json:"id" binding:"required"`
		Name        string `json:"name" binding:"required"`
		NameEN      string `json:"name_en"`
		Icon        string `json:"icon"`
		Category    string `json:"category"`
		Description string `json:"description"`
		IsDefault   bool   `json:"is_default"`
		SortOrder   int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and name are required"})
		return
	}

	tag := &models.VibeWriteTag{
		ID:          req.ID,
		Name:        req.Name,
		NameEN:      req.NameEN,
		Icon:        req.Icon,
		Category:    req.Category,
		Description: req.Description,
		IsDefault:   req.IsDefault,
		Active:      true,
		SortOrder:   req.SortOrder,
	}

	if err := services.AdminCreateTag(tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tag": tag})
}

// AdminVibeWriteUpdateTag handles PUT /api/admin/vibe-write/tags/:id
// Updates an existing tag.
func AdminVibeWriteUpdateTag(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag id is required"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Don't allow changing the ID
	delete(req, "id")

	if err := services.AdminUpdateTag(tagID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "tag_id": tagID})
}

// AdminVibeWriteDeleteTag handles DELETE /api/admin/vibe-write/tags/:id
// Soft-deletes a tag (sets inactive, removes accounts).
func AdminVibeWriteDeleteTag(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag id is required"})
		return
	}

	if err := services.AdminDeleteTag(tagID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted", "tag_id": tagID})
}

// ══════════════════════════════════════════════════════════════════════════════
// Admin: Vibe Write Tag Account Management
// ══════════════════════════════════════════════════════════════════════════════

// AdminVibeWriteListTagAccounts handles GET /api/admin/vibe-write/tags/:id/accounts
// Returns all accounts for a tag.
func AdminVibeWriteListTagAccounts(c *gin.Context) {
	tagID := c.Param("id")
	accounts, err := services.AdminListTagAccounts(tagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts, "tag_id": tagID})
}

// AdminVibeWriteAddTagAccount handles POST /api/admin/vibe-write/tags/:id/accounts
// Adds an account to a tag.
func AdminVibeWriteAddTagAccount(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag id is required"})
		return
	}

	var req struct {
		Handle           string `json:"handle" binding:"required"`
		DisplayName      string `json:"display_name"`
		RealtimePriority bool   `json:"realtime_priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	// Strip @ prefix
	handle := req.Handle
	if len(handle) > 0 && handle[0] == '@' {
		handle = handle[1:]
	}

	if err := services.AdminAddAccountToTag(tagID, handle, req.DisplayName, req.RealtimePriority); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "added", "tag_id": tagID, "handle": handle})
}

// AdminVibeWriteRemoveTagAccount handles DELETE /api/admin/vibe-write/tags/:id/accounts/:handle
// Removes an account from a tag.
func AdminVibeWriteRemoveTagAccount(c *gin.Context) {
	tagID := c.Param("id")
	handle := c.Param("handle")
	if tagID == "" || handle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag id and handle are required"})
		return
	}

	if err := services.AdminRemoveAccountFromTag(tagID, handle); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "removed", "tag_id": tagID, "handle": handle})
}

// ══════════════════════════════════════════════════════════════════════════════
// Admin: Vibe Write Tag Candidate Management
// ══════════════════════════════════════════════════════════════════════════════

// AdminVibeWriteImportCandidates handles POST /api/admin/vibe-write/candidates/import
// Imports handles for AI classification and admin review.
func AdminVibeWriteImportCandidates(c *gin.Context) {
	var req struct {
		Handles      []string `json:"handles"`       // list of handles
		ListURL      string   `json:"list_url"`      // Twitter list URL (future)
		SourceDetail string   `json:"source_detail"` // description
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handles array or list_url is required"})
		return
	}

	source := "batch_input"
	if req.ListURL != "" {
		source = "list_import"
		// TODO: fetch handles from Twitter List URL
		c.JSON(http.StatusNotImplemented, gin.H{"error": "list import not yet implemented"})
		return
	}

	if len(req.Handles) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one handle is required"})
		return
	}

	imported, err := services.AdminImportCandidates(req.Handles, source, req.SourceDetail)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"imported": imported,
		"total":    len(req.Handles),
	})
}

// AdminVibeWriteListCandidates handles GET /api/admin/vibe-write/candidates
// Returns candidates with optional status filter.
func AdminVibeWriteListCandidates(c *gin.Context) {
	status := c.Query("status")

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil {
			offset = n
		}
	}

	candidates, total, err := services.AdminListVibeWriteCandidates(status, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"candidates": candidates,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// AdminVibeWriteApproveCandidate handles POST /api/admin/vibe-write/candidates/:id/approve
// Approves a candidate and creates tag-account associations.
func AdminVibeWriteApproveCandidate(c *gin.Context) {
	candidateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid candidate ID"})
		return
	}

	var req struct {
		TagIDs           []string `json:"tag_ids" binding:"required"`
		RealtimePriority bool     `json:"realtime_priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag_ids array is required"})
		return
	}

	adminWallet := "" // Could extract from admin session if needed

	if err := services.AdminApproveCandidate(candidateID, req.TagIDs, req.RealtimePriority, adminWallet); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}

// AdminVibeWriteRejectCandidate handles POST /api/admin/vibe-write/candidates/:id/reject
// Rejects a candidate.
func AdminVibeWriteRejectCandidate(c *gin.Context) {
	candidateID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid candidate ID"})
		return
	}

	adminWallet := ""

	if err := services.AdminRejectCandidate(candidateID, adminWallet); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

// AdminVibeWriteBatchReview handles POST /api/admin/vibe-write/candidates/batch
// Batch approve/reject candidates.
func AdminVibeWriteBatchReview(c *gin.Context) {
	var req struct {
		Action           string   `json:"action" binding:"required"` // "approve" or "reject"
		CandidateIDs     []string `json:"candidate_ids" binding:"required"`
		TagIDs           []string `json:"tag_ids"`           // only for approve
		RealtimePriority bool     `json:"realtime_priority"` // only for approve
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action and candidate_ids are required"})
		return
	}

	if req.Action != "approve" && req.Action != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action must be 'approve' or 'reject'"})
		return
	}

	adminWallet := ""
	success := 0
	failed := 0

	for _, idStr := range req.CandidateIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			failed++
			continue
		}

		var opErr error
		if req.Action == "approve" {
			opErr = services.AdminApproveCandidate(id, req.TagIDs, req.RealtimePriority, adminWallet)
		} else {
			opErr = services.AdminRejectCandidate(id, adminWallet)
		}

		if opErr != nil {
			failed++
		} else {
			success++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"action":  req.Action,
		"success": success,
		"failed":  failed,
	})
}
