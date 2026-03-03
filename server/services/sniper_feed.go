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

// RefreshTagFeed forces a cache refresh for specific tags and broadcasts new tweets.
func RefreshTagFeed(tagIDs []string) (int, error) {
	handleToTags, err := GetHandleToTagsMap(tagIDs)
	if err != nil {
		return 0, err
	}

	var (
		wg       sync.WaitGroup
		totalNew int64 // use atomic-safe counter via mutex
		countMu  sync.Mutex
	)
	for _, tagID := range tagIDs {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()
			tweets, err := fetchTagFeed(tid, handleToTags)
			if err != nil {
				util.Log.Warn("[feed] Refresh failed for tag %s: %v", tid, err)
				return
			}
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
