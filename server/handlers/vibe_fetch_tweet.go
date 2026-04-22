package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// VibeFetchTweet handles GET /api/vibe-write/fetch-tweet?url=<status URL>
//
// Resolves a Twitter/X status URL into a structured AttachedTweet so the
// frontend can auto-attach it instead of dumping a bare URL into the LLM
// (which would just answer "I can't access external links").
//
// Response shape:
//
//	{ url, author_handle, text }
//
// `text` may be empty when SocialData is not configured or the API call
// fails — the frontend should fall back to asking the user to paste the
// tweet body. Always 200 unless the URL is malformed.
func VibeFetchTweet(c *gin.Context) {
	if _, _, ok := getEmailSessionUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	rawURL := c.Query("url")
	at, err := services.FetchTweetByURL(rawURL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url":           at.URL,
		"author_handle": at.AuthorHandle,
		"text":          at.Text,
	})
}
