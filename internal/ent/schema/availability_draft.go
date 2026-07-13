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

// AvailabilityDraft is a pending Telegram availability confirmation stored
// until the employee confirms or the draft expires.
type AvailabilityDraft struct {
	ent.Schema
}

func (AvailabilityDraft) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.UUID("employee_id", uuid.UUID{}),
		field.Int64("telegram_user_id"),
		field.Int64("chat_id"),
		field.Time("week_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.String("timezone"),
		field.JSON("slots", []AvailabilitySlot{}),
		field.Text("raw_message").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("expires_at"),
	}
}

func (AvailabilityDraft) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("availability_drafts").Field("shop_id").Unique().Required(),
		edge.From("employee", Employee.Type).Ref("availability_drafts").Field("employee_id").Unique().Required(),
	}
}

func (AvailabilityDraft) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("expires_at"),
		index.Fields("telegram_user_id", "expires_at"),
	}
}
