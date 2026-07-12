package planner

import (
	"fmt"
	"strings"

	"github.com/betallsoph/shiftz/internal/solver"
	"github.com/betallsoph/shiftz/internal/store"
)

func mapRules(employees []*store.Employee, rules []*store.Rule) ([]solver.PenaltyRule, []string) {
	byName := make(map[string]solver.EmployeeID, len(employees))
	for _, e := range employees {
		byName[strings.ToLower(strings.TrimSpace(e.DisplayName))] = solver.EmployeeID(e.ID.String())
	}

	var out []solver.PenaltyRule
	var warnings []string
	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}
		if rule.RuleJSON == nil {
			warnings = append(warnings, fmt.Sprintf("rule %s: missing rule_json", rule.ID))
			continue
		}
		kind, _ := rule.RuleJSON["kind"].(string)
		params, _ := rule.RuleJSON["params"].(map[string]any)
		if kind == "" || params == nil {
			warnings = append(warnings, fmt.Sprintf("rule %s: malformed rule_json", rule.ID))
			continue
		}
		weight := ruleWeight(rule)

		switch kind {
		case "avoid_pair":
			a, _ := params["a"].(string)
			b, _ := params["b"].(string)
			empA, okA := byName[strings.ToLower(strings.TrimSpace(a))]
			empB, okB := byName[strings.ToLower(strings.TrimSpace(b))]
			if !okA || !okB {
				warnings = append(warnings, fmt.Sprintf("rule %s: unknown employee in avoid_pair (%q, %q)", rule.ID, a, b))
				continue
			}
			out = append(out, solver.AvoidPairRule{A: empA, B: empB, Weight: weight})
		case "day_off":
			name, _ := params["employee"].(string)
			weekday, ok := intParam(params["weekday"])
			if !ok || weekday < 0 || weekday > 6 {
				warnings = append(warnings, fmt.Sprintf("rule %s: invalid day_off weekday", rule.ID))
				continue
			}
			emp, ok := byName[strings.ToLower(strings.TrimSpace(name))]
			if !ok {
				warnings = append(warnings, fmt.Sprintf("rule %s: unknown employee %q in day_off", rule.ID, name))
				continue
			}
			out = append(out, solver.DayOffRule{Employee: emp, Weekday: weekday, Weight: weight})
		default:
			warnings = append(warnings, fmt.Sprintf("rule %s: unsupported kind %q", rule.ID, kind))
		}
	}
	return out, warnings
}

func ruleWeight(r *store.Rule) float64 {
	if r.Weight > 0 {
		return r.Weight
	}
	if w, ok := floatParam(r.RuleJSON["weight"]); ok && w > 0 {
		return w
	}
	return 1
}

func intParam(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func floatParam(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
