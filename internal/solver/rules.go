package solver

// RuleFunc adapts a plain function to the PenaltyRule interface. Custom
// rules generated at runtime (e.g. from owner text translated by an LLM)
// can be wired in through this type.
type RuleFunc struct {
	RuleName string
	Fn       func(p *Problem, s *Schedule) float64
}

func (r RuleFunc) Name() string                            { return r.RuleName }
func (r RuleFunc) Penalty(p *Problem, s *Schedule) float64 { return r.Fn(p, s) }

// AvoidPairRule penalizes every shift on which two given employees work
// together ("don't schedule Anna and Bob on the same shift").
type AvoidPairRule struct {
	A, B   EmployeeID
	Weight float64
}

func (r AvoidPairRule) Name() string { return "avoid_pair" }

func (r AvoidPairRule) Penalty(p *Problem, s *Schedule) float64 {
	var n float64
	for shiftID := range s.Assignments {
		if s.Assigned(shiftID, r.A) && s.Assigned(shiftID, r.B) {
			n++
		}
	}
	return n * r.Weight
}

// DayOffRule penalizes assignments of an employee on a given weekday
// ("Minh never works Sundays" as a soft preference; make the employee
// Unavailable instead if it must be a hard rule).
type DayOffRule struct {
	Employee EmployeeID
	Weekday  int // time.Weekday value: 0 = Sunday
	Weight   float64
}

func (r DayOffRule) Name() string { return "day_off" }

func (r DayOffRule) Penalty(p *Problem, s *Schedule) float64 {
	var n float64
	for shiftID := range s.Assignments {
		sh := p.shift(shiftID)
		if sh == nil || int(sh.Start.Weekday()) != r.Weekday {
			continue
		}
		if s.Assigned(shiftID, r.Employee) {
			n++
		}
	}
	return n * r.Weight
}
