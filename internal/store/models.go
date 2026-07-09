package store

import (
	"time"

	"github.com/betallsoph/shiftz/internal/ent"
)

// Shop is a tenant: one restaurant or cafe.
type Shop struct {
	ID              int64
	Name            string
	Timezone        string
	InviteCode      string
	OwnerTelegramID int64
	CreatedAt       time.Time
}

// Employee is a staff member of one shop, linked to a Telegram account.
type Employee struct {
	ID              int64
	ShopID          int64
	TelegramUserID  int64
	DisplayName     string
	MaxHoursPerWeek float64
	Active          bool
	CreatedAt       time.Time
}

// AvailabilitySlot is one stored span of (un)availability for an employee.
// It mirrors the shape the llm package produces, but is defined here so the
// store stays independent of llm.
type AvailabilitySlot struct {
	Start      time.Time
	End        time.Time
	Preference int // 0 unavailable, 1 available, 2 preferred
	Note       string
}

// The repository API keeps these plain structs (rather than leaking ent
// types) so callers stay decoupled from the persistence library.

func shopFromEnt(m *ent.Shop) *Shop {
	return &Shop{
		ID:              int64(m.ID),
		Name:            m.Name,
		Timezone:        m.Timezone,
		InviteCode:      m.InviteCode,
		OwnerTelegramID: m.OwnerTelegramID,
		CreatedAt:       m.CreatedAt,
	}
}

func employeeFromEnt(m *ent.Employee) *Employee {
	return &Employee{
		ID:              int64(m.ID),
		ShopID:          int64(m.ShopID),
		TelegramUserID:  m.TelegramUserID,
		DisplayName:     m.DisplayName,
		MaxHoursPerWeek: m.MaxHoursPerWeek,
		Active:          m.Active,
		CreatedAt:       m.CreatedAt,
	}
}
