package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/util"
)

// TwitterUser holds basic user profile data from the Twitter API.
type TwitterUser struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Username        string `json:"username"`
	Description     string `json:"description"`
	ProfileImageURL string `json:"profile_image_url"`
	PublicMetrics   struct {
		FollowersCount int `json:"followers_count"`
		FollowingCount int `json:"following_count"`
		TweetCount     int `json:"tweet_count"`
	} `json:"public_metrics"`
}

// TwitterTweet holds a single tweet from the Twitter API.
type TwitterTweet struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// TwitterProfile aggregates user info and recent tweets for seed extraction.
type TwitterProfile struct {
	User   TwitterUser    `json:"user"`
	Tweets []TwitterTweet `json:"tweets"`

	// Extended fields (populated by SocialData, optional for Twitter v2)
	Location        string `json:"location,omitempty"`
	Verified        bool   `json:"verified,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	BannerURL       string `json:"banner_url,omitempty"`
	ListedCount     int    `json:"listed_count,omitempty"`
	FavouritesCount int    `json:"favourites_count,omitempty"`
	DataSource      string `json:"data_source"` // "socialdata", "twitter_v2", "mock"
}

// FetchTwitterProfile retrieves a user's profile and recent tweets.
// Priority: SocialData API → Twitter v2 API → mock fallback.
func FetchTwitterProfile(handle string) (*TwitterProfile, error) {
	handle = strings.TrimPrefix(handle, "@")

	// 1. Try SocialData API (primary source)
	if SocialDataAvailable() {
		profile, err := FetchProfileViaSocialData(handle)
		if err == nil {
			profile.DataSource = "socialdata"
			util.Log.Debug("[twitter] fetched @%s via SocialData (%d tweets)", handle, len(profile.Tweets))
			return profile, nil
		}
		util.Log.Warn("[twitter] SocialData failed for @%s, trying Twitter v2: %v", handle, err)
	}

	// 2. Try Twitter v2 API
	token := config.Cfg.TwitterBearerToken
	if token != "" {
		user, err := fetchTwitterUser(handle, token)
		if err != nil {
			util.Log.Warn("[twitter] Twitter v2 user fetch failed for @%s: %v", handle, err)
		} else {
			tweets, err := fetchUserTweets(user.ID, token)
			if err != nil {
				util.Log.Warn("[twitter] Twitter v2 tweet fetch failed for @%s: %v", handle, err)
				tweets = nil // continue with just profile
			}
			profile := &TwitterProfile{
				User:       *user,
				Tweets:     tweets,
				DataSource: "twitter_v2",
			}
			util.Log.Debug("[twitter] fetched @%s via Twitter v2 (%d tweets)", handle, len(tweets))
			return profile, nil
		}
	}

	// 3. Mock fallback
	util.Log.Debug("[twitter] no API available for @%s, using mock fallback", handle)
	profile := mockTwitterProfile(handle)
	profile.DataSource = "mock"
	return profile, nil
}

func fetchTwitterUser(username, token string) (*TwitterUser, error) {
	params := url.Values{}
	params.Set("user.fields", "id,name,username,description,profile_image_url,public_metrics")

	apiURL := fmt.Sprintf("https://api.twitter.com/2/users/by/username/%s?%s",
		url.PathEscape(username), params.Encode())

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data TwitterUser `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode Twitter user response: %w", err)
	}

	return &result.Data, nil
}

func fetchUserTweets(userID, token string) ([]TwitterTweet, error) {
	params := url.Values{}
	params.Set("max_results", "50")
	params.Set("tweet.fields", "id,text,created_at")
	params.Set("exclude", "retweets,replies")

	apiURL := fmt.Sprintf("https://api.twitter.com/2/users/%s/tweets?%s",
		url.PathEscape(userID), params.Encode())

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Twitter API error (status %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []TwitterTweet `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode Twitter tweets response: %w", err)
	}

	return result.Data, nil
}

// mockTwitterProfile returns a placeholder profile when Twitter API is not configured.
// Tweets are left empty so the seed prompt tells the LLM to rely on public knowledge.
func mockTwitterProfile(handle string) *TwitterProfile {
	return &TwitterProfile{
		User: TwitterUser{
			ID:              "mock_" + handle,
			Name:            handle,
			Username:        handle,
			Description:     "", // leave blank — LLM will use its own knowledge
			ProfileImageURL: fmt.Sprintf("https://unavatar.io/twitter/%s", handle),
			PublicMetrics: struct {
				FollowersCount int `json:"followers_count"`
				FollowingCount int `json:"following_count"`
				TweetCount     int `json:"tweet_count"`
			}{
				FollowersCount: 0,
				FollowingCount: 0,
				TweetCount:     0,
			},
		},
		Tweets: nil, // empty — triggers LLM public-knowledge fallback in seed prompt
	}
}

// IsMockProfile returns true if the profile was generated by the mock fallback
// (i.e. no real Twitter data is available).
func IsMockProfile(profile *TwitterProfile) bool {
	return profile.DataSource == "mock" || strings.HasPrefix(profile.User.ID, "mock_")
}

// FormatTweetsForLLM formats tweets into a readable text block for LLM input.
func FormatTweetsForLLM(tweets []TwitterTweet) string {
	var sb strings.Builder
	for i, t := range tweets {
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, t.Text))
		if t.CreatedAt != "" {
			sb.WriteString(fmt.Sprintf("    (posted: %s)\n", t.CreatedAt))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
