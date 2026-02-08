package services

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/util"
)

// --- OpenAI-compatible API client ---

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the request body for the OpenAI Chat Completions API.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatChoice is a single choice in the response.
type ChatChoice struct {
	Index   int         `json:"index"`
	Message ChatMessage `json:"message"`
}

// ChatResponse is the full non-streaming response from the API.
type ChatResponse struct {
	ID      string       `json:"id"`
	Choices []ChatChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// StreamDelta is the delta object in a streaming response chunk.
type StreamDelta struct {
	Content string `json:"content"`
}

// StreamChoice is a single choice in a streaming chunk.
type StreamChoice struct {
	Index int         `json:"index"`
	Delta StreamDelta `json:"delta"`
}

// StreamChunk is one chunk of a streaming response.
type StreamChunk struct {
	ID      string         `json:"id"`
	Choices []StreamChoice `json:"choices"`
}

// llmBaseURL returns the API base URL for the configured LLM provider.
func llmBaseURL() string {
	switch strings.ToLower(config.Cfg.LLMProvider) {
	case "claude", "anthropic":
		return "https://api.anthropic.com/v1"
	default: // "openai" or compatible (deepseek, openrouter, etc.)
		base := config.Cfg.LLMBaseURL
		if base == "" {
			return "https://api.openai.com/v1"
		}
		return strings.TrimRight(base, "/")
	}
}

// CallLLM sends a non-streaming chat completion request and returns the assistant's reply.
func CallLLM(messages []ChatMessage, maxTokens int, temperature float64) (string, error) {
	cfg := config.Cfg
	if cfg.LLMAPIKey == "" {
		return "", fmt.Errorf("LLM_API_KEY not configured")
	}

	provider := strings.ToLower(cfg.LLMProvider)

	if provider == "claude" || provider == "anthropic" {
		return callClaude(messages, maxTokens, temperature)
	}

	return callOpenAI(messages, maxTokens, temperature, false)
}

// StreamLLM sends a streaming chat completion request and calls onChunk for each token.
func StreamLLM(messages []ChatMessage, maxTokens int, temperature float64, onChunk func(content string)) error {
	cfg := config.Cfg
	if cfg.LLMAPIKey == "" {
		return fmt.Errorf("LLM_API_KEY not configured")
	}

	provider := strings.ToLower(cfg.LLMProvider)

	if provider == "claude" || provider == "anthropic" {
		return streamClaude(messages, maxTokens, temperature, onChunk)
	}

	return streamOpenAI(messages, maxTokens, temperature, onChunk)
}

// --- OpenAI implementation ---

func callOpenAI(messages []ChatMessage, maxTokens int, temperature float64, _ bool) (string, error) {
	cfg := config.Cfg

	reqBody := ChatRequest{
		Model:       cfg.LLMModel,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      false,
	}

	body, _ := json.Marshal(reqBody)
	url := llmBaseURL() + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.LLMAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	util.Log.Debug("[llm] Tokens used: prompt=%d, completion=%d, total=%d",
		chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens)

	return chatResp.Choices[0].Message.Content, nil
}

func streamOpenAI(messages []ChatMessage, maxTokens int, temperature float64, onChunk func(string)) error {
	cfg := config.Cfg

	reqBody := ChatRequest{
		Model:       cfg.LLMModel,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
	}

	body, _ := json.Marshal(reqBody)
	url := llmBaseURL() + "/chat/completions"

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.LLMAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("LLM streaming request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LLM API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed chunks
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				onChunk(choice.Delta.Content)
			}
		}
	}

	return scanner.Err()
}

// --- Anthropic Claude implementation ---

// claudeRequest is the request body for the Anthropic Messages API.
type claudeRequest struct {
	Model       string        `json:"model"`
	MaxTokens   int           `json:"max_tokens"`
	System      string        `json:"system,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// claudeResponse is the response from Anthropic Messages API.
type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func callClaude(messages []ChatMessage, maxTokens int, temperature float64) (string, error) {
	cfg := config.Cfg

	// Extract system message
	var system string
	var userMessages []ChatMessage
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			userMessages = append(userMessages, m)
		}
	}

	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := claudeRequest{
		Model:       cfg.LLMModel,
		MaxTokens:   maxTokens,
		System:      system,
		Messages:    userMessages,
		Temperature: temperature,
		Stream:      false,
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.LLMAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Claude API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Claude API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&claudeResp); err != nil {
		return "", fmt.Errorf("failed to decode Claude response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("Claude returned no content")
	}

	util.Log.Debug("[llm] Claude tokens: input=%d, output=%d",
		claudeResp.Usage.InputTokens, claudeResp.Usage.OutputTokens)

	return claudeResp.Content[0].Text, nil
}

func streamClaude(messages []ChatMessage, maxTokens int, temperature float64, onChunk func(string)) error {
	cfg := config.Cfg

	// Extract system message
	var system string
	var userMessages []ChatMessage
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			userMessages = append(userMessages, m)
		}
	}

	if maxTokens == 0 {
		maxTokens = 4096
	}

	reqBody := claudeRequest{
		Model:       cfg.LLMModel,
		MaxTokens:   maxTokens,
		System:      system,
		Messages:    userMessages,
		Temperature: temperature,
		Stream:      true,
	}

	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.LLMAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Claude streaming request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Claude API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		// Claude sends content_block_delta events with text
		eventType, _ := event["type"].(string)
		if eventType == "content_block_delta" {
			if delta, ok := event["delta"].(map[string]interface{}); ok {
				if text, ok := delta["text"].(string); ok && text != "" {
					onChunk(text)
				}
			}
		}
	}

	return scanner.Err()
}

// CallLLMJSON is a convenience function that calls the LLM and parses JSON from the response.
// It strips markdown code fences if present.
func CallLLMJSON(messages []ChatMessage, maxTokens int, temperature float64, result interface{}) error {
	raw, err := CallLLM(messages, maxTokens, temperature)
	if err != nil {
		return err
	}

	// Strip markdown code fences if present
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "```json") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	} else if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)
	}

	if err := json.Unmarshal([]byte(cleaned), result); err != nil {
		return fmt.Errorf("failed to parse LLM JSON response: %w\nraw response:\n%s", err, raw)
	}

	return nil
}
