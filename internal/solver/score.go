package solver

import "math"

// PenaltyRule is a pluggable soft constraint. Rules return a non-negative
// penalty subtracted from the schedule score. Owner rules expressed in
// natural language are translated (by the llm package, outside this package)
// into parameters for concrete rule types or into RuleFunc closures.
type PenaltyRule interface {
	Name() string
	Penalty(p *Problem, s *Schedule) float64
}

// Weights tunes the built-in soft-constraint terms of the scoring function.
// Different weight presets are used to generate distinct schedule candidates
// for voting.
type Weights struct {
	// Preference is the reward per assignment on a Preferred shift.
	Preference float64
	// Fairness scales the penalty on the standard deviation of assigned
	// hours across employees.
	Fairness float64
	// Coverage is the penalty per missing head below a shift's MinStaff.
	// It should dominate the other terms so the search always prioritizes
	// filling shifts.
	Coverage float64
}

// DefaultWeights returns a balanced weight set.
func DefaultWeights() Weights {
	return Weights{Preference: 2, Fairness: 1, Coverage: 100}
}

// Scorer evaluates schedules. Higher scores are better.
type Scorer struct {
	Weights Weights
	Rules   []PenaltyRule
}

// Score computes the soft-constraint score of a schedule:
// preference satisfaction minus fairness spread, coverage shortfall and
// custom rule penalties.
func (sc Scorer) Score(p *Problem, s *Schedule) float64 {
	w := sc.Weights
	score := 0.0

	for shiftID, emps := range s.Assignments {
		for _, e := range emps {
			if p.Preference(e, shiftID) == Preferred {
				score += w.Preference
			}
		}
	}

	for i := range p.Shifts {
		sh := &p.Shifts[i]
		if missing := sh.MinStaff - len(s.Assignments[sh.ID]); missing > 0 {
			score -= w.Coverage * float64(missing)
		}
	}

	if w.Fairness > 0 && len(p.Employees) > 1 {
		score -= w.Fairness * hoursStddev(p, s)
	}

	for _, r := range sc.Rules {
		score -= r.Penalty(p, s)
	}
	return score
}

func hoursStddev(p *Problem, s *Schedule) float64 {
	n := float64(len(p.Employees))
	var sum float64
	hours := make([]float64, len(p.Employees))
	for i := range p.Employees {
		hours[i] = s.HoursFor(p, p.Employees[i].ID)
		sum += hours[i]
	}
	mean := sum / n
	var variance float64
	for _, h := range hours {
		variance += (h - mean) * (h - mean)
	}
	return math.Sqrt(variance / n)
}
