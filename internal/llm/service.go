package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Service implements shiftbot's three LLM use cases on top of any Provider.
type Service struct {
	provider Provider
}

// NewService wraps a provider.
func NewService(p Provider) *Service {
	return &Service{provider: p}
}

const parseAvailabilitySystem = `You convert restaurant staff availability messages into JSON.
Respond with ONLY a JSON array of slots, no prose, no code fences. Each slot:
{"start":"RFC3339 timestamp","end":"RFC3339 timestamp","preference":0|1|2,"note":"optional"}
preference: 0 = cannot work, 1 = can work, 2 = prefers to work.
Timestamps must fall inside the week starting at the given date and use the given timezone offset.`

// ParseAvailability turns a free-text message ("I can do mornings except
// Wednesday, prefer Friday evening") into structured slots for the week
// starting at weekStart, interpreted in loc.
func (s *Service) ParseAvailability(ctx context.Context, text string, weekStart time.Time, loc *time.Location) ([]AvailabilitySlot, error) {
	if loc == nil {
		loc = time.UTC
	}
	prompt := fmt.Sprintf("Week starts on %s (timezone %s).\nMessage:\n%s",
		weekStart.In(loc).Format("Monday 2006-01-02"), loc.String(), text)

	raw, err := s.provider.Complete(ctx, Request{
		System:    parseAvailabilitySystem,
		Prompt:    prompt,
		MaxTokens: 2048,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: parse availability: %w", err)
	}
	var slots []AvailabilitySlot
	if err := json.Unmarshal([]byte(extractJSON(raw)), &slots); err != nil {
		return nil, fmt.Errorf("llm: availability response is not valid JSON: %w", err)
	}
	for i, slot := range slots {
		if !slot.End.After(slot.Start) {
			return nil, fmt.Errorf("llm: availability slot %d has end before start", i)
		}
	}
	return slots, nil
}

const translateRuleSystem = `You translate a restaurant owner's scheduling rule into JSON for a shift solver.
Respond with ONLY one JSON object, no prose, no code fences:
{"kind":"avoid_pair"|"day_off"|"custom","params":{...},"weight":number,"description":"restatement of the rule"}
Known kinds:
- avoid_pair: params {"a":"employee name","b":"employee name"} — the two should not share a shift.
- day_off: params {"employee":"name","weekday":0-6} — 0 is Sunday; the employee should not work that day.
- custom: params free-form when the rule fits neither kind.
weight expresses importance from 1 (nice to have) to 10 (very important).`

// TranslateRule turns owner text ("never put Anna and Bob on the same
// shift") into a RuleSpec the caller can map onto solver penalty rules.
func (s *Service) TranslateRule(ctx context.Context, text string) (*RuleSpec, error) {
	raw, err := s.provider.Complete(ctx, Request{
		System:    translateRuleSystem,
		Prompt:    text,
		MaxTokens: 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: translate rule: %w", err)
	}
	var spec RuleSpec
	if err := json.Unmarshal([]byte(extractJSON(raw)), &spec); err != nil {
		return nil, fmt.Errorf("llm: rule response is not valid JSON: %w", err)
	}
	if spec.Kind == "" {
		return nil, fmt.Errorf("llm: rule response missing kind")
	}
	return &spec, nil
}

const formatAnnouncementSystem = `You write short, friendly Telegram announcements for restaurant staff schedules.
Given a plain-text schedule summary, produce a concise message listing each day's assignments.
Plain text only (no markdown tables), suitable for a group chat.`

// FormatAnnouncement renders a finalized schedule summary into a
// human-friendly announcement message.
func (s *Service) FormatAnnouncement(ctx context.Context, scheduleSummary string) (string, error) {
	out, err := s.provider.Complete(ctx, Request{
		System:    formatAnnouncementSystem,
		Prompt:    scheduleSummary,
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("llm: format announcement: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// extractJSON strips code fences and any prose around the first JSON value
// in a model response.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.IndexAny(s, "[{")
	if start < 0 {
		return s
	}
	var end int
	if s[start] == '[' {
		end = strings.LastIndexByte(s, ']')
	} else {
		end = strings.LastIndexByte(s, '}')
	}
	if end <= start {
		return s
	}
	return s[start : end+1]
}
