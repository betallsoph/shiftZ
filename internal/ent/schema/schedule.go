package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Schedule is one solver-generated schedule candidate for a week; status
// tracks the voting flow.
type Schedule struct {
	ent.Schema
}

func (Schedule) Fields() []ent.Field {
	return []ent.Field{
		field.Int("shop_id"),
		field.Time("week_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		// e.g. "balanced", "fairness-first", "preference-first".
		field.String("label").Default(""),
		field.Enum("status").
			Values("draft", "voting", "final", "vetoed").
			Default("draft"),
		field.Float("score").Default(0),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Schedule) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("schedules").Field("shop_id").Unique().Required(),
		edge.To("assignments", ScheduleAssignment.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("votes", ScheduleVote.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (Schedule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("shop_id", "week_start"),
	}
}
