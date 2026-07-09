package solver

import "fmt"

// CanAssign reports whether assigning emp to shift keeps every hard
// constraint satisfied given the current schedule:
//
//   - the employee is available for the shift,
//   - the shift has spare capacity (MaxStaff),
//   - the employee is not already on the shift,
//   - the employee is not double-booked on an overlapping shift,
//   - the employee's MaxHours would not be exceeded.
//
// All moves the solver makes go through this check, so hard constraints hold
// by construction.
func CanAssign(p *Problem, s *Schedule, emp EmployeeID, shiftID ShiftID) bool {
	sh := p.shift(shiftID)
	empl := p.employee(emp)
	if sh == nil || empl == nil {
		return false
	}
	if p.Preference(emp, shiftID) == Unavailable {
		return false
	}
	assigned := s.Assignments[shiftID]
	if len(assigned) >= sh.capacity() {
		return false
	}
	for _, e := range assigned {
		if e == emp {
			return false
		}
	}
	if empl.MaxHours > 0 && s.HoursFor(p, emp)+sh.Hours() > empl.MaxHours+1e-9 {
		return false
	}
	for otherID, emps := range s.Assignments {
		if otherID == shiftID {
			continue
		}
		other := p.shift(otherID)
		if other == nil || !other.Overlaps(*sh) {
			continue
		}
		for _, e := range emps {
			if e == emp {
				return false
			}
		}
	}
	return true
}

// Violation describes a broken hard constraint in a schedule.
type Violation struct {
	Shift    ShiftID
	Employee EmployeeID // empty for shift-level violations such as understaffing
	Message  string
}

func (v Violation) String() string {
	if v.Employee == "" {
		return fmt.Sprintf("shift %s: %s", v.Shift, v.Message)
	}
	return fmt.Sprintf("shift %s / employee %s: %s", v.Shift, v.Employee, v.Message)
}

// Validate performs a full hard-constraint audit of a schedule. The solver
// keeps schedules valid by construction; this is a safety net for schedules
// coming from external sources (manual edits, persistence) and for tests.
func Validate(p *Problem, s *Schedule) []Violation {
	var out []Violation
	for i := range p.Shifts {
		sh := &p.Shifts[i]
		assigned := s.Assignments[sh.ID]
		if len(assigned) < sh.MinStaff {
			out = append(out, Violation{Shift: sh.ID, Message: fmt.Sprintf("understaffed: %d/%d", len(assigned), sh.MinStaff)})
		}
		if len(assigned) > sh.capacity() {
			out = append(out, Violation{Shift: sh.ID, Message: fmt.Sprintf("overstaffed: %d/%d", len(assigned), sh.capacity())})
		}
		seen := map[EmployeeID]bool{}
		for _, e := range assigned {
			if seen[e] {
				out = append(out, Violation{Shift: sh.ID, Employee: e, Message: "assigned twice to the same shift"})
			}
			seen[e] = true
			if p.employee(e) == nil {
				out = append(out, Violation{Shift: sh.ID, Employee: e, Message: "unknown employee"})
				continue
			}
			if p.Preference(e, sh.ID) == Unavailable {
				out = append(out, Violation{Shift: sh.ID, Employee: e, Message: "employee is unavailable"})
			}
		}
	}
	// Double-booking across overlapping shifts.
	for i := range p.Shifts {
		for j := i + 1; j < len(p.Shifts); j++ {
			a, b := &p.Shifts[i], &p.Shifts[j]
			if !a.Overlaps(*b) {
				continue
			}
			for _, e := range s.Assignments[a.ID] {
				if s.Assigned(b.ID, e) {
					out = append(out, Violation{Shift: b.ID, Employee: e, Message: fmt.Sprintf("double-booked with overlapping shift %s", a.ID)})
				}
			}
		}
	}
	// Max hours.
	for i := range p.Employees {
		e := &p.Employees[i]
		if e.MaxHours > 0 && s.HoursFor(p, e.ID) > e.MaxHours+1e-9 {
			out = append(out, Violation{Employee: e.ID, Message: fmt.Sprintf("exceeds max hours (%.1f > %.1f)", s.HoursFor(p, e.ID), e.MaxHours)})
		}
	}
	return out
}
