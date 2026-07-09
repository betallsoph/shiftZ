package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Employee is a staff member of one shop, linked to a Telegram account and
// joined via the shop's invite code.
type Employee struct {
	ent.Schema
}

func (Employee) Fields() []ent.Field {
	return []ent.Field{
		field.Int("shop_id"),
		field.Int64("telegram_user_id"),
		field.String("display_name"),
		field.Float("max_hours_per_week").Default(40),
		field.Bool("active").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Employee) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("employees").Field("shop_id").Unique().Required(),
		edge.To("availability", Availability.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("assignments", ScheduleAssignment.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("votes", ScheduleVote.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (Employee) Indexes() []ent.Index {
	return []ent.Index{
		// One membership per Telegram account per shop.
		index.Fields("shop_id", "telegram_user_id").Unique(),
		index.Fields("telegram_user_id"),
	}
}
