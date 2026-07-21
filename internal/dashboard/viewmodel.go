package dashboard

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/planner"
	"github.com/betallsoph/shiftz/internal/store"
)

const dateLayout = "2006-01-02"

// PageData is the main dashboard shell.
type PageData struct {
	Today                 string
	ShopName              string
	Shifts                ShiftsPanelView
	Employees             EmployeesPanelView
	Telegram              TelegramPanelView
	IncidentReportEnabled bool
}

// WeekView is the HTMX-swapped week panel.
type WeekView struct {
	ShopName       string
	WeekStart      string
	Notice         string
	Error          string
	Warnings       []string
	Schedules      []ScheduleView
	HasApproved    bool
	Availability   []AvailabilityEmployeeView
	SubmittedCount int
	EmployeeCount  int
}

// AvailabilityEmployeeView is one employee's weekly availability status.
type AvailabilityEmployeeView struct {
	EmployeeID string
	Name       string
	Role       string
	Submitted  bool
	Status     string
	RawMessage string
	Slots      []AvailabilitySlotView
}

// AvailabilitySlotView is one parsed availability span for display.
type AvailabilitySlotView struct {
	Date       string
	DayLabel   string
	TimeRange  string
	Preference string
	Note       string
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
	employees []*store.Employee,
	availabilities []*store.Availability,
	warnings []string,
	notice string,
	generated *planner.GenerateResult,
) WeekView {
	loc, err := time.LoadLocation(shop.Timezone)
	if err != nil {
		loc = time.UTC
	}
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

	availabilityViews, submittedCount, employeeCount := buildAvailabilityEmployeeViews(employees, availabilities, loc)

	return WeekView{
		ShopName:       shop.Name,
		WeekStart:      weekStart.Format(dateLayout),
		Notice:         notice,
		Warnings:       warnings,
		Schedules:      views,
		HasApproved:    hasApproved,
		Availability:   availabilityViews,
		SubmittedCount: submittedCount,
		EmployeeCount:  employeeCount,
	}
}

func buildAvailabilityEmployeeViews(
	employees []*store.Employee,
	availabilities []*store.Availability,
	loc *time.Location,
) ([]AvailabilityEmployeeView, int, int) {
	if loc == nil {
		loc = time.UTC
	}
	byEmployee := make(map[uuid.UUID]*store.Availability, len(availabilities))
	for _, row := range availabilities {
		byEmployee[row.EmployeeID] = row
	}

	views := make([]AvailabilityEmployeeView, len(employees))
	submitted := 0
	for i, emp := range employees {
		view := AvailabilityEmployeeView{
			EmployeeID: emp.ID.String(),
			Name:       emp.DisplayName,
			Role:       emp.Role,
			Status:     "chưa gửi",
		}
		if row, ok := byEmployee[emp.ID]; ok {
			view.Submitted = true
			view.Status = "đã gửi"
			view.RawMessage = row.RawMessage
			view.Slots = buildAvailabilitySlotViews(row.Slots, loc)
			submitted++
		}
		views[i] = view
	}
	return views, submitted, len(employees)
}

func buildAvailabilitySlotViews(slots []store.AvailabilitySlot, loc *time.Location) []AvailabilitySlotView {
	if len(slots) == 0 {
		return nil
	}
	sorted := append([]store.AvailabilitySlot(nil), slots...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Start.Before(sorted[j].Start)
	})

	views := make([]AvailabilitySlotView, len(sorted))
	for i, slot := range sorted {
		start := slot.Start.In(loc)
		end := slot.End.In(loc)
		views[i] = AvailabilitySlotView{
			Date:       start.Format(dateLayout),
			DayLabel:   start.Format("Mon"),
			TimeRange:  fmt.Sprintf("%s-%s", start.Format("15:04"), end.Format("15:04")),
			Preference: availabilityPreferenceLabel(slot.Preference),
			Note:       slot.Note,
		}
	}
	return views
}

func availabilityPreferenceLabel(pref int) string {
	switch pref {
	case 0:
		return "không có"
	case 2:
		return "ưu tiên"
	default:
		return "có thể"
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
