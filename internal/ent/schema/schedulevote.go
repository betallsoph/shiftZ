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

// ScheduleVote is one employee's vote in one weekly voting round. week_start
// is denormalized onto the vote so uniqueness can span variants: one vote
// per employee per round, re-voting switches the chosen variant.
type ScheduleVote struct {
	ent.Schema
}

func (ScheduleVote) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.UUID("schedule_id", uuid.UUID{}),
		field.UUID("employee_id", uuid.UUID{}),
		field.Time("week_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (ScheduleVote) Edges() []ent.Edge {
	return []ent.Edge{
		// Unidirectional: Shop doesn't enumerate votes directly (they're
		// reached through schedules), but rows keep the tenant FK.
		edge.To("shop", Shop.Type).Field("shop_id").Unique().Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("schedule", Schedule.Type).Ref("votes").Field("schedule_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("votes").Field("employee_id").Unique().Required(),
	}
}

func (ScheduleVote) Indexes() []ent.Index {
	return []ent.Index{
		// One vote per employee per voting round, across variants;
		// re-voting upserts on this key.
		index.Fields("shop_id", "employee_id", "week_start").Unique(),
		// Tallying: "all votes for this schedule variant".
		index.Fields("shop_id", "schedule_id"),
	}
}
