package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Shift is a slot of work that needs staffing.
type Shift struct {
	ent.Schema
}

func (Shift) Fields() []ent.Field {
	return []ent.Field{
		field.Int("shop_id"),
		field.String("role").Default(""),
		field.Time("starts_at"),
		field.Time("ends_at"),
		field.Int("min_staff").Default(1).NonNegative(),
		field.Int("max_staff").Default(1).NonNegative(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Shift) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("shifts").Field("shop_id").Unique().Required(),
		edge.To("assignments", ScheduleAssignment.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (Shift) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("shop_id", "starts_at"),
	}
}
