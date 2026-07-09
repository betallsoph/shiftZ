// Package schema holds shiftbot's ent entity definitions. Every
// tenant-owned entity carries an explicit shop_id FK field bound to its shop
// edge, keeping multi-tenant scoping visible in queries and indexes.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Shop is a tenant: one restaurant or cafe.
type Shop struct {
	ent.Schema
}

func (Shop) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("timezone").Default("UTC"),
		field.String("invite_code").Unique(),
		field.Int64("owner_telegram_id"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Shop) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("employees", Employee.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("availability", Availability.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("shifts", Shift.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("schedules", Schedule.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("assignments", ScheduleAssignment.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("votes", ScheduleVote.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("rules", Rule.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
