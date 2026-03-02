package services

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ensoul-labs/ensoul-server/util"
)

// ──────────────────────────────────────────────────────────────────────────────
// SSE Hub — Sniper 2.0 real-time tweet push via Server-Sent Events
// ──────────────────────────────────────────────────────────────────────────────

const (
	sseBufferSize  = 50            // per-client event buffer
	sseMaxConnTime = 2 * time.Hour // force disconnect after 2h
)

// SSEEvent represents an event to send to SSE clients.
type SSEEvent struct {
	Type string `json:"type"` // "new_tweets", "heartbeat", "tag_refreshed", "error"
	Data string `json:"data"` // JSON string
}

// SSEHub manages SSE client connections grouped by tag subscriptions.
type SSEHub struct {
	mu      sync.RWMutex
	clients map[string]map[chan SSEEvent]bool // tag_id → set of client channels
}

// Global SSE hub instance
var sseHubInstance *SSEHub

// InitSSEHub initializes the global SSE hub.
func InitSSEHub() *SSEHub {
	sseHubInstance = &SSEHub{
		clients: make(map[string]map[chan SSEEvent]bool),
	}
	util.Log.Info("[sse] SSE hub initialized")
	return sseHubInstance
}

// GetSSEHub returns the global SSE hub instance.
func GetSSEHub() *SSEHub {
	return sseHubInstance
}

// Subscribe creates a new SSE channel subscribed to the given tags.
// Returns the channel and an unsubscribe function.
func (h *SSEHub) Subscribe(tagIDs []string) (<-chan SSEEvent, func()) {
	ch := make(chan SSEEvent, sseBufferSize)

	h.mu.Lock()
	for _, tid := range tagIDs {
		if h.clients[tid] == nil {
			h.clients[tid] = make(map[chan SSEEvent]bool)
		}
		h.clients[tid][ch] = true
	}
	h.mu.Unlock()

	// Return cleanup function
	unsub := func() {
		h.mu.Lock()
		for _, tid := range tagIDs {
			delete(h.clients[tid], ch)
			// Clean up empty tag maps
			if len(h.clients[tid]) == 0 {
				delete(h.clients, tid)
			}
		}
		h.mu.Unlock()
		// Drain and close
		close(ch)
	}

	return ch, unsub
}

// Broadcast sends new tweets to all clients subscribed to the given tag.
func (h *SSEHub) Broadcast(tagID string, tweets []TweetCard) {
	if len(tweets) == 0 {
		return
	}

	data, err := json.Marshal(map[string]interface{}{
		"tag_id": tagID,
		"tweets": tweets,
	})
	if err != nil {
		util.Log.Warn("[sse] Failed to marshal broadcast data: %v", err)
		return
	}

	event := SSEEvent{
		Type: "new_tweets",
		Data: string(data),
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.clients[tagID]
	if len(clients) == 0 {
		return
	}

	sent := 0
	for ch := range clients {
		select {
		case ch <- event:
			sent++
		default:
			// Client is too slow, drop the event (non-blocking)
		}
	}

	if sent > 0 {
		util.Log.Debug("[sse] Broadcast %d tweets for tag %s to %d/%d clients", len(tweets), tagID, sent, len(clients))
	}
}

// BroadcastTagRefreshed notifies clients that a tag's cache has been refreshed.
func (h *SSEHub) BroadcastTagRefreshed(tagID string, tweetCount int, cacheAge int) {
	data, _ := json.Marshal(map[string]interface{}{
		"tag_id":      tagID,
		"tweet_count": tweetCount,
		"cache_age":   cacheAge,
	})

	event := SSEEvent{
		Type: "tag_refreshed",
		Data: string(data),
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.clients[tagID] {
		select {
		case ch <- event:
		default:
		}
	}
}

// ClientCount returns the total number of connected SSE clients.
func (h *SSEHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Deduplicate across tags (one client can subscribe to multiple tags)
	allChans := make(map[chan SSEEvent]bool)
	for _, clients := range h.clients {
		for ch := range clients {
			allChans[ch] = true
		}
	}
	return len(allChans)
}

// TagClientCounts returns the number of SSE clients per tag.
func (h *SSEHub) TagClientCounts() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	counts := make(map[string]int)
	for tagID, clients := range h.clients {
		counts[tagID] = len(clients)
	}
	return counts
}
