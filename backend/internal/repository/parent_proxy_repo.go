package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type ParentProxyRepo struct {
	db DBTX
}

func NewParentProxyRepo(db DBTX) *ParentProxyRepo {
	return &ParentProxyRepo{db: db}
}

func (r *ParentProxyRepo) ListByConfig(ctx context.Context, configID uuid.UUID) ([]domain.ParentProxy, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, config_id, address, port, priority, enabled, created_at
		 FROM parent_proxies WHERE config_id = $1 ORDER BY priority`, configID,
	)
	if err != nil {
		return nil, fmt.Errorf("list parent proxies: %w", err)
	}
	defer rows.Close()

	var proxies []domain.ParentProxy
	for rows.Next() {
		var pp domain.ParentProxy
		if err := rows.Scan(&pp.ID, &pp.ConfigID, &pp.Address, &pp.Port, &pp.Priority, &pp.Enabled, &pp.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan parent proxy: %w", err)
		}
		proxies = append(proxies, pp)
	}
	return proxies, nil
}

func (r *ParentProxyRepo) Create(ctx context.Context, pp *domain.ParentProxy) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO parent_proxies (config_id, address, port, priority, enabled)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at`,
		pp.ConfigID, pp.Address, pp.Port, pp.Priority, pp.Enabled,
	).Scan(&pp.ID, &pp.CreatedAt)
	if err != nil {
		return fmt.Errorf("create parent proxy: %w", err)
	}
	return nil
}

func (r *ParentProxyRepo) DeleteByConfig(ctx context.Context, configID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM parent_proxies WHERE config_id = $1`, configID,
	)
	return err
}
