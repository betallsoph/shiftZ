package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Availability is one structured span of (un)availability parsed from an
// employee's free-text message, scoped to the week starting at week_start.
type Availability struct {
	ent.Schema
}

func (Availability) Fields() []ent.Field {
	return []ent.Field{
		field.Int("shop_id"),
		field.Int("employee_id"),
		field.Time("week_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Time("starts_at"),
		field.Time("ends_at"),
		// Solver scale: 0 unavailable, 1 available, 2 preferred.
		field.Int("preference").Default(1),
		field.String("note").Default(""),
		field.Text("raw_text").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Availability) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("availability").Field("shop_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("availability").Field("employee_id").Unique().Required(),
	}
}

func (Availability) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("shop_id", "week_start"),
		index.Fields("employee_id", "week_start"),
	}
}
