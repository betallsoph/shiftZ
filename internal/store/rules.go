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
