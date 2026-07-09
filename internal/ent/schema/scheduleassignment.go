package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ScheduleAssignment puts one employee on one shift template on one concrete
// date, within one schedule variant.
type ScheduleAssignment struct {
	ent.Schema
}

func (ScheduleAssignment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.UUID("schedule_id", uuid.UUID{}),
		field.UUID("shift_id", uuid.UUID{}),
		field.UUID("employee_id", uuid.UUID{}),
		// The concrete date this shift-template occurrence falls on.
		field.Time("date").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
	}
}

func (ScheduleAssignment) Edges() []ent.Edge {
	return []ent.Edge{
		// Unidirectional: Shop doesn't enumerate assignments directly
		// (they're reached through schedules), but rows keep the tenant FK.
		edge.To("shop", Shop.Type).Field("shop_id").Unique().Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.From("schedule", Schedule.Type).Ref("assignments").Field("schedule_id").Unique().Required(),
		edge.From("shift", Shift.Type).Ref("assignments").Field("shift_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("assignments").Field("employee_id").Unique().Required(),
	}
}

func (ScheduleAssignment) Indexes() []ent.Index {
	return []ent.Index{
		// Rendering a schedule variant: "all assignments of this schedule".
		index.Fields("shop_id", "schedule_id"),
		// Per-employee views and conflict checks: "what does this employee
		// work on this date".
		index.Fields("shop_id", "employee_id", "date"),
	}
}
