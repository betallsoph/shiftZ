// Package llm provides a provider-agnostic client for the language-model
// features shiftbot needs:
//
//  1. parsing free-text availability messages into structured slots,
//  2. translating an owner's natural-language rules into solver penalty
//     rule specifications,
//  3. formatting schedule announcements for the team.
//
// The Provider interface is the only integration point with an actual LLM
// API; concrete providers (Anthropic, OpenAI-compatible, local models, ...)
// are wired in by cmd based on configuration. This package must not be
// imported by store or solver.
package llm

import (
	"context"
	"errors"
	"time"
)

// Request is a single completion request to a provider.
type Request struct {
	// System is the system prompt establishing the task.
	System string
	// Prompt is the user-level input.
	Prompt string
	// MaxTokens caps the response length; 0 lets the provider choose.
	MaxTokens int
}

// Provider is the minimal surface a model backend must implement.
type Provider interface {
	// Complete returns the model's text completion for req.
	Complete(ctx context.Context, req Request) (string, error)
}

// ProviderFunc adapts a function to the Provider interface.
type ProviderFunc func(ctx context.Context, req Request) (string, error)

func (f ProviderFunc) Complete(ctx context.Context, req Request) (string, error) {
	return f(ctx, req)
}

// ErrNoProvider is returned when no LLM backend has been configured.
var ErrNoProvider = errors.New("llm: no provider configured")

// Unconfigured returns a Provider that always fails with ErrNoProvider.
// It lets the bot boot (and serve non-LLM commands) before credentials for
// a real provider are set up.
func Unconfigured() Provider {
	return ProviderFunc(func(context.Context, Request) (string, error) {
		return "", ErrNoProvider
	})
}

// AvailabilitySlot is one contiguous span of (un)availability parsed from an
// employee's free-text message.
type AvailabilitySlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	// Preference matches the solver's scale: 0 unavailable, 1 available,
	// 2 preferred.
	Preference int    `json:"preference"`
	Note       string `json:"note,omitempty"`
}

// RuleSpec is a structured description of an owner rule ("Anna and Bob
// shouldn't work together"), produced from natural language. The caller maps
// Kind/Params onto concrete solver penalty rules.
type RuleSpec struct {
	// Kind names the rule type, e.g. "avoid_pair", "day_off", "custom".
	Kind        string         `json:"kind"`
	Params      map[string]any `json:"params"`
	Weight      float64        `json:"weight"`
	Description string         `json:"description"`
}
