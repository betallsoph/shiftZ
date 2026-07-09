package solver

import (
	"fmt"
	"math/rand/v2"
)

// Candidate is one proposed schedule offered to the team for voting.
type Candidate struct {
	Label    string
	Schedule *Schedule
	Score    float64
	Weights  Weights
}

// weight presets used to make candidates genuinely different, not just
// re-rolls of the same objective.
var candidatePresets = []struct {
	label string
	w     Weights
}{
	{"balanced", Weights{Preference: 2, Fairness: 1, Coverage: 100}},
	{"fairness-first", Weights{Preference: 1, Fairness: 3, Coverage: 100}},
	{"preference-first", Weights{Preference: 4, Fairness: 0.5, Coverage: 100}},
}

// GenerateCandidates produces n distinct schedule candidates (n is clamped
// to 2..3) by running greedy + annealing under different weight presets and
// seeds. rules apply to every candidate. Candidates are deduplicated by
// assignment fingerprint; if the search keeps converging to the same
// schedule, a duplicate may be returned rather than failing.
func GenerateCandidates(p *Problem, rules []PenaltyRule, n int, seed int64) ([]Candidate, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if len(p.Employees) == 0 || len(p.Shifts) == 0 {
		return nil, fmt.Errorf("solver: problem needs at least one employee and one shift")
	}
	if n < 2 {
		n = 2
	}
	if n > 3 {
		n = 3
	}

	seen := make(map[string]bool)
	out := make([]Candidate, 0, n)
	for i := 0; i < n; i++ {
		preset := candidatePresets[i%len(candidatePresets)]
		sc := Scorer{Weights: preset.w, Rules: rules}

		var sched *Schedule
		const maxAttempts = 4
		for attempt := 0; attempt < maxAttempts; attempt++ {
			rng := rand.New(rand.NewPCG(uint64(seed)+uint64(i*131+attempt*911), uint64(i+1)))
			sched = Anneal(p, Greedy(p, rng), sc, DefaultAnnealConfig(), rng)
			if !seen[sched.Fingerprint()] {
				break
			}
		}
		seen[sched.Fingerprint()] = true
		out = append(out, Candidate{
			Label:    preset.label,
			Schedule: sched,
			Score:    sc.Score(p, sched),
			Weights:  preset.w,
		})
	}
	return out, nil
}
