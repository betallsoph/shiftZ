package solver

import (
	"math/rand/v2"
	"testing"
	"time"
)

// sampleProblem builds a feasible week: 4 employees, morning + evening shift
// every day, MinStaff 1 / MaxStaff 2, everyone available, scattered
// preferences.
func sampleProblem(t *testing.T) *Problem {
	t.Helper()
	monday := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	p := &Problem{
		Employees: []Employee{
			{ID: "anna", Name: "Anna", MaxHours: 40},
			{ID: "bob", Name: "Bob", MaxHours: 40},
			{ID: "chi", Name: "Chi", MaxHours: 32},
			{ID: "dave", Name: "Dave", MaxHours: 40},
		},
		Availability: map[EmployeeID]map[ShiftID]Preference{},
	}
	for day := 0; day < 7; day++ {
		date := monday.AddDate(0, 0, day)
		morning := Shift{
			ID:       ShiftID(date.Format("2006-01-02") + "-am"),
			Role:     "floor",
			Start:    date.Add(8 * time.Hour),
			End:      date.Add(14 * time.Hour),
			MinStaff: 1, MaxStaff: 2,
		}
		evening := Shift{
			ID:       ShiftID(date.Format("2006-01-02") + "-pm"),
			Role:     "floor",
			Start:    date.Add(14 * time.Hour),
			End:      date.Add(20 * time.Hour),
			MinStaff: 1, MaxStaff: 2,
		}
		p.Shifts = append(p.Shifts, morning, evening)
	}
	for _, e := range p.Employees {
		p.Availability[e.ID] = map[ShiftID]Preference{}
		for i := range p.Shifts {
			p.Availability[e.ID][p.Shifts[i].ID] = Available
		}
	}
	// Anna prefers mornings, Bob prefers evenings.
	for i := range p.Shifts {
		id := p.Shifts[i].ID
		if p.Shifts[i].Start.Hour() < 12 {
			p.Availability["anna"][id] = Preferred
		} else {
			p.Availability["bob"][id] = Preferred
		}
	}
	return p
}

func TestGenerateCandidatesFeasibleAndValid(t *testing.T) {
	p := sampleProblem(t)
	cands, err := GenerateCandidates(p, nil, 3, 42)
	if err != nil {
		t.Fatalf("GenerateCandidates: %v", err)
	}
	if len(cands) != 3 {
		t.Fatalf("got %d candidates, want 3", len(cands))
	}
	for _, c := range cands {
		if v := Validate(p, c.Schedule); len(v) != 0 {
			t.Errorf("candidate %q has hard-constraint violations: %v", c.Label, v)
		}
	}
	// At least two candidates should differ (three presets on a loose problem).
	fps := map[string]bool{}
	for _, c := range cands {
		fps[c.Schedule.Fingerprint()] = true
	}
	if len(fps) < 2 {
		t.Errorf("expected at least 2 distinct candidates, got %d", len(fps))
	}
}

func TestAnnealNeverWorsensScore(t *testing.T) {
	p := sampleProblem(t)
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	sc := Scorer{Weights: DefaultWeights()}
	rng := rand.New(rand.NewPCG(7, 7))
	start := Greedy(p, rng)
	startScore := sc.Score(p, start)
	improved := Anneal(p, start, sc, DefaultAnnealConfig(), rng)
	if got := sc.Score(p, improved); got < startScore {
		t.Errorf("anneal worsened score: %.2f -> %.2f", startScore, got)
	}
	if v := Validate(p, improved); len(v) != 0 {
		t.Errorf("annealed schedule violates hard constraints: %v", v)
	}
}

func TestCanAssignRejectsDoubleBooking(t *testing.T) {
	base := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	p := &Problem{
		Employees: []Employee{{ID: "anna", MaxHours: 40}},
		Shifts: []Shift{
			{ID: "s1", Start: base, End: base.Add(6 * time.Hour), MinStaff: 1},
			{ID: "s2", Start: base.Add(4 * time.Hour), End: base.Add(10 * time.Hour), MinStaff: 1},
			{ID: "s3", Start: base.Add(6 * time.Hour), End: base.Add(12 * time.Hour), MinStaff: 1},
		},
		Availability: map[EmployeeID]map[ShiftID]Preference{
			"anna": {"s1": Available, "s2": Available, "s3": Available},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	s := NewSchedule()
	s.assign("s1", "anna")
	if CanAssign(p, s, "anna", "s2") {
		t.Error("expected overlap with s1 to block assignment to s2")
	}
	if !CanAssign(p, s, "anna", "s3") {
		t.Error("expected back-to-back shift s3 to be assignable")
	}
}

func TestCanAssignRespectsMaxHours(t *testing.T) {
	base := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	p := &Problem{
		Employees: []Employee{{ID: "anna", MaxHours: 10}},
		Shifts: []Shift{
			{ID: "d1", Start: base, End: base.Add(8 * time.Hour), MinStaff: 1},
			{ID: "d2", Start: base.AddDate(0, 0, 1), End: base.AddDate(0, 0, 1).Add(8 * time.Hour), MinStaff: 1},
		},
		Availability: map[EmployeeID]map[ShiftID]Preference{
			"anna": {"d1": Available, "d2": Available},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	s := NewSchedule()
	s.assign("d1", "anna")
	if CanAssign(p, s, "anna", "d2") {
		t.Error("expected MaxHours to block a second 8h shift on a 10h cap")
	}
}

func TestCanAssignRespectsUnavailability(t *testing.T) {
	base := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	p := &Problem{
		Employees: []Employee{{ID: "anna"}},
		Shifts:    []Shift{{ID: "s1", Start: base, End: base.Add(6 * time.Hour), MinStaff: 1}},
		// no availability entry at all -> Unavailable
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if CanAssign(p, NewSchedule(), "anna", "s1") {
		t.Error("expected missing availability to block assignment")
	}
}

func TestPenaltyRules(t *testing.T) {
	p := sampleProblem(t)
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	s := NewSchedule()
	first := p.Shifts[0].ID
	s.assign(first, "anna")
	s.assign(first, "bob")

	pair := AvoidPairRule{A: "anna", B: "bob", Weight: 5}
	if got := pair.Penalty(p, s); got != 5 {
		t.Errorf("AvoidPairRule penalty = %v, want 5", got)
	}
	dayOff := DayOffRule{Employee: "anna", Weekday: int(p.Shifts[0].Start.Weekday()), Weight: 3}
	if got := dayOff.Penalty(p, s); got != 3 {
		t.Errorf("DayOffRule penalty = %v, want 3", got)
	}
}
