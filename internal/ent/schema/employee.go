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

// Employee is a staff member of one shop, linked to a Telegram account and
// joined via the shop's invite code.
type Employee struct {
	ent.Schema
}

func (Employee) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.Int64("telegram_user_id"),
		field.String("display_name"),
		// Job role, e.g. "barista", "kitchen"; matched against Shift.name
		// or used by custom rules later.
		field.String("role").Default(""),
		field.Float("max_hours_per_week").Default(40),
		field.Bool("is_active").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Employee) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("employees").Field("shop_id").Unique().Required(),
		edge.To("availabilities", Availability.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("assignments", ScheduleAssignment.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("votes", ScheduleVote.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (Employee) Indexes() []ent.Index {
	return []ent.Index{
		// One membership per Telegram account per shop.
		index.Fields("shop_id", "telegram_user_id").Unique(),
		// Roster queries: "all active employees of this shop"
		// (reminders, solver input).
		index.Fields("shop_id", "is_active"),
		// Webhook lookup: the bot resolves the sender by Telegram id
		// before it knows the shop.
		index.Fields("telegram_user_id"),
	}
}
