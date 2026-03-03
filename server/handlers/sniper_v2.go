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
// Sniper 2.0 Handlers — Tag-based Feed + Snipe
// ══════════════════════════════════════════════════════════════════════════════

// ──────────────────────────────────────────────────────────────────────────────
// Tags
// ──────────────────────────────────────────────────────────────────────────────

// SniperGetTags handles GET /api/sniper/tags
// Returns all active tags with their accounts. Public endpoint.
func SniperGetTags(c *gin.Context) {
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

// SniperGetFeed handles GET /api/sniper/feed
// Returns aggregated tweets from selected tags. Public endpoint.
func SniperGetFeed(c *gin.Context) {
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

// SniperFeedRefresh handles GET /api/sniper/feed/refresh
// Forces a cache refresh for the specified tags. Public endpoint.
func SniperFeedRefresh(c *gin.Context) {
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

// SniperFeedStream handles GET /api/sniper/feed/stream
// SSE endpoint for real-time tweet push. Public endpoint.
func SniperFeedStream(c *gin.Context) {
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

// SniperGetUserTags handles GET /api/sniper/user/tags
// Returns the user's selected tags. Requires session.
func SniperGetUserTags(c *gin.Context) {
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

// SniperUpdateUserTags handles PUT /api/sniper/user/tags
// Updates the user's selected tags. Requires session.
func SniperUpdateUserTags(c *gin.Context) {
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

// SniperGetMuted handles GET /api/sniper/user/muted
// Returns the user's muted accounts. Requires session.
func SniperGetMuted(c *gin.Context) {
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

// SniperMuteAccount handles POST /api/sniper/user/muted
// Mutes an account for the user. Requires session.
func SniperMuteAccount(c *gin.Context) {
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

// SniperUnmuteAccount handles DELETE /api/sniper/user/muted/:handle
// Unmutes an account. Requires session.
func SniperUnmuteAccount(c *gin.Context) {
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

// SniperSnipe handles POST /api/sniper/snipe
// Generates reply suggestions for a tweet. Requires Pro subscription.
func SniperSnipe(c *gin.Context) {
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
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tweet_id, tweet_text, and author_handle are required"})
		return
	}

	reply, err := services.Snipe(walletAddr, req.AuthorHandle, req.TweetID, req.TweetText, req.TagID)
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
// Admin: Sniper Tag CRUD
// ══════════════════════════════════════════════════════════════════════════════

// AdminSniperListTags handles GET /api/admin/sniper/tags
// Returns all tags (including inactive) with their accounts.
func AdminSniperListTags(c *gin.Context) {
	tags, err := services.AdminListTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// AdminSniperCreateTag handles POST /api/admin/sniper/tags
// Creates a new tag.
func AdminSniperCreateTag(c *gin.Context) {
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

	tag := &models.SniperTag{
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

// AdminSniperUpdateTag handles PUT /api/admin/sniper/tags/:id
// Updates an existing tag.
func AdminSniperUpdateTag(c *gin.Context) {
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

// AdminSniperDeleteTag handles DELETE /api/admin/sniper/tags/:id
// Soft-deletes a tag (sets inactive, removes accounts).
func AdminSniperDeleteTag(c *gin.Context) {
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
// Admin: Sniper Tag Account Management
// ══════════════════════════════════════════════════════════════════════════════

// AdminSniperListTagAccounts handles GET /api/admin/sniper/tags/:id/accounts
// Returns all accounts for a tag.
func AdminSniperListTagAccounts(c *gin.Context) {
	tagID := c.Param("id")
	accounts, err := services.AdminListTagAccounts(tagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts, "tag_id": tagID})
}

// AdminSniperAddTagAccount handles POST /api/admin/sniper/tags/:id/accounts
// Adds an account to a tag.
func AdminSniperAddTagAccount(c *gin.Context) {
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

// AdminSniperRemoveTagAccount handles DELETE /api/admin/sniper/tags/:id/accounts/:handle
// Removes an account from a tag.
func AdminSniperRemoveTagAccount(c *gin.Context) {
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
// Admin: Sniper Tag Candidate Management
// ══════════════════════════════════════════════════════════════════════════════

// AdminSniperImportCandidates handles POST /api/admin/sniper/candidates/import
// Imports handles for AI classification and admin review.
func AdminSniperImportCandidates(c *gin.Context) {
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

// AdminSniperListCandidates handles GET /api/admin/sniper/candidates
// Returns candidates with optional status filter.
func AdminSniperListCandidates(c *gin.Context) {
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

	candidates, total, err := services.AdminListSniperCandidates(status, limit, offset)
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

// AdminSniperApproveCandidate handles POST /api/admin/sniper/candidates/:id/approve
// Approves a candidate and creates tag-account associations.
func AdminSniperApproveCandidate(c *gin.Context) {
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

// AdminSniperRejectCandidate handles POST /api/admin/sniper/candidates/:id/reject
// Rejects a candidate.
func AdminSniperRejectCandidate(c *gin.Context) {
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

// AdminSniperBatchReview handles POST /api/admin/sniper/candidates/batch
// Batch approve/reject candidates.
func AdminSniperBatchReview(c *gin.Context) {
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
