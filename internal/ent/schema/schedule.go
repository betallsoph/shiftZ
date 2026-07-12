package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Schedule is one solver-generated schedule variant for a week; status
// tracks the vote-then-approve flow.
type Schedule struct {
	ent.Schema
}

func (Schedule) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.Time("week_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Enum("status").
			Values("draft", "voting", "approved", "published").
			Default("draft"),
		// Which variant this is in the vote, e.g. "A", "B", "C".
		field.String("variant_label").Default(""),
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
		// Voting flow: "this shop's variants for this week".
		index.Fields("shop_id", "week_start"),
		// One variant label per shop/week; prevents duplicate generate races.
		index.Fields("shop_id", "week_start", "variant_label").Unique(),
	}
}
