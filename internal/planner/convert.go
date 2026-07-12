package planner

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/solver"
	"github.com/betallsoph/shiftz/internal/store"
)

// ShiftOccurrence maps a solver shift ID back to DB persistence data.
type ShiftOccurrence struct {
	ShiftID uuid.UUID
	Date    time.Time
}

// BuildProblem converts store rows into a solver problem for one week.
func BuildProblem(
	weekStart time.Time,
	loc *time.Location,
	employees []*store.Employee,
	shifts []*store.Shift,
	availability []*store.Availability,
) (*solver.Problem, map[solver.ShiftID]ShiftOccurrence, error) {
	if loc == nil {
		loc = time.UTC
	}

	availByEmployee := make(map[uuid.UUID]*store.Availability, len(availability))
	for _, a := range availability {
		availByEmployee[a.EmployeeID] = a
	}

	problem := &solver.Problem{
		Employees:    make([]solver.Employee, 0, len(employees)),
		Shifts:       make([]solver.Shift, 0, len(shifts)),
		Availability: make(map[solver.EmployeeID]map[solver.ShiftID]solver.Preference),
	}
	occurrences := make(map[solver.ShiftID]ShiftOccurrence, len(shifts))

	for _, e := range employees {
		problem.Employees = append(problem.Employees, solver.Employee{
			ID:       solver.EmployeeID(e.ID.String()),
			Name:     e.DisplayName,
			MaxHours: e.MaxHoursPerWeek,
		})
	}

	for _, sh := range shifts {
		date := shiftDate(weekStart, sh.Weekday)
		start, err := clockOnDate(date, sh.StartTime, loc)
		if err != nil {
			return nil, nil, fmt.Errorf("planner: shift %s start time: %w", sh.ID, err)
		}
		end, err := clockOnDate(date, sh.EndTime, loc)
		if err != nil {
			return nil, nil, fmt.Errorf("planner: shift %s end time: %w", sh.ID, err)
		}
		if !end.After(start) {
			end = end.Add(24 * time.Hour)
		}

		shiftID := solver.ShiftID(sh.ID.String())
		problem.Shifts = append(problem.Shifts, solver.Shift{
			ID:       shiftID,
			Role:     sh.Name,
			Start:    start,
			End:      end,
			MinStaff: sh.MinStaff,
			MaxStaff: sh.MaxStaff,
		})
		occurrences[shiftID] = ShiftOccurrence{
			ShiftID: sh.ID,
			Date:    dateInLoc(date, loc),
		}
	}

	for _, e := range employees {
		empID := solver.EmployeeID(e.ID.String())
		prefs := make(map[solver.ShiftID]solver.Preference, len(problem.Shifts))
		var slots []store.AvailabilitySlot
		if row := availByEmployee[e.ID]; row != nil {
			slots = row.Slots
		}
		for _, sh := range problem.Shifts {
			pref, err := preferenceForShift(sh.Start, sh.End, slots)
			if err != nil {
				return nil, nil, fmt.Errorf("planner: employee %s shift %s: %w", e.ID, sh.ID, err)
			}
			prefs[sh.ID] = pref
		}
		problem.Availability[empID] = prefs
	}

	if err := problem.Validate(); err != nil {
		return nil, nil, fmt.Errorf("planner: build problem: %w", err)
	}
	return problem, occurrences, nil
}

func shiftDate(weekStart time.Time, weekday int) time.Time {
	offset := weekday - 1
	if weekday == 0 {
		offset = 6
	}
	return weekStart.AddDate(0, 0, offset)
}

func clockOnDate(date time.Time, hhmm string, loc *time.Location) (time.Time, error) {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		return time.Time{}, fmt.Errorf("parse %q: %w", hhmm, err)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, fmt.Errorf("invalid time %q", hhmm)
	}
	d := dateInLoc(date, loc)
	return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, loc), nil
}

func dateInLoc(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func preferenceForShift(shiftStart, shiftEnd time.Time, slots []store.AvailabilitySlot) (solver.Preference, error) {
	if len(slots) == 0 {
		return solver.Unavailable, nil
	}

	best := solver.Unavailable
	for _, slot := range slots {
		switch slot.Preference {
		case 0:
			if timesOverlap(slot.Start, slot.End, shiftStart, shiftEnd) {
				return solver.Unavailable, nil
			}
		case 1, 2:
			if !slot.Start.After(shiftStart) && !slot.End.Before(shiftEnd) {
				p := solver.Preference(slot.Preference)
				if p > best {
					best = p
				}
			}
		default:
			return solver.Unavailable, fmt.Errorf("invalid preference %d", slot.Preference)
		}
	}
	return best, nil
}

func timesOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}
