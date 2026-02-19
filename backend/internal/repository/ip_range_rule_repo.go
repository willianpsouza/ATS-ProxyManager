package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type IPRangeRuleRepo struct {
	db DBTX
}

func NewIPRangeRuleRepo(db DBTX) *IPRangeRuleRepo {
	return &IPRangeRuleRepo{db: db}
}

func (r *IPRangeRuleRepo) ListByConfig(ctx context.Context, configID uuid.UUID) ([]domain.IPRangeRule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, config_id, cidr, action, priority, created_at
		 FROM ip_range_rules WHERE config_id = $1 ORDER BY priority`, configID,
	)
	if err != nil {
		return nil, fmt.Errorf("list ip range rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.IPRangeRule
	for rows.Next() {
		var ir domain.IPRangeRule
		if err := rows.Scan(&ir.ID, &ir.ConfigID, &ir.CIDR, &ir.Action, &ir.Priority, &ir.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan ip range rule: %w", err)
		}
		rules = append(rules, ir)
	}
	return rules, nil
}

func (r *IPRangeRuleRepo) Create(ctx context.Context, ir *domain.IPRangeRule) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO ip_range_rules (config_id, cidr, action, priority)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		ir.ConfigID, ir.CIDR, ir.Action, ir.Priority,
	).Scan(&ir.ID, &ir.CreatedAt)
	if err != nil {
		return fmt.Errorf("create ip range rule: %w", err)
	}
	return nil
}

func (r *IPRangeRuleRepo) DeleteByConfig(ctx context.Context, configID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM ip_range_rules WHERE config_id = $1`, configID,
	)
	return err
}
