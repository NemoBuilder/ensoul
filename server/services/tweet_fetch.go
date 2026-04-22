package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/util"
)

// FetchTweetByURL parses a Twitter/X status URL, fetches the tweet via
// SocialData, and returns an AttachedTweet ready for reply generation.
//
// Resolution order:
//  1. Parse URL → tweet_id + author_handle
//  2. Try SocialData /twitter/tweets/{id} for full text
//  3. If SocialData is unavailable or fails, return a partial AttachedTweet
//     with just URL + author_handle so the caller can decide whether to
//     prompt the user to paste the text manually.
//
// The returned `*AttachedTweet` is never nil unless the URL itself is invalid.
func FetchTweetByURL(rawURL string) (*AttachedTweet, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("empty url")
	}
	m := tweetURLRegex.FindStringSubmatch(rawURL)
	if len(m) != 3 {
		return nil, fmt.Errorf("not a valid twitter/x status URL")
	}
	handle := strings.ToLower(m[1])
	tweetID := m[2]

	at := &AttachedTweet{
		URL:          rawURL,
		AuthorHandle: handle,
	}

	if !SocialDataAvailable() {
		return at, nil
	}

	c := newSocialDataClient()
	body, status, err := c.doRequest("/twitter/tweets/" + tweetID)
	if err != nil {
		util.Log.Warn("[tweet-fetch] socialdata request failed for %s: %v", tweetID, err)
		return at, nil
	}
	if status != http.StatusOK {
		util.Log.Warn("[tweet-fetch] socialdata status %d for tweet %s: %s", status, tweetID, string(body))
		return at, nil
	}

	// SocialData returns the same shape as a single timeline tweet
	var t sdTweet
	if err := json.Unmarshal(body, &t); err != nil {
		util.Log.Warn("[tweet-fetch] socialdata decode failed for %s: %v", tweetID, err)
		return at, nil
	}

	if t.FullText != "" {
		at.Text = t.FullText
	} else if t.Text != nil {
		at.Text = *t.Text
	}
	// Prefer the canonical handle from the API if present
	if t.User != nil && t.User.ScreenName != "" {
		at.AuthorHandle = strings.ToLower(t.User.ScreenName)
	}
	return at, nil
}
