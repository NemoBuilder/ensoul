// Distill extractor — turns one text blob into NodeDraft/EdgeDraft.
//
// Strategy: one LLM call, JSON output schema, defensive parsing. Conf
// scores are taken from the LLM but clamped to [0,1] and downgraded if the
// provenance span is missing or unrooted in the source.
package distill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ensoul-labs/ensoul-server/services/llm"
)

// extractorOutput is the JSON schema the LLM must emit.
type extractorOutput struct {
	Nodes []struct {
		Label      string  `json:"label"`
		NodeType   string  `json:"type"`
		Summary    string  `json:"summary"`
		Confidence float64 `json:"confidence"`
		Provenance string  `json:"provenance"`
	} `json:"nodes"`
	Edges []struct {
		HeadIdx    int     `json:"head"`
		TailIdx    int     `json:"tail"`
		Label      string  `json:"label"`
		Dir        string  `json:"dir"`
		Confidence float64 `json:"confidence"`
		Provenance string  `json:"provenance"`
	} `json:"edges"`
}

// buildPrompt renders the extractor prompt. Kept here (not a template file)
// so the schema stays in sync with extractorOutput in one place.
func buildPrompt(job Job) []llm.Message {
	hints := job.Hints
	nodeTypes := "person, org, concept, event, place, work"
	if len(hints.NodeTypes) > 0 {
		nodeTypes = strings.Join(hints.NodeTypes, ", ")
	}
	edgeVocab := "authored, cites, influenced_by, founded, member_of, located_in"
	if len(hints.EdgeVocab) > 0 {
		edgeVocab = strings.Join(hints.EdgeVocab, ", ")
	}
	maxNodes := hints.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 25
	}

	system := fmt.Sprintf(`You are a knowledge-graph extractor. Read the SOURCE text and emit STRICT JSON with two arrays: "nodes" and "edges".

Rules:
- Output JSON only. No commentary, no markdown fences.
- At most %d nodes. Edges reference nodes by ZERO-BASED index into the nodes array.
- Node "type" must be one of: %s.
- Edge "label" should prefer this vocabulary: %s (but you may add others when clearly warranted).
- "dir" is "directed" or "undirected".
- "confidence" is 0..1 — your honest assessment of how strongly the source supports the claim.
- "provenance" is a short verbatim quote (<=120 chars) from the source that supports the node/edge.
- Skip anything not clearly supported. Quality > quantity.

Schema:
{
  "nodes": [{"label":"...","type":"...","summary":"...","confidence":0.0,"provenance":"..."}],
  "edges": [{"head":0,"tail":1,"label":"...","dir":"directed","confidence":0.0,"provenance":"..."}]
}`, maxNodes, nodeTypes, edgeVocab)

	user := "SOURCE:\n" + job.Text
	return []llm.Message{
		{Role: llm.RoleSystem, Content: system},
		{Role: llm.RoleUser, Content: user},
	}
}

// extract performs one LLM call and parses the JSON envelope.
func extract(ctx context.Context, job Job) (*Result, error) {
	raw, err := llm.Chat(ctx, buildPrompt(job), llm.ChatOpts{
		Temperature: 0.1,
		MaxTokens:   3000,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("extract: llm: %w", err)
	}

	// Defensive strip — some models still wrap JSON in ```json fences.
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var out extractorOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("extract: parse json: %w (got: %.200s)", err, raw)
	}

	res := &Result{
		Nodes: make([]NodeDraft, 0, len(out.Nodes)),
		Edges: make([]EdgeDraft, 0, len(out.Edges)),
	}
	for _, n := range out.Nodes {
		if strings.TrimSpace(n.Label) == "" {
			continue
		}
		res.Nodes = append(res.Nodes, NodeDraft{
			Label:      strings.TrimSpace(n.Label),
			NodeType:   strings.TrimSpace(n.NodeType),
			Summary:    strings.TrimSpace(n.Summary),
			Confidence: clamp01(n.Confidence),
			Provenance: n.Provenance,
		})
	}
	nNodes := len(res.Nodes)
	for _, e := range out.Edges {
		if e.HeadIdx < 0 || e.HeadIdx >= nNodes || e.TailIdx < 0 || e.TailIdx >= nNodes {
			continue // drop edges with bad indices
		}
		dir := strings.ToLower(strings.TrimSpace(e.Dir))
		if dir != "undirected" {
			dir = "directed"
		}
		res.Edges = append(res.Edges, EdgeDraft{
			HeadIdx:    e.HeadIdx,
			TailIdx:    e.TailIdx,
			Label:      strings.TrimSpace(e.Label),
			Dir:        dir,
			Confidence: clamp01(e.Confidence),
			Provenance: e.Provenance,
		})
	}
	return res, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
