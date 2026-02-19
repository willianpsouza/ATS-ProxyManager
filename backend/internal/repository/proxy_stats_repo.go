package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type ProxyStatsRepo struct {
	db DBTX
}

func NewProxyStatsRepo(db DBTX) *ProxyStatsRepo {
	return &ProxyStatsRepo{db: db}
}

func (r *ProxyStatsRepo) Create(ctx context.Context, s *domain.ProxyStat) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO proxy_stats (proxy_id, active_connections, total_connections, cache_hits, cache_misses, errors)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, collected_at`,
		s.ProxyID, s.ActiveConnections, s.TotalConnections, s.CacheHits, s.CacheMisses, s.Errors,
	).Scan(&s.ID, &s.CollectedAt)
	if err != nil {
		return fmt.Errorf("create proxy stat: %w", err)
	}
	return nil
}

func (r *ProxyStatsRepo) ListByProxy(ctx context.Context, proxyID uuid.UUID, limit int) ([]domain.ProxyStat, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, proxy_id, collected_at, active_connections, total_connections, cache_hits, cache_misses, errors
		 FROM proxy_stats WHERE proxy_id = $1
		 ORDER BY collected_at DESC LIMIT $2`, proxyID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list proxy stats: %w", err)
	}
	defer rows.Close()

	var stats []domain.ProxyStat
	for rows.Next() {
		var s domain.ProxyStat
		if err := rows.Scan(&s.ID, &s.ProxyID, &s.CollectedAt, &s.ActiveConnections, &s.TotalConnections, &s.CacheHits, &s.CacheMisses, &s.Errors); err != nil {
			return nil, fmt.Errorf("scan proxy stat: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// LatestForProxy returns the most recent aggregated stats for a proxy (1 hour window).
type ProxyStatsSummary struct {
	ActiveConnections  int     `json:"active_connections"`
	TotalConnections1h int64   `json:"total_connections_1h"`
	CacheHitRate       float64 `json:"cache_hit_rate"`
}

func (r *ProxyStatsRepo) SummaryForProxy(ctx context.Context, proxyID uuid.UUID) (*ProxyStatsSummary, error) {
	var s ProxyStatsSummary
	var cacheHits, cacheMisses int64
	err := r.db.QueryRow(ctx,
		`SELECT
		   COALESCE((SELECT active_connections FROM proxy_stats WHERE proxy_id = $1 ORDER BY collected_at DESC LIMIT 1), 0),
		   COALESCE(SUM(total_connections), 0),
		   COALESCE(SUM(cache_hits), 0),
		   COALESCE(SUM(cache_misses), 0)
		 FROM proxy_stats
		 WHERE proxy_id = $1 AND collected_at > NOW() - INTERVAL '1 hour'`,
		proxyID,
	).Scan(&s.ActiveConnections, &s.TotalConnections1h, &cacheHits, &cacheMisses)
	if err != nil {
		return nil, fmt.Errorf("summary for proxy: %w", err)
	}
	total := cacheHits + cacheMisses
	if total > 0 {
		s.CacheHitRate = float64(cacheHits) / float64(total)
	}
	return &s, nil
}

func (r *ProxyStatsRepo) CleanupOld(ctx context.Context) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM proxy_stats WHERE collected_at < NOW() - INTERVAL '7 days'`,
	)
	return err
}
