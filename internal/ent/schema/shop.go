// Package schema holds shiftbot's ent entity definitions.
//
// MULTI-TENANCY: every entity except Shop has a required edge to Shop and
// stores shop_id as an explicit field so it can lead composite indexes.
// All uniqueness constraints are scoped by shop_id.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Shop is a tenant: one restaurant or cafe.
type Shop struct {
	ent.Schema
}

func (Shop) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("name"),
		field.String("timezone").Default("UTC"),
		field.String("invite_code").Unique(),
		// Broadcast Telegram group: schedules, votes, and shop-wide announcements.
		field.Int64("telegram_group_id"),
		// Optional internal team chat group (separate from the broadcast group).
		field.Int64("telegram_team_chat_id").Optional().Nillable(),
		// Linked owner Telegram user ID for bot commands and notifications.
		field.Int64("owner_telegram_id").Optional().Nillable(),
		// SHA-256 hex hash of a one-time owner Telegram link token.
		field.String("owner_link_token_hash").Optional().Nillable(),
		field.Time("owner_link_token_expires_at").Optional().Nillable(),
		// SaaS plan tier, e.g. "free", "pro".
		field.String("plan").Default("free"),
		// SHA-256 hex hash of the owner dashboard token (never store plaintext).
		field.String("dashboard_token_hash").Optional().Nillable(),
		// Lowercase owner dashboard login username; optional until provisioned.
		field.String("dashboard_username").Optional().Nillable(),
		// bcrypt hash of the owner dashboard password (never store plaintext).
		field.String("dashboard_password_hash").Optional().Nillable(),
		// Owner contact email for password recovery.
		field.String("dashboard_email").Optional().Nillable(),
		// Optional owner password hint shown only at first setup.
		field.String("dashboard_password_hint").Optional().Nillable(),
		// SHA-256 hex hash of a one-time password reset token.
		field.String("dashboard_password_reset_hash").Optional().Nillable(),
		field.Time("dashboard_password_reset_expires_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Shop) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("dashboard_username").Unique(),
		index.Fields("owner_telegram_id").Unique(),
	}
}

func (Shop) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("employees", Employee.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("shifts", Shift.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("schedules", Schedule.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("rules", Rule.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("availabilities", Availability.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("availability_drafts", AvailabilityDraft.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("reminder_deliveries", ReminderDelivery.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
