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
	TelegramGroupID int64
	Plan            string
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

// AvailabilitySlot is one parsed span of (un)availability inside a weekly
// submission. It mirrors the shape the llm package produces, but is defined
// here so the store stays independent of llm.
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
		ID:              m.ID,
		Name:            m.Name,
		Timezone:        m.Timezone,
		InviteCode:      m.InviteCode,
		TelegramGroupID: m.TelegramGroupID,
		Plan:            m.Plan,
		CreatedAt:       m.CreatedAt,
	}
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
