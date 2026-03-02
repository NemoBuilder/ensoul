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
// Feed Service — Sniper 2.0 tag-based tweet feed with caching
// ──────────────────────────────────────────────────────────────────────────────

const (
	feedCacheTTL       = 10 * time.Minute // cache TTL per tag
	feedRefreshMinWait = 2 * time.Minute  // minimum time between refreshes per tag
	feedDefaultCount   = 20
	feedMaxCount       = 50
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
	feedCache   = make(map[string]*feedCacheEntry) // key: tag_id
	feedCacheMu sync.RWMutex
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

// ──────────────────────────────────────────────────────────────────────────────
// Feed Building
// ──────────────────────────────────────────────────────────────────────────────

// FeedResult is the response for GET /api/sniper/feed.
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

	for _, tagID := range tagIDs {
		// Try cache first
		cached, fetchedAt, ok := getCachedFeed(tagID)
		if ok {
			allTweets = append(allTweets, cached...)
			age := time.Since(fetchedAt)
			if age > cacheAge {
				cacheAge = age
			}
		} else {
			// Cache miss: fetch from SocialData
			allCached = false
			tweets, err := fetchTagFeed(tagID, handleToTags)
			if err != nil {
				util.Log.Warn("[feed] Failed to fetch feed for tag %s: %v", tagID, err)
				continue
			}
			allTweets = append(allTweets, tweets...)

			// Store in cache + broadcast new tweets via SSE
			newTweets := setCachedFeed(tagID, tweets)
			if len(newTweets) > 0 && sseHubInstance != nil {
				sseHubInstance.Broadcast(tagID, newTweets)
			}
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

// RefreshTagFeed forces a cache refresh for specific tags and broadcasts new tweets.
func RefreshTagFeed(tagIDs []string) (int, error) {
	handleToTags, err := GetHandleToTagsMap(tagIDs)
	if err != nil {
		return 0, err
	}

	totalNew := 0
	for _, tagID := range tagIDs {
		tweets, err := fetchTagFeed(tagID, handleToTags)
		if err != nil {
			util.Log.Warn("[feed] Refresh failed for tag %s: %v", tagID, err)
			continue
		}

		newTweets := setCachedFeed(tagID, tweets)
		if len(newTweets) > 0 && sseHubInstance != nil {
			sseHubInstance.Broadcast(tagID, newTweets)
		}
		totalNew += len(newTweets)
	}

	return totalNew, nil
}

// fetchTagFeed fetches tweets for all accounts under a tag from SocialData.
func fetchTagFeed(tagID string, handleToTags map[string][]string) ([]TweetCard, error) {
	if !SocialDataAvailable() {
		return nil, fmt.Errorf("SocialData API not configured")
	}

	// Get accounts for this tag
	var accounts []models.SniperTagAccount
	database.DB.Where("tag_id = ?", tagID).Find(&accounts)

	if len(accounts) == 0 {
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
	sdTweets, err := client.SearchTweets(query, feedMaxCount)
	if err != nil {
		return nil, fmt.Errorf("search failed for tag %s: %w", tagID, err)
	}

	// Convert to TweetCards
	cards := make([]TweetCard, 0, len(sdTweets))
	for _, t := range sdTweets {
		card := sdTweetToCard(t, handleToTags)
		cards = append(cards, card)
	}

	util.Log.Debug("[feed] Fetched %d tweets for tag %s (%d accounts)", len(cards), tagID, len(accounts))
	return cards, nil
}

// sdTweetToCard converts a SocialData tweet to a TweetCard.
func sdTweetToCard(t sdTweet, handleToTags map[string][]string) TweetCard {
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

	// Check if author has a Soul
	hasSoul := false
	var soulHandle *string
	if t.User != nil {
		var shell models.Shell
		if err := database.DB.Where("LOWER(handle) = ?", strings.ToLower(t.User.ScreenName)).
			First(&shell).Error; err == nil {
			hasSoul = true
			h := shell.Handle
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

// StartFeedRefresher starts a background loop that periodically refreshes
// all active tag feeds and broadcasts new tweets via SSE.
func StartFeedRefresher(interval time.Duration) {
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
	var tags []models.SniperTag
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
