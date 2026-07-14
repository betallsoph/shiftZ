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

const parseAvailabilitySystem = `You convert restaurant staff availability messages into structured JSON for shift scheduling.

Rules:
- Interpret all dates and times in the shop timezone given in the user message.
- Every slot must fall inside the target week (Monday 00:00 through the following Monday 00:00).
- Use RFC3339 timestamps with the correct timezone offset for start and end.
- preference: 0 = unavailable, 1 = available, 2 = preferred.
- Staff may write in Vietnamese or English.
- If the message is ambiguous, vague ("maybe", "not sure"), or missing needed days/times, set uncertain=true and add short clarification questions.
- Do not invent exact times when the employee did not provide them.
- When uncertain, slots may be empty.`

func availabilityResponseSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"slots": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"start":      map[string]any{"type": "string"},
						"end":        map[string]any{"type": "string"},
						"preference": map[string]any{"type": "integer"},
						"note":       map[string]any{"type": "string"},
					},
					"required": []string{"start", "end", "preference"},
				},
			},
			"uncertain": map[string]any{"type": "boolean"},
			"questions": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
		"required": []string{"slots", "uncertain", "questions"},
	}
}

type availabilityParseResult struct {
	Slots []struct {
		Start      string `json:"start"`
		End        string `json:"end"`
		Preference int    `json:"preference"`
		Note       string `json:"note,omitempty"`
	} `json:"slots"`
	Uncertain bool     `json:"uncertain"`
	Questions []string `json:"questions"`
}

// ParseAvailability turns a free-text message into structured slots for the week
// starting at weekStart, interpreted in loc.
func (s *Service) ParseAvailability(ctx context.Context, text string, weekStart time.Time, loc *time.Location) ([]AvailabilitySlot, error) {
	if loc == nil {
		loc = time.UTC
	}
	weekEnd := weekStart.AddDate(0, 0, 7)
	prompt := fmt.Sprintf(`Target week starts %s and ends before %s.
Shop timezone: %s

Employee message:
%s`,
		weekStart.In(loc).Format(time.RFC3339),
		weekEnd.In(loc).Format(time.RFC3339),
		loc.String(),
		text,
	)

	raw, err := s.provider.Complete(ctx, Request{
		System:           parseAvailabilitySystem,
		Prompt:           prompt,
		MaxTokens:        1200,
		Temperature:      0,
		ResponseMIMEType: "application/json",
		ResponseSchema:   availabilityResponseSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("llm: parse availability: %w", err)
	}

	var parsed availabilityParseResult
	if err := json.Unmarshal([]byte(extractJSON(raw)), &parsed); err != nil {
		return nil, fmt.Errorf("llm: availability response is not valid JSON: %w", err)
	}
	if parsed.Uncertain {
		questions := parsed.Questions
		if len(questions) == 0 {
			questions = []string{"Could you clarify your availability with specific days and times?"}
		}
		return nil, &ClarificationError{Questions: questions}
	}

	slots := make([]AvailabilitySlot, 0, len(parsed.Slots))
	for i, slot := range parsed.Slots {
		start, err := time.Parse(time.RFC3339, slot.Start)
		if err != nil {
			return nil, fmt.Errorf("llm: availability slot %d has invalid start: %w", i, err)
		}
		end, err := time.Parse(time.RFC3339, slot.End)
		if err != nil {
			return nil, fmt.Errorf("llm: availability slot %d has invalid end: %w", i, err)
		}
		if !end.After(start) {
			return nil, fmt.Errorf("llm: availability slot %d has end before start", i)
		}
		slots = append(slots, AvailabilitySlot{
			Start:      start,
			End:        end,
			Preference: slot.Preference,
			Note:       slot.Note,
		})
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

// TranslateRule turns owner text into a RuleSpec the caller can map onto solver penalty rules.
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

// FormatAnnouncement renders a finalized schedule summary into a human-friendly message.
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

// extractJSON strips code fences and any prose around the first JSON value in a model response.
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
