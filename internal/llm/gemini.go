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

// GeminiProvider calls the Google Gemini generateContent REST API.
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

// Complete sends a generateContent request and returns model text.
func (p *GeminiProvider) Complete(ctx context.Context, req Request) (string, error) {
	payload := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]string{{"text": req.Prompt}},
		}},
	}
	if req.System != "" {
		payload["system_instruction"] = map[string]any{
			"parts": []map[string]string{{"text": req.System}},
		}
	}

	genConfig := map[string]any{}
	if req.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = req.MaxTokens
	}
	if req.Temperature > 0 || req.ResponseMIMEType != "" {
		genConfig["temperature"] = req.Temperature
	}
	if req.ResponseMIMEType != "" {
		genConfig["responseMimeType"] = req.ResponseMIMEType
	}
	if req.ResponseSchema != nil {
		genConfig["responseSchema"] = req.ResponseSchema
	}
	if len(genConfig) > 0 {
		payload["generationConfig"] = genConfig
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("gemini: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", p.baseURL, p.model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gemini: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

	var parsed geminiGenerateResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("gemini: decode response: %w", err)
	}
	text, err := parsed.text()
	if err != nil {
		return "", err
	}
	return text, nil
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func (r geminiGenerateResponse) text() (string, error) {
	if r.Error != nil && r.Error.Message != "" {
		return "", fmt.Errorf("gemini: %s", r.Error.Message)
	}
	if len(r.Candidates) == 0 {
		return "", fmt.Errorf("gemini: empty candidates")
	}
	parts := r.Candidates[0].Content.Parts
	if len(parts) == 0 || strings.TrimSpace(parts[0].Text) == "" {
		return "", fmt.Errorf("gemini: empty candidate text")
	}
	return parts[0].Text, nil
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
