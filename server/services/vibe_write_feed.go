package services

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// ──────────────────────────────────────────────────────────────────────────────
// Feed Service — Vibe Write 2.0 tag-based tweet feed with caching
// ──────────────────────────────────────────────────────────────────────────────

const (
	feedCacheTTL       = 15 * time.Minute // cache TTL per tag
	feedRefreshMinWait = 10 * time.Minute // minimum time between refreshes per tag
	feedDefaultCount   = 20
	feedMaxCount       = 20   // reduced from 50 to save SocialData credits
	feedDailyBudget    = 2000 // max SocialData API calls per day (safety)
)

// TweetCard is the unified tweet representation for the feed.
type TweetCard struct {
	ID         string          `json:"id"`
	Text       string          `json:"text"`
	Author     TweetCardAuthor `json:"author"`
	Tags       []string        `json:"tags"`
	CreatedAt  string          `json:"created_at"`
	ParsedTime time.Time       `json:"-"` // for sorting, not serialized
	Stats      TweetCardStats  `json:"stats"`
	HasMedia   bool            `json:"has_media"`
	TweetURL   string          `json:"tweet_url"`
	HasSoul    bool            `json:"has_soul"`
	SoulHandle *string         `json:"soul_handle"`
}

// TweetCardAuthor is the author info on a tweet card.
type TweetCardAuthor struct {
	Handle         string `json:"handle"`
	Name           string `json:"name"`
	Avatar         string `json:"avatar"`
	Verified       bool   `json:"verified"`
	FollowersCount int    `json:"followers_count"`
}

// TweetCardStats holds tweet engagement metrics.
type TweetCardStats struct {
	Replies  int `json:"replies"`
	Retweets int `json:"retweets"`
	Likes    int `json:"likes"`
	Views    int `json:"views"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Feed Cache (in-memory, per-tag)
// ──────────────────────────────────────────────────────────────────────────────

type feedCacheEntry struct {
	tweets    []TweetCard
	fetchedAt time.Time
}

var (
	feedCache       = make(map[string]*feedCacheEntry) // key: tag_id
	feedCacheMu     sync.RWMutex
	lastRefreshed   = make(map[string]time.Time) // key: tag_id → last SocialData call time
	lastRefreshedMu sync.RWMutex
	dailyAPICount   int64  // daily SocialData API call counter
	dailyAPIDate    string // date string (YYYY-MM-DD) for daily reset
	dailyAPIMu      sync.Mutex
)

// getCachedFeed returns cached tweets for a tag, or nil if expired/missing.
func getCachedFeed(tagID string) ([]TweetCard, time.Time, bool) {
	feedCacheMu.RLock()
	defer feedCacheMu.RUnlock()

	entry, ok := feedCache[tagID]
	if !ok {
		return nil, time.Time{}, false
	}

	if time.Since(entry.fetchedAt) > feedCacheTTL {
		return nil, time.Time{}, false
	}

	return entry.tweets, entry.fetchedAt, true
}

// setCachedFeed stores tweets in the cache for a tag.
// Returns newly added tweets (for SSE broadcasting).
func setCachedFeed(tagID string, tweets []TweetCard) []TweetCard {
	feedCacheMu.Lock()
	defer feedCacheMu.Unlock()

	oldEntry := feedCache[tagID]
	var newTweets []TweetCard

	if oldEntry != nil {
		// Find new tweets that weren't in the old cache
		oldIDs := make(map[string]bool)
		for _, t := range oldEntry.tweets {
			oldIDs[t.ID] = true
		}
		for _, t := range tweets {
			if !oldIDs[t.ID] {
				newTweets = append(newTweets, t)
			}
		}
	} else {
		// All tweets are new on first cache fill
		newTweets = tweets
	}

	feedCache[tagID] = &feedCacheEntry{
		tweets:    tweets,
		fetchedAt: time.Now(),
	}

	return newTweets
}

// canRefreshTag returns true if enough time has passed since last refresh for this tag.
func canRefreshTag(tagID string) bool {
	lastRefreshedMu.RLock()
	defer lastRefreshedMu.RUnlock()
	t, ok := lastRefreshed[tagID]
	if !ok {
		return true
	}
	return time.Since(t) >= feedRefreshMinWait
}

// markTagRefreshed records the current time as last refresh for a tag.
func markTagRefreshed(tagID string) {
	lastRefreshedMu.Lock()
	defer lastRefreshedMu.Unlock()
	lastRefreshed[tagID] = time.Now()
}

// trackAPICall increments the daily API call counter. Returns false if budget exceeded.
func trackAPICall(count int) bool {
	dailyAPIMu.Lock()
	defer dailyAPIMu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	if dailyAPIDate != today {
		// New day — reset counter
		dailyAPICount = 0
		dailyAPIDate = today
		util.Log.Info("[feed] Daily API budget reset for %s (limit: %d)", today, feedDailyBudget)
	}

	if dailyAPICount >= int64(feedDailyBudget) {
		return false // budget exceeded
	}

	dailyAPICount += int64(count)
	return true
}

// GetDailyAPIUsage returns the current daily API call count.
func GetDailyAPIUsage() (int64, int) {
	dailyAPIMu.Lock()
	defer dailyAPIMu.Unlock()
	return dailyAPICount, feedDailyBudget
}

// ──────────────────────────────────────────────────────────────────────────────
// Feed Building
// ──────────────────────────────────────────────────────────────────────────────

// FeedResult is the response for GET /api/vibe-write/feed.
type FeedResult struct {
	TagIDs          []string    `json:"tag_ids"`
	Tweets          []TweetCard `json:"tweets"`
	NextCursor      string      `json:"next_cursor"`
	Cached          bool        `json:"cached"`
	CacheAgeSeconds int         `json:"cache_age_seconds"`
}

// BuildFeed aggregates tweets from multiple tags, with caching and mute filtering.
func BuildFeed(tagIDs []string, mutedHandles []string, cursor string, count int) (*FeedResult, error) {
	if count <= 0 {
		count = feedDefaultCount
	}
	if count > feedMaxCount {
		count = feedMaxCount
	}

	mutedSet := make(map[string]bool)
	for _, h := range mutedHandles {
		mutedSet[strings.ToLower(h)] = true
	}

	// Get handle→tags mapping for tag annotation
	handleToTags, err := GetHandleToTagsMap(tagIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get handle-tag mapping: %w", err)
	}

	allCached := true
	var cacheAge time.Duration
	var allTweets []TweetCard

	// Separate cached vs uncached tags
	var uncachedTagIDs []string
	for _, tagID := range tagIDs {
		cached, fetchedAt, ok := getCachedFeed(tagID)
		if ok {
			allTweets = append(allTweets, cached...)
			age := time.Since(fetchedAt)
			if age > cacheAge {
				cacheAge = age
			}
		} else {
			allCached = false
			uncachedTagIDs = append(uncachedTagIDs, tagID)
		}
	}

	// Fetch uncached tags concurrently
	if len(uncachedTagIDs) > 0 {
		type tagResult struct {
			tagID  string
			tweets []TweetCard
		}
		var (
			wg      sync.WaitGroup
			results []tagResult
			resMu   sync.Mutex
		)
		for _, tagID := range uncachedTagIDs {
			wg.Add(1)
			go func(tid string) {
				defer wg.Done()
				tweets, err := fetchTagFeed(tid, handleToTags)
				if err != nil {
					util.Log.Warn("[feed] Failed to fetch feed for tag %s: %v", tid, err)
					return
				}
				markTagRefreshed(tid)
				// Store in cache + broadcast
				newTweets := setCachedFeed(tid, tweets)
				if len(newTweets) > 0 && sseHubInstance != nil {
					sseHubInstance.Broadcast(tid, newTweets)
				}
				resMu.Lock()
				results = append(results, tagResult{tagID: tid, tweets: tweets})
				resMu.Unlock()
			}(tagID)
		}
		wg.Wait()

		for _, r := range results {
			allTweets = append(allTweets, r.tweets...)
		}
	}

	// Deduplicate by tweet ID
	seen := make(map[string]bool)
	var deduped []TweetCard
	for _, t := range allTweets {
		if seen[t.ID] {
			// Merge tags for duplicate
			for i := range deduped {
				if deduped[i].ID == t.ID {
					deduped[i].Tags = mergeTags(deduped[i].Tags, t.Tags)
					break
				}
			}
			continue
		}
		seen[t.ID] = true
		deduped = append(deduped, t)
	}

	// Filter muted accounts
	var filtered []TweetCard
	for _, t := range deduped {
		if !mutedSet[strings.ToLower(t.Author.Handle)] {
			filtered = append(filtered, t)
		}
	}

	// Sort by time descending
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ParsedTime.After(filtered[j].ParsedTime)
	})

	// Apply cursor-based pagination (cursor = last tweet ID)
	if cursor != "" {
		startIdx := -1
		for i, t := range filtered {
			if t.ID == cursor {
				startIdx = i + 1
				break
			}
		}
		if startIdx >= 0 && startIdx < len(filtered) {
			filtered = filtered[startIdx:]
		} else if startIdx >= len(filtered) {
			filtered = nil
		}
	}

	// Limit
	nextCursor := ""
	if len(filtered) > count {
		nextCursor = filtered[count-1].ID
		filtered = filtered[:count]
	}

	return &FeedResult{
		TagIDs:          tagIDs,
		Tweets:          filtered,
		NextCursor:      nextCursor,
		Cached:          allCached,
		CacheAgeSeconds: int(cacheAge.Seconds()),
	}, nil
}

// RefreshTagFeed refreshes feeds for specific tags, respecting per-tag cooldown and daily budget.
func RefreshTagFeed(tagIDs []string) (int, error) {
	// Filter out tags that were refreshed too recently
	var eligibleTags []string
	for _, tid := range tagIDs {
		if canRefreshTag(tid) {
			eligibleTags = append(eligibleTags, tid)
		}
	}

	if len(eligibleTags) == 0 {
		return 0, nil // all tags still within cooldown
	}

	// Check daily budget
	if !trackAPICall(0) {
		util.Log.Warn("[feed] Daily SocialData API budget (%d) exceeded, skipping refresh", feedDailyBudget)
		return 0, nil
	}

	handleToTags, err := GetHandleToTagsMap(eligibleTags)
	if err != nil {
		return 0, err
	}

	var (
		wg       sync.WaitGroup
		totalNew int64
		countMu  sync.Mutex
	)
	for _, tagID := range eligibleTags {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			tweets, err := fetchTagFeed(tid, handleToTags)
			if err != nil {
				util.Log.Warn("[feed] Refresh failed for tag %s: %v", tid, err)
				return
			}
			markTagRefreshed(tid)
			newTweets := setCachedFeed(tid, tweets)
			if len(newTweets) > 0 && sseHubInstance != nil {
				sseHubInstance.Broadcast(tid, newTweets)
			}
			countMu.Lock()
			totalNew += int64(len(newTweets))
			countMu.Unlock()
		}(tagID)
	}
	wg.Wait()

	return int(totalNew), nil
}

// fetchTagFeed fetches tweets for all accounts under a tag from SocialData.
func fetchTagFeed(tagID string, handleToTags map[string][]string) ([]TweetCard, error) {
	if !SocialDataAvailable() {
		return nil, fmt.Errorf("SocialData API not configured")
	}

	// Get accounts for this tag
	var accounts []models.VibeWriteTagAccount
	database.DB.Where("tag_id = ?", tagID).Find(&accounts)

	if len(accounts) == 0 {
		return nil, nil
	}

	// Check daily budget before making API call
	if !trackAPICall(1) {
		util.Log.Warn("[feed] Daily API budget exceeded, skipping fetch for tag %s", tagID)
		return nil, nil
	}

	// Build search query: "from:handle1 OR from:handle2 OR ..."
	// SocialData search API supports Twitter search syntax
	var parts []string
	for _, a := range accounts {
		parts = append(parts, "from:"+a.Handle)
	}
	query := strings.Join(parts, " OR ")

	client := newSocialDataClient()
	// Only fetch 1 page (20 tweets) — enough for a live feed, saves credits
	sdTweets, err := client.SearchTweets(query, socialDataTweetsPerPage)
	if err != nil {
		return nil, fmt.Errorf("search failed for tag %s: %w", tagID, err)
	}

	// Batch lookup: collect all unique handles and query Soul table once
	handleSet := make(map[string]bool)
	for _, t := range sdTweets {
		if t.User != nil {
			handleSet[strings.ToLower(t.User.ScreenName)] = true
		}
	}
	soulMap := batchLookupSouls(handleSet)

	// Convert to TweetCards
	cards := make([]TweetCard, 0, len(sdTweets))
	for _, t := range sdTweets {
		card := sdTweetToCard(t, handleToTags, soulMap)
		cards = append(cards, card)
	}

	util.Log.Debug("[feed] Fetched %d tweets for tag %s (%d accounts)", len(cards), tagID, len(accounts))
	return cards, nil
}

// batchLookupSouls queries the shells table once for all handles and returns
// a map[lowercase_handle] → shell.Handle for those that exist.
func batchLookupSouls(handleSet map[string]bool) map[string]string {
	result := make(map[string]string)
	if len(handleSet) == 0 {
		return result
	}

	handles := make([]string, 0, len(handleSet))
	for h := range handleSet {
		handles = append(handles, h)
	}

	var shells []models.Shell
	database.DB.Where("LOWER(handle) IN ?", handles).Find(&shells)
	for _, s := range shells {
		result[strings.ToLower(s.Handle)] = s.Handle
	}
	return result
}

// sdTweetToCard converts a SocialData tweet to a TweetCard.
// soulMap is a pre-fetched map[lowercase_handle] → handle for batch Soul lookup.
func sdTweetToCard(t sdTweet, handleToTags map[string][]string, soulMap map[string]string) TweetCard {
	text := t.FullText
	if text == "" && t.Text != nil {
		text = *t.Text
	}

	var author TweetCardAuthor
	if t.User != nil {
		author = TweetCardAuthor{
			Handle: t.User.ScreenName,
			Name:   t.User.Name,
			Avatar: fmt.Sprintf("https://unavatar.io/twitter/%s", t.User.ScreenName),
		}
	}

	tweetURL := ""
	if t.User != nil {
		tweetURL = fmt.Sprintf("https://twitter.com/%s/status/%s", t.User.ScreenName, t.IDStr)
	}

	// Parse time
	parsedTime, _ := time.Parse("Mon Jan 02 15:04:05 +0000 2006", t.TweetCreatedAt)

	// Determine tags from handle
	var tags []string
	if t.User != nil {
		tags = handleToTags[t.User.ScreenName]
		if tags == nil {
			tags = handleToTags[strings.ToLower(t.User.ScreenName)]
		}
	}

	// Check if author has a Soul (from pre-fetched batch)
	hasSoul := false
	var soulHandle *string
	if t.User != nil {
		if h, ok := soulMap[strings.ToLower(t.User.ScreenName)]; ok {
			hasSoul = true
			soulHandle = &h
		}
	}

	return TweetCard{
		ID:         t.IDStr,
		Text:       text,
		Author:     author,
		Tags:       tags,
		CreatedAt:  t.TweetCreatedAt,
		ParsedTime: parsedTime,
		Stats: TweetCardStats{
			Replies:  t.ReplyCount,
			Retweets: t.RetweetCount,
			Likes:    t.FavoriteCount,
			Views:    t.ViewsCount,
		},
		HasMedia:   false, // TODO: detect media from tweet
		TweetURL:   tweetURL,
		HasSoul:    hasSoul,
		SoulHandle: soulHandle,
	}
}

// mergeTags merges two tag slices, deduplicating.
func mergeTags(a, b []string) []string {
	set := make(map[string]bool)
	for _, t := range a {
		set[t] = true
	}
	for _, t := range b {
		set[t] = true
	}
	result := make([]string, 0, len(set))
	for t := range set {
		result = append(result, t)
	}
	return result
}

// ──────────────────────────────────────────────────────────────────────────────
// Background Feed Refresher
// ──────────────────────────────────────────────────────────────────────────────

// WarmupFeedCache pre-fills the feed cache for all active tags on startup.
// This runs synchronously to ensure cache is warm before first user request.
func WarmupFeedCache() {
	if !SocialDataAvailable() {
		util.Log.Warn("[feed] SocialData not configured, skipping cache warmup")
		return
	}

	util.Log.Info("[feed] Warming up feed cache...")
	start := time.Now()
	refreshAllTagFeeds()
	util.Log.Info("[feed] Cache warmup complete in %s", time.Since(start).Round(time.Millisecond))
}

// StartFeedRefresher starts a background loop that periodically refreshes
// all active tag feeds and broadcasts new tweets via SSE.
func StartFeedRefresher(interval time.Duration) {
	// Warm up cache immediately in background
	go WarmupFeedCache()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			refreshAllTagFeeds()
		}
	}()
	util.Log.Info("[feed] Background refresher started (interval: %s)", interval)
}

// refreshAllTagFeeds refreshes feeds for all active tags.
func refreshAllTagFeeds() {
	var tags []models.VibeWriteTag
	database.DB.Where("active = ?", true).Find(&tags)

	if len(tags) == 0 {
		return
	}

	tagIDs := make([]string, 0, len(tags))
	for _, t := range tags {
		tagIDs = append(tagIDs, t.ID)
	}

	newCount, err := RefreshTagFeed(tagIDs)
	if err != nil {
		util.Log.Warn("[feed] Background refresh error: %v", err)
	}
	if newCount > 0 {
		util.Log.Info("[feed] Background refresh: %d new tweets across %d tags", newCount, len(tagIDs))
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// External Tweet Push (Requirement ①)
// ──────────────────────────────────────────────────────────────────────────────

// ExternalTweetInput represents a tweet pushed from an external source.
type ExternalTweetInput struct {
	ID           string          `json:"id" binding:"required"`
	Text         string          `json:"text" binding:"required"`
	AuthorHandle string          `json:"author_handle" binding:"required"`
	AuthorName   string          `json:"author_name"`
	AuthorAvatar string          `json:"author_avatar"`
	CreatedAt    string          `json:"created_at"`
	TagIDs       []string        `json:"tag_ids" binding:"required,min=1"`
	Stats        *TweetCardStats `json:"stats"`
}

// InjectExternalTweets injects externally pushed tweets into the feed cache
// and broadcasts them via SSE. Returns the number of new tweets injected.
func InjectExternalTweets(tweets []ExternalTweetInput, source string) (int, error) {
	if len(tweets) == 0 {
		return 0, nil
	}

	// Group tweets by tag
	tagTweets := make(map[string][]TweetCard)

	for _, t := range tweets {
		// Build TweetCard from external input
		createdAt := t.CreatedAt
		if createdAt == "" {
			createdAt = time.Now().UTC().Format("Mon Jan 02 15:04:05 +0000 2006")
		}
		parsedTime, _ := time.Parse("Mon Jan 02 15:04:05 +0000 2006", createdAt)
		if parsedTime.IsZero() {
			// Try ISO format
			parsedTime, _ = time.Parse(time.RFC3339, createdAt)
		}
		if parsedTime.IsZero() {
			parsedTime = time.Now().UTC()
		}

		avatar := t.AuthorAvatar
		if avatar == "" {
			avatar = fmt.Sprintf("https://unavatar.io/twitter/%s", t.AuthorHandle)
		}

		tweetURL := fmt.Sprintf("https://twitter.com/%s/status/%s", t.AuthorHandle, t.ID)

		// Check if author has a Soul
		hasSoul := false
		var soulHandle *string
		var shell models.Shell
		if err := database.DB.Where("LOWER(handle) = ?", strings.ToLower(t.AuthorHandle)).First(&shell).Error; err == nil {
			hasSoul = true
			soulHandle = &shell.Handle
		}

		stats := TweetCardStats{}
		if t.Stats != nil {
			stats = *t.Stats
		}

		card := TweetCard{
			ID:   t.ID,
			Text: t.Text,
			Author: TweetCardAuthor{
				Handle: t.AuthorHandle,
				Name:   t.AuthorName,
				Avatar: avatar,
			},
			Tags:       t.TagIDs,
			CreatedAt:  createdAt,
			ParsedTime: parsedTime,
			Stats:      stats,
			TweetURL:   tweetURL,
			HasSoul:    hasSoul,
			SoulHandle: soulHandle,
		}

		for _, tagID := range t.TagIDs {
			tagTweets[tagID] = append(tagTweets[tagID], card)
		}
	}

	// Inject into each tag's cache and broadcast via SSE
	totalNew := 0
	for tagID, cards := range tagTweets {
		newCards := injectIntoTagCache(tagID, cards)
		totalNew += len(newCards)
		if len(newCards) > 0 && sseHubInstance != nil {
			sseHubInstance.Broadcast(tagID, newCards)
		}
	}

	util.Log.Info("[feed] Injected %d external tweets across %d tags (source: %s)", totalNew, len(tagTweets), source)
	return totalNew, nil
}

// injectIntoTagCache merges new tweets into an existing tag cache entry.
// Returns only the truly new tweets (not already in cache).
func injectIntoTagCache(tagID string, tweets []TweetCard) []TweetCard {
	feedCacheMu.Lock()
	defer feedCacheMu.Unlock()

	entry := feedCache[tagID]

	// Build set of existing IDs
	existingIDs := make(map[string]bool)
	if entry != nil {
		for _, t := range entry.tweets {
			existingIDs[t.ID] = true
		}
	}

	// Filter to truly new tweets
	var newTweets []TweetCard
	for _, t := range tweets {
		if !existingIDs[t.ID] {
			newTweets = append(newTweets, t)
		}
	}

	if len(newTweets) == 0 {
		return nil
	}

	if entry == nil {
		// Create new cache entry
		feedCache[tagID] = &feedCacheEntry{
			tweets:    newTweets,
			fetchedAt: time.Now(),
		}
	} else {
		// Prepend new tweets to existing
		entry.tweets = append(newTweets, entry.tweets...)
		entry.fetchedAt = time.Now()
	}

	return newTweets
}
