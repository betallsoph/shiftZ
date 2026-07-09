package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Rule is an owner scheduling rule: the original message plus the structured
// penalty rule the LLM translated it into, consumed by the solver.
type Rule struct {
	ent.Schema
}

func (Rule) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		// The owner's original message, kept for audit and re-translation.
		field.Text("description").Default(""),
		// Structured penalty rule (kind + params) consumed by the solver;
		// mirrors llm.RuleSpec.
		field.JSON("rule_json", map[string]any{}).Optional(),
		field.Float("weight").Default(1),
		field.Bool("is_active").Default(true),
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
		// Solver input: "active rules of this shop".
		index.Fields("shop_id", "is_active"),
	}
}
