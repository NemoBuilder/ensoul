// V3-backed provider for services/llm.
//
// Rather than re-implement HTTP / streaming / retry, this adapter delegates
// to the V3 services.CallLLM family (services/llm.go) which already routes
// to OpenAI / Claude / OpenAI-compatible base URLs based on Cfg.LLMProvider.
//
// Wire-up: call llm.UseV3Default() once from main.go after config.Load().
package llm

import (
	"context"

	"github.com/ensoul-labs/ensoul-server/services"
)

type v3Provider struct{}

func (v3Provider) Name() string { return "v3-llm" }

func (v3Provider) Chat(_ context.Context, msgs []Message, opts ChatOpts) (string, error) {
	// Translate to V3 ChatMessage. Roles match 1:1.
	v3msgs := make([]services.ChatMessage, len(msgs))
	for i, m := range msgs {
		v3msgs[i] = services.ChatMessage{Role: m.Role, Content: m.Content}
	}
	max := opts.MaxTokens
	if max <= 0 {
		max = 2000
	}
	temp := opts.Temperature
	// V3 CallLLM has no JSON-mode flag; we instead instruct the model via a
	// system message in distill (extractor.go). JSON parsing happens caller-side.
	return services.CallLLM(v3msgs, max, temp)
}

// UseV3Default registers the V3-backed provider as the default LLM.
// Safe to call multiple times — Register is idempotent.
func UseV3Default() {
	Register(v3Provider{})
}
