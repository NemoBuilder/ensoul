// Package llm — multi-backend LLM client.
//
// Phase 1 scope: a single Chat(messages) → string interface, with a thin
// adapter per provider (OpenAI / DeepSeek / etc.). Implementations live in
// sibling files (openai.go, deepseek.go, …). Keep adapters tiny; the
// orchestration (caching, retry, token-cap) lives here so providers are
// interchangeable.
//
// V3 already wires OPENAI_API_KEY / DEEPSEEK_API_KEY through config.Cfg.
// We reuse those env vars here — DO NOT add a parallel set of LLM env vars.
package llm

import (
	"context"
	"errors"
	"strings"
)

// Role constants for chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message is one entry in a chat completion request.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOpts narrows behaviour of a single completion call.
type ChatOpts struct {
	Model       string  // empty → provider default
	Temperature float64 // 0 → deterministic
	MaxTokens   int     // 0 → provider default
	JSONMode    bool    // request JSON-only output when supported
}

// Provider is implemented by each LLM backend adapter.
type Provider interface {
	Name() string
	Chat(ctx context.Context, msgs []Message, opts ChatOpts) (string, error)
}

// defaultProvider is the active backend for Chat(). Set by Register().
var defaultProvider Provider

// Register makes a provider the default LLM backend. Idempotent overwrite.
func Register(p Provider) {
	defaultProvider = p
}

// Default returns the registered default provider, or nil.
func Default() Provider { return defaultProvider }

// Chat is the package-level convenience for the registered default provider.
// Returns an error if no provider has been registered yet (rather than
// panicking) so the distill pipeline can degrade gracefully in tests.
func Chat(ctx context.Context, msgs []Message, opts ChatOpts) (string, error) {
	if defaultProvider == nil {
		return "", errors.New("llm: no provider registered")
	}
	out, err := defaultProvider.Chat(ctx, msgs, opts)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
