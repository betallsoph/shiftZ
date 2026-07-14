package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewGeminiProviderDefaultModel(t *testing.T) {
	p := NewGeminiProvider("key", "")
	if p.model != defaultGeminiModel {
		t.Fatalf("model = %q, want %q", p.model, defaultGeminiModel)
	}
}

func TestGeminiProviderBuildsStructuredRequest(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/interactions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Has("key") {
			t.Fatal("API key must not be sent in the query string")
		}
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatalf("x-goog-api-key header = %q", r.Header.Get("x-goog-api-key"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"steps":[{"type":"model_output","content":[{"type":"text","text":"{\"ok\":true}"}]}]}`))
	}))
	defer srv.Close()

	p := NewGeminiProvider("test-key", "gemini-3.5-flash")
	p.baseURL = srv.URL
	p.client = srv.Client()

	schema := map[string]any{"type": "object"}
	_, err := p.Complete(context.Background(), Request{
		System:           "system prompt",
		Prompt:           "user prompt",
		MaxTokens:        1200,
		Temperature:      0,
		ResponseMIMEType: "application/json",
		ResponseSchema:   schema,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got["model"] != "gemini-3.5-flash" || got["input"] != "user prompt" {
		t.Fatalf("model/input = %v/%v", got["model"], got["input"])
	}
	if got["store"] != false {
		t.Fatalf("store = %v, want false", got["store"])
	}
	if got["system_instruction"] != "system prompt" {
		t.Fatalf("system_instruction = %#v", got["system_instruction"])
	}
	gen, ok := got["generation_config"].(map[string]any)
	if !ok {
		t.Fatalf("generation_config = %#v", got["generation_config"])
	}
	if gen["max_output_tokens"] != float64(1200) {
		t.Fatalf("max_output_tokens = %v", gen["max_output_tokens"])
	}
	if gen["thinking_level"] != "minimal" {
		t.Fatalf("thinking_level = %v", gen["thinking_level"])
	}
	format, ok := got["response_format"].(map[string]any)
	if !ok || format["mime_type"] != "application/json" {
		t.Fatalf("response_format = %#v", got["response_format"])
	}
	schemaGot, ok := format["schema"].(map[string]any)
	if !ok || schemaGot["type"] != "object" {
		t.Fatalf("schema = %#v", format["schema"])
	}
}

func TestGeminiProviderExtractsModelOutputText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"steps":[{"type":"thought","content":[{"type":"text","text":"ignore"}]},{"type":"model_output","content":[{"type":"text","text":"hello"}]}]}`))
	}))
	defer srv.Close()

	p := NewGeminiProvider("test-key", "gemini-3.5-flash")
	p.baseURL = srv.URL
	p.client = srv.Client()

	out, err := p.Complete(context.Background(), Request{Prompt: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello" {
		t.Fatalf("out = %q", out)
	}
}

func TestGeminiProviderHandlesNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()

	p := NewGeminiProvider("test-key", "gemini-3.5-flash")
	p.baseURL = srv.URL
	p.client = srv.Client()

	_, err := p.Complete(context.Background(), Request{Prompt: "hi"})
	if err == nil || !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseAvailabilityStructuredObject(t *testing.T) {
	weekStart := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	start := weekStart.Add(8 * time.Hour).Format(time.RFC3339)
	end := weekStart.Add(14 * time.Hour).Format(time.RFC3339)

	provider := ProviderFunc(func(_ context.Context, req Request) (string, error) {
		if req.ResponseMIMEType != "application/json" {
			t.Fatalf("mime = %q", req.ResponseMIMEType)
		}
		if req.ResponseSchema == nil {
			t.Fatal("expected response schema")
		}
		if req.Temperature != 0 {
			t.Fatalf("temperature = %v", req.Temperature)
		}
		return `{"slots":[{"start":"` + start + `","end":"` + end + `","preference":1}],"uncertain":false,"questions":[]}`, nil
	})

	svc := NewService(provider)
	slots, err := svc.ParseAvailability(context.Background(), "Mon morning", weekStart, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 1 || slots[0].Preference != 1 {
		t.Fatalf("slots = %+v", slots)
	}
}

func TestParseAvailabilityClarificationError(t *testing.T) {
	provider := ProviderFunc(func(context.Context, Request) (string, error) {
		return `{"slots":[],"uncertain":true,"questions":["Did you mean morning or evening?"]}`, nil
	})
	svc := NewService(provider)

	_, err := svc.ParseAvailability(context.Background(), "maybe Monday", time.Now(), time.UTC)
	var clarify *ClarificationError
	if !errors.As(err, &clarify) {
		t.Fatalf("err = %v, want ClarificationError", err)
	}
	if len(clarify.Questions) != 1 {
		t.Fatalf("questions = %v", clarify.Questions)
	}
}
