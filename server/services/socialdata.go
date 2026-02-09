package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/util"
)

// ──────────────────────────────────────────────────────────────────────────────
// SocialData API client — https://docs.socialdata.tools
// Used as primary data source for Twitter profile + tweets during Seed stage.
// Falls back to Twitter v2 API or mock if SocialData is not configured.
// ──────────────────────────────────────────────────────────────────────────────

const (
	socialDataDefaultBaseURL = "https://api.socialdata.tools"
	socialDataTweetsPerPage  = 20 // SocialData returns ~20 tweets per page
	socialDataMaxTweets      = 50 // We want at most 50 tweets for seed extraction
)

// socialDataClient is a thin HTTP wrapper for SocialData API calls.
type socialDataClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func newSocialDataClient() *socialDataClient {
	base := config.Cfg.SocialDataBaseURL
	if base == "" {
		base = socialDataDefaultBaseURL
	}
	// Trim trailing slash
	base = strings.TrimRight(base, "/")

	return &socialDataClient{
		baseURL: base,
		apiKey:  config.Cfg.SocialDataAPIKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Response types — match SocialData JSON schema
// ──────────────────────────────────────────────────────────────────────────────

// sdUserProfile is the raw response from GET /twitter/user/{username}.
type sdUserProfile struct {
	ID                   int64  `json:"id"`
	IDStr                string `json:"id_str"`
	Name                 string `json:"name"`
	ScreenName           string `json:"screen_name"`
	Description          string `json:"description"`
	Location             string `json:"location"`
	Protected            bool   `json:"protected"`
	Verified             bool   `json:"verified"`
	FollowersCount       int    `json:"followers_count"`
	FriendsCount         int    `json:"friends_count"` // following
	ListedCount          int    `json:"listed_count"`
	FavouritesCount      int    `json:"favourites_count"`
	StatusesCount        int    `json:"statuses_count"`
	CreatedAt            string `json:"created_at"`
	ProfileBannerURL     string `json:"profile_banner_url"`
	ProfileImageURLHTTPS string `json:"profile_image_url_https"`
}

// sdTweet is a single tweet from the tweets timeline endpoint.
type sdTweet struct {
	TweetCreatedAt string       `json:"tweet_created_at"`
	IDStr          string       `json:"id_str"`
	Text           *string      `json:"text"` // may be null
	FullText       string       `json:"full_text"`
	Truncated      bool         `json:"truncated"`
	IsQuoteStatus  bool         `json:"is_quote_status"`
	QuoteCount     int          `json:"quote_count"`
	ReplyCount     int          `json:"reply_count"`
	RetweetCount   int          `json:"retweet_count"`
	FavoriteCount  int          `json:"favorite_count"`
	ViewsCount     int          `json:"views_count"`
	BookmarkCount  int          `json:"bookmark_count"`
	Lang           string       `json:"lang"`
	User           *sdTweetUser `json:"user"`
	// Retweet detection
	RetweetedStatus      *json.RawMessage `json:"retweeted_status"`
	InReplyToStatusIDStr *string          `json:"in_reply_to_status_id_str"`
}

// sdTweetUser is the embedded user object within a tweet.
type sdTweetUser struct {
	IDStr      string `json:"id_str"`
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

// sdTweetsResponse is the top-level response from the tweets endpoint.
type sdTweetsResponse struct {
	NextCursor string    `json:"next_cursor"`
	Tweets     []sdTweet `json:"tweets"`
}

// ──────────────────────────────────────────────────────────────────────────────
// API methods
// ──────────────────────────────────────────────────────────────────────────────

// doRequest performs a GET request with Bearer auth and returns the body.
func (c *socialDataClient) doRequest(endpoint string) ([]byte, int, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("socialdata: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("socialdata: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("socialdata: failed to read response: %w", err)
	}

	return body, resp.StatusCode, nil
}

// FetchUser retrieves a user profile by screen_name (handle without @).
func (c *socialDataClient) FetchUser(screenName string) (*sdUserProfile, error) {
	endpoint := fmt.Sprintf("/twitter/user/%s", screenName)

	body, status, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("socialdata: user profile request failed (status %d): %s", status, string(body))
	}

	var user sdUserProfile
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("socialdata: failed to decode user profile: %w", err)
	}

	return &user, nil
}

// FetchTweets retrieves recent tweets (original, no retweets/replies) for a user.
// Paginates with cursor to collect up to maxTweets tweets.
func (c *socialDataClient) FetchTweets(userID string, maxTweets int) ([]sdTweet, error) {
	if maxTweets <= 0 {
		maxTweets = socialDataMaxTweets
	}

	var allTweets []sdTweet
	cursor := ""
	pagesLoaded := 0
	maxPages := (maxTweets / socialDataTweetsPerPage) + 2 // safety margin

	for pagesLoaded < maxPages {
		endpoint := fmt.Sprintf("/twitter/user/%s/tweets", userID)
		if cursor != "" {
			endpoint += "?cursor=" + cursor
		}

		body, status, err := c.doRequest(endpoint)
		if err != nil {
			return allTweets, err // return whatever we have so far
		}

		if status != http.StatusOK {
			// If we already have some tweets, return them instead of erroring
			if len(allTweets) > 0 {
				util.Log.Warn("[socialdata] tweets page %d returned status %d, returning %d tweets collected so far",
					pagesLoaded+1, status, len(allTweets))
				break
			}
			return nil, fmt.Errorf("socialdata: tweets request failed (status %d): %s", status, string(body))
		}

		var resp sdTweetsResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return allTweets, fmt.Errorf("socialdata: failed to decode tweets response: %w", err)
		}

		// Filter: keep only original tweets (not retweets, not replies)
		for _, t := range resp.Tweets {
			if t.RetweetedStatus != nil {
				continue // skip retweets
			}
			if t.InReplyToStatusIDStr != nil && *t.InReplyToStatusIDStr != "" {
				continue // skip replies
			}
			allTweets = append(allTweets, t)
			if len(allTweets) >= maxTweets {
				break
			}
		}

		pagesLoaded++

		// Stop if we have enough or no more pages
		if len(allTweets) >= maxTweets || resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
	}

	util.Log.Debug("[socialdata] collected %d tweets for user %s across %d pages", len(allTweets), userID, pagesLoaded)
	return allTweets, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Conversion helpers: SocialData → internal TwitterProfile
// ──────────────────────────────────────────────────────────────────────────────

// SocialDataAvailable returns true if the SocialData API key is configured.
func SocialDataAvailable() bool {
	return config.Cfg.SocialDataAPIKey != ""
}

// FetchProfileViaSocialData fetches a Twitter profile using the SocialData API
// and converts it to our internal TwitterProfile format.
func FetchProfileViaSocialData(handle string) (*TwitterProfile, error) {
	client := newSocialDataClient()

	// Step 1: Fetch user profile (also gives us user_id for tweets endpoint)
	handle = strings.TrimPrefix(handle, "@")
	util.Log.Debug("[socialdata] fetching profile for @%s", handle)

	user, err := client.FetchUser(handle)
	if err != nil {
		return nil, fmt.Errorf("socialdata user fetch: %w", err)
	}

	// Step 2: Fetch tweets using the numeric user_id
	util.Log.Debug("[socialdata] fetching tweets for user_id=%s (@%s)", user.IDStr, handle)

	tweets, err := client.FetchTweets(user.IDStr, socialDataMaxTweets)
	if err != nil {
		// Non-fatal: we have the profile, return with whatever tweets we got
		util.Log.Warn("[socialdata] tweet fetch partial failure for @%s: %v (got %d tweets)", handle, err, len(tweets))
	}

	// Step 3: Convert to internal format
	profile := convertSocialDataProfile(user, tweets)
	return profile, nil
}

// convertSocialDataProfile maps SocialData response types to our TwitterProfile.
func convertSocialDataProfile(user *sdUserProfile, tweets []sdTweet) *TwitterProfile {
	profile := &TwitterProfile{
		User: TwitterUser{
			ID:              user.IDStr,
			Name:            user.Name,
			Username:        user.ScreenName,
			Description:     user.Description,
			ProfileImageURL: normalizeAvatarURLFromSocialData(user.ProfileImageURLHTTPS, user.ScreenName),
			PublicMetrics: struct {
				FollowersCount int `json:"followers_count"`
				FollowingCount int `json:"following_count"`
				TweetCount     int `json:"tweet_count"`
			}{
				FollowersCount: user.FollowersCount,
				FollowingCount: user.FriendsCount,
				TweetCount:     user.StatusesCount,
			},
		},
		// Extended fields from SocialData
		Location:        user.Location,
		Verified:        user.Verified,
		CreatedAt:       user.CreatedAt,
		BannerURL:       user.ProfileBannerURL,
		ListedCount:     user.ListedCount,
		FavouritesCount: user.FavouritesCount,
	}

	// Convert tweets
	for _, t := range tweets {
		text := t.FullText
		if text == "" && t.Text != nil {
			text = *t.Text
		}

		profile.Tweets = append(profile.Tweets, TwitterTweet{
			ID:        t.IDStr,
			Text:      text,
			CreatedAt: t.TweetCreatedAt,
		})
	}

	return profile
}

// normalizeAvatarURLFromSocialData ensures we get the higher-res version of the avatar.
func normalizeAvatarURLFromSocialData(imageURL, screenName string) string {
	if imageURL == "" {
		return fmt.Sprintf("https://unavatar.io/twitter/%s", screenName)
	}
	// SocialData returns "_normal.jpg" suffix — replace with "_400x400" for higher res
	return strings.Replace(imageURL, "_normal.", "_400x400.", 1)
}
