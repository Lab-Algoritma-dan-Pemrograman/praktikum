// Package grading: OpenAI-compatible adapter. Calls POST {BaseURL}/chat/completions
// once per request; suggestions are always manual-review only, same as the fixture.
package grading

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/algohub/algohub-rewrite/backend/internal/entity"
)

const (
	openaiAdapterVersion = "openai-compatible-grader-v1"
	openaiTimeout        = 15 * time.Second
	// Bounded response; a grading suggestion must never approach this.
	maxOpenAIResponse = 128 << 10
	maxChatMessages   = 8
)

var ErrOpenAIBadResponse = errors.New("grading provider returned an unusable response")

// OpenAI is an OpenAI-compatible grading adapter (Ollama, vLLM, OpenRouter, …).
// It never publishes a grade; every suggestion requires manual review upstream.
type OpenAI struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

func NewOpenAI(baseURL, model, apiKey string) *OpenAI {
	return &OpenAI{baseURL: baseURL, model: model, apiKey: apiKey, client: &http.Client{Timeout: openaiTimeout}}
}

func (a *OpenAI) Available() bool { return true }
func (a *OpenAI) Simulated() bool { return false }

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Grade sends one bounded chat request and parses a strict JSON reply.
// Malformed or out-of-bounds replies collapse to needs_manual, never a guess.
func (a *OpenAI) Grade(ctx context.Context, req entity.GradeRequest) (entity.GradeResult, error) {
	if err := ctx.Err(); err != nil {
		return manualFallback(openaiAdapterVersion, false, "cancelled"), err
	}
	if err := ValidateRequest(req); err != nil {
		return manualFallback(openaiAdapterVersion, false, "invalid_request"), err
	}

	prompt := buildGradingPrompt(req)
	payload, err := json.Marshal(chatRequest{
		Model: a.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
		MaxTokens:   2048,
	})
	if err != nil {
		return manualFallback(openaiAdapterVersion, false, "invalid_request"), err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return manualFallback(openaiAdapterVersion, false, "invalid_request"), err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return manualFallback(openaiAdapterVersion, false, "provider_error"), err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, maxOpenAIResponse+1))
	if err != nil {
		return manualFallback(openaiAdapterVersion, false, "provider_error"), err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return manualFallback(openaiAdapterVersion, false, "provider_error"), fmt.Errorf("%w: status %d", ErrOpenAIBadResponse, httpResp.StatusCode)
	}

	var chat chatResponse
	if err := json.Unmarshal(body, &chat); err != nil || len(chat.Choices) == 0 {
		return manualFallback(openaiAdapterVersion, false, "provider_error"), ErrOpenAIBadResponse
	}

	result, err := parseSuggestions(req, chat.Choices[0].Message.Content)
	if err != nil {
		return manualFallback(openaiAdapterVersion, false, "invalid_result"), err
	}
	return AcceptResult(req, result)
}

const systemPrompt = `You are a grading assistant. You receive questions, rubric criteria, a reference answer, and a student answer as JSON. Reply with ONLY a JSON object of the form {"suggestions":[{"question_id":"...","score":<int>,"feedback":"..."}]} — one entry per item, score between 0 and the sum of the rubric max scores, feedback at most 500 characters. No markdown, no prose outside the JSON.`

func buildGradingPrompt(req entity.GradeRequest) string {
	var b strings.Builder
	b.WriteString(`{"items":[`)
	for i, item := range req.Items {
		if i > 0 {
			b.WriteByte(',')
		}
		rubric := make([]map[string]any, 0, len(item.Rubric))
		for _, c := range item.Rubric {
			rubric = append(rubric, map[string]any{"id": c.ID, "description": c.Description, "max_score": c.MaxScore})
		}
		enc, _ := json.Marshal(map[string]any{
			"question_id":      item.QuestionID,
			"type":             item.Type,
			"prompt":           item.Prompt,
			"answer":           item.Answer,
			"max_score":        item.MaxScore,
			"rubric":           rubric,
			"reference_answer": item.ReferenceAnswer,
			"language":         item.Language,
		})
		b.Write(enc)
	}
	b.WriteString(`]}`)
	return b.String()
}

func parseSuggestions(req entity.GradeRequest, content string) (entity.GradeResult, error) {
	content = strings.TrimSpace(content)
	// Strip a markdown fence if the model added one anyway.
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		Suggestions []entity.SuggestedGrade `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return entity.GradeResult{}, ErrOpenAIBadResponse
	}
	result := entity.GradeResult{
		Status:          StatusSuggested,
		ProviderVersion: openaiAdapterVersion,
		Suggestions:     parsed.Suggestions,
	}
	return result, nil
}

var _ Grader = (*OpenAI)(nil)

// Complete sends one bounded raw chat completion (shared by non-grading LLM
// features such as rumpang generation). No retries; caller owns deadlines.
func (a *OpenAI) Complete(ctx context.Context, system, user string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(chatRequest{
		Model:       a.model,
		Messages:    []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		Temperature: 0,
		MaxTokens:   4096,
	})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	httpResp, err := a.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, maxOpenAIResponse+1))
	if err != nil {
		return "", err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: status %d", ErrOpenAIBadResponse, httpResp.StatusCode)
	}
	var chat chatResponse
	if err := json.Unmarshal(body, &chat); err != nil || len(chat.Choices) == 0 {
		return "", ErrOpenAIBadResponse
	}
	content := strings.TrimSpace(chat.Choices[0].Message.Content)
	// Strip a markdown fence if the model added one anyway.
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content), nil
}
