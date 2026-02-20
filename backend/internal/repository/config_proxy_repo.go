package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type ConfigProxyRepo struct {
	db DBTX
}

func NewConfigProxyRepo(db DBTX) *ConfigProxyRepo {
	return &ConfigProxyRepo{db: db}
}

func (r *ConfigProxyRepo) ListByConfig(ctx context.Context, configID uuid.UUID) ([]domain.Proxy, error) {
	rows, err := r.db.Query(ctx,
		`SELECT p.id, p.hostname, p.config_id, p.is_online, p.last_seen, p.current_config_hash, p.registered_at, p.capture_logs_until
		 FROM proxies p
		 JOIN config_proxies cp ON p.id = cp.proxy_id
		 WHERE cp.config_id = $1
		 ORDER BY p.hostname`, configID,
	)
	if err != nil {
		return nil, fmt.Errorf("list proxies by config: %w", err)
	}
	defer rows.Close()

	var proxies []domain.Proxy
	for rows.Next() {
		var p domain.Proxy
		if err := rows.Scan(&p.ID, &p.Hostname, &p.ConfigID, &p.IsOnline, &p.LastSeen, &p.CurrentConfigHash, &p.RegisteredAt, &p.CaptureLogsUntil); err != nil {
			return nil, fmt.Errorf("scan proxy: %w", err)
		}
		proxies = append(proxies, p)
	}
	return proxies, nil
}

func (r *ConfigProxyRepo) Assign(ctx context.Context, configID, proxyID, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO config_proxies (config_id, proxy_id, assigned_by)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (config_id, proxy_id) DO NOTHING`,
		configID, proxyID, userID,
	)
	if err != nil {
		return fmt.Errorf("assign proxy to config: %w", err)
	}
	return nil
}

func (r *ConfigProxyRepo) DeleteByConfig(ctx context.Context, configID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM config_proxies WHERE config_id = $1`, configID,
	)
	return err
}

func (r *ConfigProxyRepo) DeleteByProxy(ctx context.Context, proxyID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM config_proxies WHERE proxy_id = $1`, proxyID,
	)
	return err
}
