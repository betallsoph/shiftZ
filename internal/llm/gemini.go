package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultGeminiModel = "gemini-3.5-flash"

// GeminiProvider calls the Google Gemini Interactions REST API.
type GeminiProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewGeminiProvider returns a Gemini REST client.
func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	if strings.TrimSpace(model) == "" {
		model = defaultGeminiModel
	}
	return &GeminiProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://generativelanguage.googleapis.com/v1beta",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Complete creates a stateless interaction and returns model text.
func (p *GeminiProvider) Complete(ctx context.Context, req Request) (string, error) {
	payload := map[string]any{
		"model": p.model,
		"input": req.Prompt,
		"store": false,
	}
	if req.System != "" {
		payload["system_instruction"] = req.System
	}

	genConfig := map[string]any{"thinking_level": "minimal"}
	if req.MaxTokens > 0 {
		genConfig["max_output_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 || req.ResponseMIMEType != "" {
		genConfig["temperature"] = req.Temperature
	}
	if len(genConfig) > 0 {
		payload["generation_config"] = genConfig
	}

	if req.ResponseMIMEType != "" || req.ResponseSchema != nil {
		responseFormat := map[string]any{"type": "text"}
		if req.ResponseMIMEType != "" {
			responseFormat["mime_type"] = req.ResponseMIMEType
		}
		if req.ResponseSchema != nil {
			responseFormat["schema"] = req.ResponseSchema
		}
		payload["response_format"] = responseFormat
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/interactions", p.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gemini: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("gemini: request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("gemini: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini: API %s: %s", resp.Status, geminiErrorMessage(respBody))
	}

	var parsed geminiInteractionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("gemini: decode response: %w", err)
	}
	text, err := parsed.text()
	if err != nil {
		return "", err
	}
	return text, nil
}

type geminiInteractionResponse struct {
	Steps []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"steps"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (r geminiInteractionResponse) text() (string, error) {
	if r.Error != nil && r.Error.Message != "" {
		return "", fmt.Errorf("gemini: %s", r.Error.Message)
	}
	var text strings.Builder
	for _, step := range r.Steps {
		if step.Type != "model_output" {
			continue
		}
		for _, content := range step.Content {
			if content.Type == "text" {
				text.WriteString(content.Text)
			}
		}
	}
	if strings.TrimSpace(text.String()) == "" {
		return "", fmt.Errorf("gemini: empty model output")
	}
	return text.String(), nil
}

func geminiErrorMessage(body []byte) string {
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Error.Message != "" {
		return errResp.Error.Message
	}
	return strings.TrimSpace(string(body))
}
