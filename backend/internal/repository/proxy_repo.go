package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type ProxyRepo struct {
	db DBTX
}

func NewProxyRepo(db DBTX) *ProxyRepo {
	return &ProxyRepo{db: db}
}

func (r *ProxyRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Proxy, error) {
	var p domain.Proxy
	err := r.db.QueryRow(ctx,
		`SELECT id, hostname, config_id, is_online, last_seen, current_config_hash, registered_at, capture_logs_until
		 FROM proxies WHERE id = $1`, id,
	).Scan(&p.ID, &p.Hostname, &p.ConfigID, &p.IsOnline, &p.LastSeen, &p.CurrentConfigHash, &p.RegisteredAt, &p.CaptureLogsUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get proxy by id: %w", err)
	}
	return &p, nil
}

func (r *ProxyRepo) GetByHostname(ctx context.Context, hostname string) (*domain.Proxy, error) {
	var p domain.Proxy
	err := r.db.QueryRow(ctx,
		`SELECT id, hostname, config_id, is_online, last_seen, current_config_hash, registered_at, capture_logs_until
		 FROM proxies WHERE hostname = $1`, hostname,
	).Scan(&p.ID, &p.Hostname, &p.ConfigID, &p.IsOnline, &p.LastSeen, &p.CurrentConfigHash, &p.RegisteredAt, &p.CaptureLogsUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get proxy by hostname: %w", err)
	}
	return &p, nil
}

func (r *ProxyRepo) List(ctx context.Context) ([]domain.Proxy, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, hostname, config_id, is_online, last_seen, current_config_hash, registered_at, capture_logs_until
		 FROM proxies ORDER BY hostname`,
	)
	if err != nil {
		return nil, fmt.Errorf("list proxies: %w", err)
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

func (r *ProxyRepo) Create(ctx context.Context, p *domain.Proxy) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO proxies (hostname, config_id)
		 VALUES ($1, $2)
		 RETURNING id, is_online, registered_at`,
		p.Hostname, p.ConfigID,
	).Scan(&p.ID, &p.IsOnline, &p.RegisteredAt)
	if err != nil {
		return fmt.Errorf("create proxy: %w", err)
	}
	return nil
}

func (r *ProxyRepo) Upsert(ctx context.Context, p *domain.Proxy) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO proxies (hostname, config_id)
		 VALUES ($1, $2)
		 ON CONFLICT (hostname) DO UPDATE SET config_id = COALESCE(EXCLUDED.config_id, proxies.config_id)
		 RETURNING id, is_online, last_seen, current_config_hash, registered_at, capture_logs_until`,
		p.Hostname, p.ConfigID,
	).Scan(&p.ID, &p.IsOnline, &p.LastSeen, &p.CurrentConfigHash, &p.RegisteredAt, &p.CaptureLogsUntil)
	if err != nil {
		return fmt.Errorf("upsert proxy: %w", err)
	}
	return nil
}

func (r *ProxyRepo) UpdateLastSeen(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE proxies SET last_seen = NOW(), is_online = true WHERE id = $1`, id,
	)
	return err
}

func (r *ProxyRepo) UpdateConfigHash(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE proxies SET current_config_hash = $1, last_seen = NOW(), is_online = true WHERE id = $2`, hash, id,
	)
	return err
}

func (r *ProxyRepo) SetCaptureLogsUntil(ctx context.Context, id uuid.UUID, until time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE proxies SET capture_logs_until = $1 WHERE id = $2`, until, id,
	)
	return err
}

func (r *ProxyRepo) MarkOfflineStale(ctx context.Context) error {
	_, err := r.db.Exec(ctx,
		`UPDATE proxies SET is_online = FALSE WHERE last_seen < NOW() - INTERVAL '2 minutes' AND is_online = TRUE`,
	)
	return err
}

func (r *ProxyRepo) DeleteOfflineStale(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM proxies WHERE is_online = FALSE AND last_seen < NOW() - INTERVAL '4 hours'`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete offline stale proxies: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *ProxyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM proxies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete proxy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
