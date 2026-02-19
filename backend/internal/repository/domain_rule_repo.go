package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type DomainRuleRepo struct {
	db DBTX
}

func NewDomainRuleRepo(db DBTX) *DomainRuleRepo {
	return &DomainRuleRepo{db: db}
}

func (r *DomainRuleRepo) ListByConfig(ctx context.Context, configID uuid.UUID) ([]domain.DomainRule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, config_id, domain, action, priority, created_at
		 FROM domain_rules WHERE config_id = $1 ORDER BY priority`, configID,
	)
	if err != nil {
		return nil, fmt.Errorf("list domain rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.DomainRule
	for rows.Next() {
		var dr domain.DomainRule
		if err := rows.Scan(&dr.ID, &dr.ConfigID, &dr.Domain, &dr.Action, &dr.Priority, &dr.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan domain rule: %w", err)
		}
		rules = append(rules, dr)
	}
	return rules, nil
}

func (r *DomainRuleRepo) Create(ctx context.Context, dr *domain.DomainRule) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO domain_rules (config_id, domain, action, priority)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		dr.ConfigID, dr.Domain, dr.Action, dr.Priority,
	).Scan(&dr.ID, &dr.CreatedAt)
	if err != nil {
		return fmt.Errorf("create domain rule: %w", err)
	}
	return nil
}

func (r *DomainRuleRepo) DeleteByConfig(ctx context.Context, configID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM domain_rules WHERE config_id = $1`, configID,
	)
	return err
}
