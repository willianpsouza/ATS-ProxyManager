package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type ClientACLRepo struct {
	db DBTX
}

func NewClientACLRepo(db DBTX) *ClientACLRepo {
	return &ClientACLRepo{db: db}
}

func (r *ClientACLRepo) ListByConfig(ctx context.Context, configID uuid.UUID) ([]domain.ClientACLRule, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, config_id, cidr, action, priority, created_at
		 FROM client_acl_rules WHERE config_id = $1 ORDER BY priority`, configID,
	)
	if err != nil {
		return nil, fmt.Errorf("list client acl rules: %w", err)
	}
	defer rows.Close()

	var rules []domain.ClientACLRule
	for rows.Next() {
		var r domain.ClientACLRule
		if err := rows.Scan(&r.ID, &r.ConfigID, &r.CIDR, &r.Action, &r.Priority, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan client acl rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (r *ClientACLRepo) Create(ctx context.Context, rule *domain.ClientACLRule) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO client_acl_rules (config_id, cidr, action, priority)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		rule.ConfigID, rule.CIDR, rule.Action, rule.Priority,
	).Scan(&rule.ID, &rule.CreatedAt)
	if err != nil {
		return fmt.Errorf("create client acl rule: %w", err)
	}
	return nil
}

func (r *ClientACLRepo) DeleteByConfig(ctx context.Context, configID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM client_acl_rules WHERE config_id = $1`, configID,
	)
	return err
}
