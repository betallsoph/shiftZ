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
		// The Telegram group chat the bot posts schedules and votes into.
		field.Int64("telegram_group_id"),
		// SHA-256 hex hash of a short-lived Telegram group setup code.
		field.String("telegram_setup_code_hash").Optional().Nillable(),
		field.Time("telegram_setup_code_expires_at").Optional().Nillable(),
		// SaaS plan tier, e.g. "free", "pro".
		field.String("plan").Default("free"),
		// SHA-256 hex hash of the owner dashboard token (never store plaintext).
		field.String("dashboard_token_hash").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Shop) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("telegram_setup_code_hash").Unique(),
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
