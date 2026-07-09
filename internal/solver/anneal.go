package solver

import (
	"math"
	"math/rand/v2"
)

// AnnealConfig controls the simulated-annealing local search.
type AnnealConfig struct {
	Iterations int
	StartTemp  float64
	EndTemp    float64
}

// DefaultAnnealConfig is tuned for weekly schedules of small shops
// (tens of shifts, up to a few dozen employees).
func DefaultAnnealConfig() AnnealConfig {
	return AnnealConfig{Iterations: 20000, StartTemp: 4.0, EndTemp: 0.05}
}

// Anneal improves a schedule with simulated annealing. Each step applies one
// random move (add / drop / replace / swap); every move preserves hard
// constraints via CanAssign, and worsening moves are accepted with a
// probability that decays with the temperature. The best schedule seen is
// returned, so the result never scores below the starting point.
func Anneal(p *Problem, start *Schedule, sc Scorer, cfg AnnealConfig, rng *rand.Rand) *Schedule {
	cur := start.Clone()
	curScore := sc.Score(p, cur)
	best := cur.Clone()
	bestScore := curScore

	if cfg.Iterations <= 0 {
		return best
	}
	for i := 0; i < cfg.Iterations; i++ {
		frac := float64(i) / float64(cfg.Iterations)
		temp := cfg.StartTemp * math.Pow(cfg.EndTemp/cfg.StartTemp, frac)

		cand := neighbor(p, cur, rng)
		if cand == nil {
			continue
		}
		candScore := sc.Score(p, cand)
		if candScore >= curScore || rng.Float64() < math.Exp((candScore-curScore)/temp) {
			cur, curScore = cand, candScore
			if candScore > bestScore {
				best, bestScore = cand.Clone(), candScore
			}
		}
	}
	return best
}

// neighbor returns a random valid one-move variation of s, or nil if no move
// applies.
func neighbor(p *Problem, s *Schedule, rng *rand.Rand) *Schedule {
	moves := []func(*Problem, *Schedule, *rand.Rand) *Schedule{
		moveAdd, moveDrop, moveReplace, moveSwap,
	}
	for _, i := range rng.Perm(len(moves)) {
		if next := moves[i](p, s, rng); next != nil {
			return next
		}
	}
	return nil
}

// moveAdd assigns a random eligible employee to a random shift with spare
// capacity.
func moveAdd(p *Problem, s *Schedule, rng *rand.Rand) *Schedule {
	var open []ShiftID
	for i := range p.Shifts {
		sh := &p.Shifts[i]
		if len(s.Assignments[sh.ID]) < sh.capacity() {
			open = append(open, sh.ID)
		}
	}
	if len(open) == 0 {
		return nil
	}
	shiftID := open[rng.IntN(len(open))]
	var eligible []EmployeeID
	for i := range p.Employees {
		if CanAssign(p, s, p.Employees[i].ID, shiftID) {
			eligible = append(eligible, p.Employees[i].ID)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	next := s.Clone()
	next.assign(shiftID, eligible[rng.IntN(len(eligible))])
	return next
}

// moveDrop removes a random employee from a shift staffed above MinStaff,
// so the move never breaks the coverage hard constraint.
func moveDrop(p *Problem, s *Schedule, rng *rand.Rand) *Schedule {
	var droppable []ShiftID
	for i := range p.Shifts {
		sh := &p.Shifts[i]
		if len(s.Assignments[sh.ID]) > sh.MinStaff {
			droppable = append(droppable, sh.ID)
		}
	}
	if len(droppable) == 0 {
		return nil
	}
	shiftID := droppable[rng.IntN(len(droppable))]
	emps := s.Assignments[shiftID]
	next := s.Clone()
	next.unassign(shiftID, emps[rng.IntN(len(emps))])
	return next
}

// moveReplace swaps one assigned employee on a shift for a different
// eligible employee, keeping head count unchanged.
func moveReplace(p *Problem, s *Schedule, rng *rand.Rand) *Schedule {
	shiftID, emp := randomAssignment(p, s, rng)
	if shiftID == "" {
		return nil
	}
	next := s.Clone()
	next.unassign(shiftID, emp)
	var eligible []EmployeeID
	for i := range p.Employees {
		e := p.Employees[i].ID
		if e != emp && CanAssign(p, next, e, shiftID) {
			eligible = append(eligible, e)
		}
	}
	if len(eligible) == 0 {
		return nil
	}
	next.assign(shiftID, eligible[rng.IntN(len(eligible))])
	return next
}

// moveSwap exchanges the employees of two assignments on different shifts.
func moveSwap(p *Problem, s *Schedule, rng *rand.Rand) *Schedule {
	s1, e1 := randomAssignment(p, s, rng)
	s2, e2 := randomAssignment(p, s, rng)
	if s1 == "" || s2 == "" || s1 == s2 || e1 == e2 {
		return nil
	}
	next := s.Clone()
	next.unassign(s1, e1)
	next.unassign(s2, e2)
	if !CanAssign(p, next, e2, s1) {
		return nil
	}
	next.assign(s1, e2)
	if !CanAssign(p, next, e1, s2) {
		return nil
	}
	next.assign(s2, e1)
	return next
}

func randomAssignment(p *Problem, s *Schedule, rng *rand.Rand) (ShiftID, EmployeeID) {
	type pair struct {
		shift ShiftID
		emp   EmployeeID
	}
	var pairs []pair
	for i := range p.Shifts {
		id := p.Shifts[i].ID
		for _, e := range s.Assignments[id] {
			pairs = append(pairs, pair{id, e})
		}
	}
	if len(pairs) == 0 {
		return "", ""
	}
	pick := pairs[rng.IntN(len(pairs))]
	return pick.shift, pick.emp
}
