package store

import (
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
)

// Shop is a tenant: one restaurant or cafe.
type Shop struct {
	ID              uuid.UUID
	Name            string
	Timezone        string
	InviteCode      string
	TelegramGroupID            int64
	TelegramSetupCodeExpiresAt *time.Time
	Plan                       string
	CreatedAt       time.Time
}

// Employee is a staff member of one shop, linked to a Telegram account.
type Employee struct {
	ID              uuid.UUID
	ShopID          uuid.UUID
	TelegramUserID  int64
	DisplayName     string
	Role            string
	MaxHoursPerWeek float64
	IsActive        bool
	CreatedAt       time.Time
}

// Shift is a weekly shift template: a named slot on one weekday with a
// time-of-day span and staffing bounds.
type Shift struct {
	ID        uuid.UUID
	ShopID    uuid.UUID
	Name      string
	Weekday   int
	StartTime string
	EndTime   string
	MinStaff  int
	MaxStaff  int
	IsActive  bool
}

// AvailabilitySlot is one parsed span of (un)availability inside a weekly
// submission. It mirrors the shape the llm package produces, but is defined
// here so the store stays independent of llm.
type AvailabilitySlot struct {
	Start      time.Time
	End        time.Time
	Preference int // 0 unavailable, 1 available, 2 preferred
	Note       string
}

// Availability is one employee's availability for one week.
type Availability struct {
	ID         uuid.UUID
	ShopID     uuid.UUID
	EmployeeID uuid.UUID
	WeekStart  time.Time
	Slots      []AvailabilitySlot
	RawMessage string
	CreatedAt  time.Time
}

// AvailabilityDraft is a pending Telegram availability confirmation.
type AvailabilityDraft struct {
	ID             uuid.UUID
	ShopID         uuid.UUID
	EmployeeID     uuid.UUID
	TelegramUserID int64
	ChatID         int64
	WeekStart      time.Time
	Timezone       string
	Slots          []AvailabilitySlot
	RawMessage     string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// ReminderDelivery is one queued or completed Telegram reminder/nag send.
type ReminderDelivery struct {
	ID         uuid.UUID
	ShopID     uuid.UUID
	EmployeeID uuid.UUID
	WeekStart  time.Time
	Kind       string
	Status     string
	Attempts   int
	LastError  string
	CreatedAt  time.Time
	SentAt     *time.Time
}

// Schedule is one solver-generated schedule variant for a week.
type Schedule struct {
	ID           uuid.UUID
	ShopID       uuid.UUID
	WeekStart    time.Time
	Status       string
	VariantLabel string
	Score        float64
	CreatedAt    time.Time
	Assignments  []*ScheduleAssignment
}

// ScheduleAssignment puts one employee on one shift template on one date.
type ScheduleAssignment struct {
	ID         uuid.UUID
	ShopID     uuid.UUID
	ScheduleID uuid.UUID
	ShiftID    uuid.UUID
	EmployeeID uuid.UUID
	Date       time.Time

	ShiftName      string
	ShiftWeekday   int
	ShiftStartTime string
	ShiftEndTime   string
	EmployeeName   string
}

// Rule is an owner scheduling rule consumed by the solver.
type Rule struct {
	ID          uuid.UUID
	ShopID      uuid.UUID
	Description string
	RuleJSON    map[string]any
	Weight      float64
	IsActive    bool
	CreatedAt   time.Time
}

// The repository API keeps these plain structs (rather than leaking ent
// types) so callers stay decoupled from the persistence library.

func shopFromEnt(m *ent.Shop) *Shop {
	s := &Shop{
		ID:              m.ID,
		Name:            m.Name,
		Timezone:        m.Timezone,
		InviteCode:      m.InviteCode,
		TelegramGroupID: m.TelegramGroupID,
		Plan:            m.Plan,
		CreatedAt:       m.CreatedAt,
	}
	if m.TelegramSetupCodeExpiresAt != nil {
		t := *m.TelegramSetupCodeExpiresAt
		s.TelegramSetupCodeExpiresAt = &t
	}
	return s
}

func employeeFromEnt(m *ent.Employee) *Employee {
	return &Employee{
		ID:              m.ID,
		ShopID:          m.ShopID,
		TelegramUserID:  m.TelegramUserID,
		DisplayName:     m.DisplayName,
		Role:            m.Role,
		MaxHoursPerWeek: m.MaxHoursPerWeek,
		IsActive:        m.IsActive,
		CreatedAt:       m.CreatedAt,
	}
}

func shiftFromEnt(m *ent.Shift) *Shift {
	return &Shift{
		ID:        m.ID,
		ShopID:    m.ShopID,
		Name:      m.Name,
		Weekday:   m.Weekday,
		StartTime: m.StartTime,
		EndTime:   m.EndTime,
		MinStaff:  m.MinStaff,
		MaxStaff:  m.MaxStaff,
		IsActive:  m.IsActive,
	}
}

func availabilityFromEnt(m *ent.Availability) *Availability {
	slots := make([]AvailabilitySlot, len(m.Slots))
	for i, s := range m.Slots {
		slots[i] = AvailabilitySlot{
			Start:      s.Start,
			End:        s.End,
			Preference: s.Preference,
			Note:       s.Note,
		}
	}
	return &Availability{
		ID:         m.ID,
		ShopID:     m.ShopID,
		EmployeeID: m.EmployeeID,
		WeekStart:  m.WeekStart,
		Slots:      slots,
		RawMessage: m.RawMessage,
		CreatedAt:  m.CreatedAt,
	}
}

func availabilityDraftFromEnt(m *ent.AvailabilityDraft) *AvailabilityDraft {
	slots := make([]AvailabilitySlot, len(m.Slots))
	for i, s := range m.Slots {
		slots[i] = AvailabilitySlot{
			Start:      s.Start,
			End:        s.End,
			Preference: s.Preference,
			Note:       s.Note,
		}
	}
	return &AvailabilityDraft{
		ID:             m.ID,
		ShopID:         m.ShopID,
		EmployeeID:     m.EmployeeID,
		TelegramUserID: m.TelegramUserID,
		ChatID:         m.ChatID,
		WeekStart:      m.WeekStart,
		Timezone:       m.Timezone,
		Slots:          slots,
		RawMessage:     m.RawMessage,
		CreatedAt:      m.CreatedAt,
		ExpiresAt:      m.ExpiresAt,
	}
}

func scheduleFromEnt(m *ent.Schedule) *Schedule {
	s := &Schedule{
		ID:           m.ID,
		ShopID:       m.ShopID,
		WeekStart:    m.WeekStart,
		Status:       string(m.Status),
		VariantLabel: m.VariantLabel,
		Score:        m.Score,
		CreatedAt:    m.CreatedAt,
	}
	if assignments := m.Edges.Assignments; len(assignments) > 0 {
		s.Assignments = make([]*ScheduleAssignment, len(assignments))
		for i, a := range assignments {
			s.Assignments[i] = scheduleAssignmentFromEnt(a)
		}
	}
	return s
}

func scheduleAssignmentFromEnt(m *ent.ScheduleAssignment) *ScheduleAssignment {
	a := &ScheduleAssignment{
		ID:         m.ID,
		ShopID:     m.ShopID,
		ScheduleID: m.ScheduleID,
		ShiftID:    m.ShiftID,
		EmployeeID: m.EmployeeID,
		Date:       m.Date,
	}
	if shift := m.Edges.Shift; shift != nil {
		a.ShiftName = shift.Name
		a.ShiftWeekday = shift.Weekday
		a.ShiftStartTime = shift.StartTime
		a.ShiftEndTime = shift.EndTime
	}
	if emp := m.Edges.Employee; emp != nil {
		a.EmployeeName = emp.DisplayName
	}
	return a
}

func ruleFromEnt(m *ent.Rule) *Rule {
	return &Rule{
		ID:          m.ID,
		ShopID:      m.ShopID,
		Description: m.Description,
		RuleJSON:    m.RuleJSON,
		Weight:      m.Weight,
		IsActive:    m.IsActive,
		CreatedAt:   m.CreatedAt,
	}
}
