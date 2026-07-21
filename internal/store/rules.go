package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/ent"
	"github.com/betallsoph/shiftz/internal/ent/rule"
)

// RuleRepo covers owner scheduling rules for a shop.
type RuleRepo struct {
	client *ent.Client
}

// Create inserts a new soft scheduling rule for a shop.
func (r *RuleRepo) Create(ctx context.Context, shopID uuid.UUID, description string, ruleJSON map[string]any, weight float64) (*Rule, error) {
	if ruleJSON == nil {
		ruleJSON = map[string]any{}
	}
	if weight == 0 {
		weight = 1
	}
	row, err := r.client.Rule.Create().
		SetShopID(shopID).
		SetDescription(description).
		SetRuleJSON(ruleJSON).
		SetWeight(weight).
		SetIsActive(true).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: create rule: %w", err)
	}
	return ruleFromEnt(row), nil
}

// ListByShop returns all rules for a shop, ordered by created_at.
func (r *RuleRepo) ListByShop(ctx context.Context, shopID uuid.UUID) ([]*Rule, error) {
	rows, err := r.client.Rule.Query().
		Where(rule.ShopID(shopID)).
		Order(rule.ByCreatedAt()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list rules: %w", err)
	}
	out := make([]*Rule, len(rows))
	for i, row := range rows {
		out[i] = ruleFromEnt(row)
	}
	return out, nil
}
