package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ScheduleAssignment puts one employee on one shift within one schedule
// candidate.
type ScheduleAssignment struct {
	ent.Schema
}

func (ScheduleAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.Int("shop_id"),
		field.Int("schedule_id"),
		field.Int("shift_id"),
		field.Int("employee_id"),
	}
}

func (ScheduleAssignment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("assignments").Field("shop_id").Unique().Required(),
		edge.From("schedule", Schedule.Type).Ref("assignments").Field("schedule_id").Unique().Required(),
		edge.From("shift", Shift.Type).Ref("assignments").Field("shift_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("assignments").Field("employee_id").Unique().Required(),
	}
}

func (ScheduleAssignment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("schedule_id", "shift_id", "employee_id").Unique(),
		index.Fields("schedule_id"),
	}
}
