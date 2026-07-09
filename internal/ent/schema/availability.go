package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Availability is one employee's availability for one week: the parsed
// per-day/shift slots plus the original Telegram message kept for audit and
// re-parsing. Resubmitting the same week upserts the row.
type Availability struct {
	ent.Schema
}

func (Availability) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.UUID("employee_id", uuid.UUID{}),
		field.Time("week_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		// Parsed per-day/shift availability produced by the LLM.
		field.JSON("slots", []AvailabilitySlot{}),
		// Original Telegram text, kept for audit and re-parsing.
		field.Text("raw_message").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Availability) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("availabilities").Field("shop_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("availabilities").Field("employee_id").Unique().Required(),
	}
}

func (Availability) Indexes() []ent.Index {
	return []ent.Index{
		// One row per employee per week; resubmit = upsert on this key.
		index.Fields("shop_id", "employee_id", "week_start").Unique(),
		// Solver input: "all availability of this shop for this week".
		index.Fields("shop_id", "week_start"),
	}
}
