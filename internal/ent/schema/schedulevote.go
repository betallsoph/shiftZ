package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ScheduleVote is one employee's vote for a schedule candidate; at most one
// vote per employee per candidate.
type ScheduleVote struct {
	ent.Schema
}

func (ScheduleVote) Fields() []ent.Field {
	return []ent.Field{
		field.Int("shop_id"),
		field.Int("schedule_id"),
		field.Int("employee_id"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (ScheduleVote) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("votes").Field("shop_id").Unique().Required(),
		edge.From("schedule", Schedule.Type).Ref("votes").Field("schedule_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("votes").Field("employee_id").Unique().Required(),
	}
}

func (ScheduleVote) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("schedule_id", "employee_id").Unique(),
		index.Fields("shop_id"),
	}
}
