package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Rule is an owner scheduling rule translated from natural language into a
// solver penalty spec (kind + params), mirroring llm.RuleSpec.
type Rule struct {
	ent.Schema
}

func (Rule) Fields() []ent.Field {
	return []ent.Field{
		field.Int("shop_id"),
		// avoid_pair | day_off | custom
		field.String("kind"),
		field.JSON("params", map[string]any{}).Optional(),
		field.Float("weight").Default(1),
		// The owner's original wording, kept for auditing and re-translation.
		field.Text("source_text").Default(""),
		field.Bool("active").Default(true),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Rule) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("rules").Field("shop_id").Unique().Required(),
	}
}

func (Rule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("shop_id", "active"),
	}
}
