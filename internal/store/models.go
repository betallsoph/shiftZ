package store

import "time"

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
