package solver

import (
	"math/rand/v2"
	"sort"
)

// Greedy builds an initial feasible schedule. It fills the most constrained
// shifts first (fewest eligible candidates relative to required staff) and
// picks, for each open slot, the eligible employee with the strongest
// preference and the fewest hours assigned so far. Every assignment goes
// through CanAssign, so the result satisfies all hard constraints; shifts
// that cannot be filled are left understaffed and penalized by the scorer.
//
// Problem.Validate must have been called first.
func Greedy(p *Problem, rng *rand.Rand) *Schedule {
	s := NewSchedule()

	type shiftSlack struct {
		id    ShiftID
		slack int
	}
	order := make([]shiftSlack, 0, len(p.Shifts))
	for i := range p.Shifts {
		sh := &p.Shifts[i]
		candidates := 0
		for j := range p.Employees {
			if p.Preference(p.Employees[j].ID, sh.ID) != Unavailable {
				candidates++
			}
		}
		order = append(order, shiftSlack{id: sh.ID, slack: candidates - sh.MinStaff})
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].slack != order[j].slack {
			return order[i].slack < order[j].slack
		}
		return p.shift(order[i].id).Start.Before(p.shift(order[j].id).Start)
	})

	for _, o := range order {
		sh := p.shift(o.id)
		for len(s.Assignments[sh.ID]) < sh.MinStaff {
			best := pickCandidate(p, s, sh.ID, rng)
			if best == "" {
				break // infeasible; leave understaffed, scorer penalizes it
			}
			s.assign(sh.ID, best)
		}
	}
	return s
}

// pickCandidate returns the best eligible employee for a shift, preferring
// higher preference, then fewer assigned hours, with a small random
// tie-break so different seeds explore different schedules.
func pickCandidate(p *Problem, s *Schedule, shiftID ShiftID, rng *rand.Rand) EmployeeID {
	var best EmployeeID
	bestKey := [3]float64{}
	for i := range p.Employees {
		e := p.Employees[i].ID
		if !CanAssign(p, s, e, shiftID) {
			continue
		}
		key := [3]float64{
			float64(p.Preference(e, shiftID)),
			-s.HoursFor(p, e),
			rng.Float64(),
		}
		if best == "" || keyGreater(key, bestKey) {
			best, bestKey = e, key
		}
	}
	return best
}

func keyGreater(a, b [3]float64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}
