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

// ReminderDelivery logs a Telegram reminder/nag send attempt for idempotency.
type ReminderDelivery struct {
	ent.Schema
}

func (ReminderDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.UUID("employee_id", uuid.UUID{}),
		field.Time("week_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.String("kind"),
		field.Enum("status").
			Values("pending", "sent", "failed").
			Default("pending"),
		field.Int("attempts").Default(0),
		field.Text("last_error").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("sent_at").Optional().Nillable(),
	}
}

func (ReminderDelivery) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("reminder_deliveries").Field("shop_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("reminder_deliveries").Field("employee_id").Unique().Required(),
	}
}

func (ReminderDelivery) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("shop_id", "employee_id", "week_start", "kind").Unique(),
		index.Fields("status", "created_at"),
	}
}
