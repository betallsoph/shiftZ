// Package solver implements shiftbot's hand-written shift scheduler:
// greedy initial assignment followed by simulated-annealing local search.
//
// Hard constraints (no double-booking, per-shift capacity, max hours per
// employee) are enforced at assignment time via CanAssign, so every schedule
// the solver produces satisfies them by construction. Soft preferences
// (fairness of hours, preference satisfaction, custom penalty rules) are
// expressed through a pluggable scoring function; see Scorer and PenaltyRule.
//
// The package is self-contained: it must not import telegram, llm, store or
// any other shiftbot package. Callers translate their domain models into
// Problem and read the resulting Schedule back out.
package solver

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// EmployeeID identifies an employee within a Problem. Callers typically use
// their database primary key rendered as a string.
type EmployeeID string

// ShiftID identifies a shift within a Problem.
type ShiftID string

// Preference expresses how much an employee wants a given shift.
type Preference int

const (
	// Unavailable means the employee cannot work the shift (hard constraint).
	Unavailable Preference = 0
	// Available means the employee can work the shift.
	Available Preference = 1
	// Preferred means the employee would like to work the shift.
	Preferred Preference = 2
)

// Employee is a schedulable worker.
type Employee struct {
	ID   EmployeeID
	Name string
	// MaxHours is the maximum number of working hours for the scheduling
	// horizon (typically one week). Zero means unlimited.
	MaxHours float64
}

// Shift is a slot of work that needs staffing.
type Shift struct {
	ID    ShiftID
	Role  string
	Start time.Time
	End   time.Time
	// MinStaff is the minimum head count required (hard constraint; if the
	// problem is infeasible the shortfall is penalized via Weights.Coverage).
	MinStaff int
	// MaxStaff is the capacity of the shift. Zero defaults to MinStaff.
	MaxStaff int
}

// Hours returns the shift duration in hours.
func (s Shift) Hours() float64 { return s.End.Sub(s.Start).Hours() }

// Overlaps reports whether two shifts overlap in time.
func (s Shift) Overlaps(o Shift) bool {
	return s.Start.Before(o.End) && o.Start.Before(s.End)
}

func (s Shift) capacity() int {
	if s.MaxStaff <= 0 {
		return s.MinStaff
	}
	return s.MaxStaff
}

// Problem is a self-contained scheduling instance for one shop and one
// scheduling horizon (typically a week).
type Problem struct {
	Employees []Employee
	Shifts    []Shift
	// Availability maps employee -> shift -> preference. A missing entry
	// means Unavailable.
	Availability map[EmployeeID]map[ShiftID]Preference

	employees map[EmployeeID]*Employee
	shifts    map[ShiftID]*Shift
}

// Validate checks the problem for structural errors and builds internal
// lookup tables. It must be called (directly or via GenerateCandidates)
// before Greedy or Anneal.
func (p *Problem) Validate() error {
	p.employees = make(map[EmployeeID]*Employee, len(p.Employees))
	for i := range p.Employees {
		e := &p.Employees[i]
		if e.ID == "" {
			return fmt.Errorf("solver: employee %d has empty ID", i)
		}
		if _, dup := p.employees[e.ID]; dup {
			return fmt.Errorf("solver: duplicate employee ID %q", e.ID)
		}
		if e.MaxHours < 0 {
			return fmt.Errorf("solver: employee %q has negative MaxHours", e.ID)
		}
		p.employees[e.ID] = e
	}
	p.shifts = make(map[ShiftID]*Shift, len(p.Shifts))
	for i := range p.Shifts {
		s := &p.Shifts[i]
		if s.ID == "" {
			return fmt.Errorf("solver: shift %d has empty ID", i)
		}
		if _, dup := p.shifts[s.ID]; dup {
			return fmt.Errorf("solver: duplicate shift ID %q", s.ID)
		}
		if !s.End.After(s.Start) {
			return fmt.Errorf("solver: shift %q ends before it starts", s.ID)
		}
		if s.MinStaff < 0 || (s.MaxStaff != 0 && s.MaxStaff < s.MinStaff) {
			return fmt.Errorf("solver: shift %q has invalid staffing bounds", s.ID)
		}
		p.shifts[s.ID] = s
	}
	return nil
}

func (p *Problem) employee(id EmployeeID) *Employee { return p.employees[id] }
func (p *Problem) shift(id ShiftID) *Shift          { return p.shifts[id] }

// Preference returns the employee's preference for a shift; missing entries
// are Unavailable.
func (p *Problem) Preference(e EmployeeID, s ShiftID) Preference {
	if p.Availability == nil {
		return Unavailable
	}
	return p.Availability[e][s]
}

// Schedule is an assignment of employees to shifts.
type Schedule struct {
	// Assignments maps shift -> assigned employees.
	Assignments map[ShiftID][]EmployeeID
}

// NewSchedule returns an empty schedule.
func NewSchedule() *Schedule {
	return &Schedule{Assignments: make(map[ShiftID][]EmployeeID)}
}

// Clone returns a deep copy of the schedule.
func (s *Schedule) Clone() *Schedule {
	c := &Schedule{Assignments: make(map[ShiftID][]EmployeeID, len(s.Assignments))}
	for shift, emps := range s.Assignments {
		c.Assignments[shift] = append([]EmployeeID(nil), emps...)
	}
	return c
}

// Assigned reports whether the employee works the shift.
func (s *Schedule) Assigned(shift ShiftID, emp EmployeeID) bool {
	for _, e := range s.Assignments[shift] {
		if e == emp {
			return true
		}
	}
	return false
}

func (s *Schedule) assign(shift ShiftID, emp EmployeeID) {
	s.Assignments[shift] = append(s.Assignments[shift], emp)
}

func (s *Schedule) unassign(shift ShiftID, emp EmployeeID) {
	emps := s.Assignments[shift]
	for i, e := range emps {
		if e == emp {
			s.Assignments[shift] = append(emps[:i:i], emps[i+1:]...)
			return
		}
	}
}

// HoursFor returns the total hours the employee works in this schedule.
func (s *Schedule) HoursFor(p *Problem, emp EmployeeID) float64 {
	var h float64
	for shiftID, emps := range s.Assignments {
		for _, e := range emps {
			if e == emp {
				if sh := p.shift(shiftID); sh != nil {
					h += sh.Hours()
				}
			}
		}
	}
	return h
}

// Fingerprint returns a canonical string identifying the assignment set,
// used to deduplicate candidate schedules.
func (s *Schedule) Fingerprint() string {
	shiftIDs := make([]string, 0, len(s.Assignments))
	for id := range s.Assignments {
		shiftIDs = append(shiftIDs, string(id))
	}
	sort.Strings(shiftIDs)
	var b strings.Builder
	for _, id := range shiftIDs {
		emps := s.Assignments[ShiftID(id)]
		if len(emps) == 0 {
			continue
		}
		names := make([]string, len(emps))
		for i, e := range emps {
			names[i] = string(e)
		}
		sort.Strings(names)
		b.WriteString(id)
		b.WriteByte('=')
		b.WriteString(strings.Join(names, ","))
		b.WriteByte(';')
	}
	return b.String()
}
