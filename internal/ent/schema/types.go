package schema

import "time"

// AvailabilitySlot is one parsed span of (un)availability inside
// Availability.slots. Stored as JSON: the slot list is written and read as a
// unit (resubmitting a week replaces it wholesale), so relational modelling
// would buy nothing.
type AvailabilitySlot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	// Solver scale: 0 unavailable, 1 available, 2 preferred.
	Preference int    `json:"preference"`
	Note       string `json:"note,omitempty"`
}
