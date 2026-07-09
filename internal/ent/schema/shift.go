package schema

import (
	"context"
	"fmt"
	"regexp"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// timeOfDay matches "HH:MM" on a 24h clock.
var timeOfDay = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Shift is a weekly shift template: a named slot on one weekday with a
// time-of-day span and staffing bounds. Concrete dates appear on
// ScheduleAssignment when the solver instantiates a week.
type Shift struct {
	ent.Schema
}

func (Shift) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.UUID("shop_id", uuid.UUID{}),
		field.String("name"),
		// time.Weekday convention: 0 = Sunday ... 6 = Saturday.
		field.Int("weekday").Min(0).Max(6),
		field.String("start_time").Match(timeOfDay),
		field.String("end_time").Match(timeOfDay),
		field.Int("min_staff").Default(1).NonNegative(),
		field.Int("max_staff").Default(1).NonNegative(),
	}
}

func (Shift) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("shop", Shop.Type).Ref("shifts").Field("shop_id").Unique().Required(),
		edge.To("assignments", ScheduleAssignment.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

// Hooks enforces min_staff <= max_staff across fields. The check runs when
// both values are present on the mutation (always true on create, where
// defaults are populated first); a partial update touching only one bound is
// validated by the repo layer.
func (Shift) Hooks() []ent.Hook {
	return []ent.Hook{
		func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
				type staffMutation interface {
					MinStaff() (int, bool)
					MaxStaff() (int, bool)
				}
				if sm, ok := m.(staffMutation); ok {
					minStaff, okMin := sm.MinStaff()
					maxStaff, okMax := sm.MaxStaff()
					if okMin && okMax && minStaff > maxStaff {
						return nil, fmt.Errorf("schema: shift min_staff (%d) exceeds max_staff (%d)", minStaff, maxStaff)
					}
				}
				return next.Mutate(ctx, m)
			})
		},
	}
}
