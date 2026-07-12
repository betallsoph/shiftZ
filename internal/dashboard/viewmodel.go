package dashboard

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

const dateLayout = "2006-01-02"

// PageData is the main dashboard shell.
type PageData struct {
	Today string
}

// WeekView is the HTMX-swapped week panel.
type WeekView struct {
	ShopID    string
	ShopName  string
	WeekStart string
	Notice    string
	Error     string
	Warnings  []string
	Schedules     []ScheduleView
	HasApproved   bool
}

// ScheduleView is one schedule candidate card.
type ScheduleView struct {
	ID              string
	VariantLabel    string
	Status          string
	StatusLabel     string
	Score           string
	AssignmentCount int
	IsApproved      bool
	Violations      []string
	Days            []DayView
}

// DayView groups shifts on one calendar date.
type DayView struct {
	Date   string
	Label  string
	Shifts []ShiftAssignmentView
}

// ShiftAssignmentView is one shift row with assigned employees.
type ShiftAssignmentView struct {
	Name      string
	TimeRange string
	Employees []string
}

func buildWeekView(
	shop *store.Shop,
	weekStart time.Time,
	schedules []*store.Schedule,
	warnings []string,
	notice string,
	generated *planner.GenerateResult,
) WeekView {
	violationsByID := map[string][]string{}
	if generated != nil {
		for _, c := range generated.Candidates {
			violationsByID[c.ID.String()] = c.Violations
		}
		if warnings == nil && len(generated.Warnings) > 0 {
			warnings = generated.Warnings
		}
	}

	views := make([]ScheduleView, len(schedules))
	hasApproved := false
	for i, sched := range schedules {
		if sched.Status == "approved" {
			hasApproved = true
		}
		violations := violationsByID[sched.ID.String()]
		if violations == nil {
			violations = []string{}
		}
		views[i] = ScheduleView{
			ID:              sched.ID.String(),
			VariantLabel:    sched.VariantLabel,
			Status:          sched.Status,
			StatusLabel:     statusLabel(sched.Status),
			Score:           formatScore(sched.Score),
			AssignmentCount: len(sched.Assignments),
			IsApproved:      sched.Status == "approved",
			Violations:      violations,
			Days:            groupAssignments(sched.Assignments, weekStart.Location()),
		}
	}

	if warnings == nil {
		warnings = []string{}
	}

	return WeekView{
		ShopID:      shop.ID.String(),
		ShopName:    shop.Name,
		WeekStart:   weekStart.Format(dateLayout),
		Notice:      notice,
		Warnings:    warnings,
		Schedules:   views,
		HasApproved: hasApproved,
	}
}

func groupAssignments(assignments []*store.ScheduleAssignment, loc *time.Location) []DayView {
	if len(assignments) == 0 {
		return nil
	}
	if loc == nil {
		loc = time.UTC
	}

	type shiftKey struct {
		date      string
		shiftID   string
		name      string
		startTime string
		endTime   string
	}
	byDay := make(map[string]map[shiftKey][]string)

	for _, a := range assignments {
		date := a.Date.In(loc)
		dateKey := date.Format(dateLayout)
		key := shiftKey{
			date:      dateKey,
			shiftID:   a.ShiftID.String(),
			name:      a.ShiftName,
			startTime: a.ShiftStartTime,
			endTime:   a.ShiftEndTime,
		}
		if byDay[dateKey] == nil {
			byDay[dateKey] = make(map[shiftKey][]string)
		}
		byDay[dateKey][key] = append(byDay[dateKey][key], a.EmployeeName)
	}

	dateKeys := make([]string, 0, len(byDay))
	for d := range byDay {
		dateKeys = append(dateKeys, d)
	}
	sort.Strings(dateKeys)

	days := make([]DayView, 0, len(dateKeys))
	for _, dateKey := range dateKeys {
		shiftMap := byDay[dateKey]
		keys := make([]shiftKey, 0, len(shiftMap))
		for k := range shiftMap {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].startTime != keys[j].startTime {
				return keys[i].startTime < keys[j].startTime
			}
			return keys[i].name < keys[j].name
		})

		shifts := make([]ShiftAssignmentView, len(keys))
		for i, k := range keys {
			names := append([]string(nil), shiftMap[k]...)
			sort.Strings(names)
			shifts[i] = ShiftAssignmentView{
				Name:      k.name,
				TimeRange: fmt.Sprintf("%s-%s", k.startTime, k.endTime),
				Employees: names,
			}
		}

		parsed, _ := time.ParseInLocation(dateLayout, dateKey, loc)
		days = append(days, DayView{
			Date:   dateKey,
			Label:  formatDayLabel(parsed),
			Shifts: shifts,
		})
	}
	return days
}

func formatScore(score float64) string {
	return strconv.FormatFloat(score, 'f', 1, 64)
}

func formatEmployees(names []string) string {
	return strings.Join(names, ", ")
}

var weekdayVI = []string{
	"Chủ nhật", "Thứ hai", "Thứ ba", "Thứ tư", "Thứ năm", "Thứ sáu", "Thứ bảy",
}

func formatDayLabel(t time.Time) string {
	return fmt.Sprintf("%s %s", weekdayVI[t.Weekday()], t.Format("02/01/2006"))
}

func statusLabel(status string) string {
	switch status {
	case "draft":
		return "nháp"
	case "approved":
		return "đã duyệt"
	case "voting":
		return "đang bỏ phiếu"
	case "published":
		return "đã phát hành"
	default:
		return status
	}
}
