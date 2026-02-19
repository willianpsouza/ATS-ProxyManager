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
		`INSERT INTO proxy_stats (
			proxy_id, active_connections, total_connections, cache_hits, cache_misses, errors,
			total_requests, connect_requests, responses_2xx, responses_3xx, responses_4xx, responses_5xx,
			err_connect_fail, err_client_abort, broken_server_conns, bytes_in, bytes_out
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		 RETURNING id, collected_at`,
		s.ProxyID, s.ActiveConnections, s.TotalConnections, s.CacheHits, s.CacheMisses, s.Errors,
		s.TotalRequests, s.ConnectRequests, s.Responses2xx, s.Responses3xx, s.Responses4xx, s.Responses5xx,
		s.ErrConnectFail, s.ErrClientAbort, s.BrokenServerConns, s.BytesIn, s.BytesOut,
	).Scan(&s.ID, &s.CollectedAt)
	if err != nil {
		return fmt.Errorf("create proxy stat: %w", err)
	}
	return nil
}

func (r *ProxyStatsRepo) ListByProxy(ctx context.Context, proxyID uuid.UUID, limit int) ([]domain.ProxyStat, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, proxy_id, collected_at,
			active_connections, total_connections, cache_hits, cache_misses, errors,
			COALESCE(total_requests, 0), COALESCE(connect_requests, 0),
			COALESCE(responses_2xx, 0), COALESCE(responses_3xx, 0), COALESCE(responses_4xx, 0), COALESCE(responses_5xx, 0),
			COALESCE(err_connect_fail, 0), COALESCE(err_client_abort, 0), COALESCE(broken_server_conns, 0),
			COALESCE(bytes_in, 0), COALESCE(bytes_out, 0)
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
		if err := rows.Scan(
			&s.ID, &s.ProxyID, &s.CollectedAt,
			&s.ActiveConnections, &s.TotalConnections, &s.CacheHits, &s.CacheMisses, &s.Errors,
			&s.TotalRequests, &s.ConnectRequests,
			&s.Responses2xx, &s.Responses3xx, &s.Responses4xx, &s.Responses5xx,
			&s.ErrConnectFail, &s.ErrClientAbort, &s.BrokenServerConns,
			&s.BytesIn, &s.BytesOut,
		); err != nil {
			return nil, fmt.Errorf("scan proxy stat: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func (r *ProxyStatsRepo) ListByProxyAggregated(ctx context.Context, proxyID uuid.UUID, limit int) ([]domain.ProxyStat, error) {
	rows, err := r.db.Query(ctx,
		`SELECT MIN(id::text)::uuid, proxy_id, date_trunc('minute', collected_at) AS minute,
			MAX(active_connections)::int,
			SUM(total_connections)::bigint,
			SUM(cache_hits)::bigint,
			SUM(cache_misses)::bigint,
			SUM(errors)::int,
			SUM(COALESCE(total_requests, 0))::bigint,
			SUM(COALESCE(connect_requests, 0))::bigint,
			SUM(COALESCE(responses_2xx, 0))::bigint,
			SUM(COALESCE(responses_3xx, 0))::bigint,
			SUM(COALESCE(responses_4xx, 0))::bigint,
			SUM(COALESCE(responses_5xx, 0))::bigint,
			SUM(COALESCE(err_connect_fail, 0))::int,
			SUM(COALESCE(err_client_abort, 0))::int,
			SUM(COALESCE(broken_server_conns, 0))::int,
			SUM(COALESCE(bytes_in, 0))::bigint,
			SUM(COALESCE(bytes_out, 0))::bigint
		 FROM proxy_stats WHERE proxy_id = $1
		 GROUP BY proxy_id, date_trunc('minute', collected_at)
		 ORDER BY minute DESC LIMIT $2`, proxyID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list proxy stats aggregated: %w", err)
	}
	defer rows.Close()

	var stats []domain.ProxyStat
	for rows.Next() {
		var s domain.ProxyStat
		if err := rows.Scan(
			&s.ID, &s.ProxyID, &s.CollectedAt,
			&s.ActiveConnections, &s.TotalConnections, &s.CacheHits, &s.CacheMisses, &s.Errors,
			&s.TotalRequests, &s.ConnectRequests,
			&s.Responses2xx, &s.Responses3xx, &s.Responses4xx, &s.Responses5xx,
			&s.ErrConnectFail, &s.ErrClientAbort, &s.BrokenServerConns,
			&s.BytesIn, &s.BytesOut,
		); err != nil {
			return nil, fmt.Errorf("scan proxy stat aggregated: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// ProxyStatsSummary returns the most recent aggregated stats for a proxy (1 hour window).
type ProxyStatsSummary struct {
	ActiveConnections  int     `json:"active_connections"`
	TotalConnections1h int64   `json:"total_connections_1h"`
	CacheHitRate       float64 `json:"cache_hit_rate"`

	TotalRequests1h int64 `json:"total_requests_1h"`
	Errors1h        int64 `json:"errors_1h"`
	Responses2xx1h  int64 `json:"responses_2xx_1h"`
	Responses4xx1h  int64 `json:"responses_4xx_1h"`
	Responses5xx1h  int64 `json:"responses_5xx_1h"`
	BytesIn1h       int64 `json:"bytes_in_1h"`
	BytesOut1h      int64 `json:"bytes_out_1h"`
}

func (r *ProxyStatsRepo) SummaryForProxy(ctx context.Context, proxyID uuid.UUID) (*ProxyStatsSummary, error) {
	var s ProxyStatsSummary
	var cacheHits, cacheMisses int64
	err := r.db.QueryRow(ctx,
		`SELECT
		   COALESCE((SELECT active_connections FROM proxy_stats WHERE proxy_id = $1 ORDER BY collected_at DESC LIMIT 1), 0),
		   COALESCE(SUM(total_connections), 0),
		   COALESCE(SUM(cache_hits), 0),
		   COALESCE(SUM(cache_misses), 0),
		   COALESCE(SUM(COALESCE(total_requests, 0)), 0),
		   COALESCE(SUM(errors), 0),
		   COALESCE(SUM(COALESCE(responses_2xx, 0)), 0),
		   COALESCE(SUM(COALESCE(responses_4xx, 0)), 0),
		   COALESCE(SUM(COALESCE(responses_5xx, 0)), 0),
		   COALESCE(SUM(COALESCE(bytes_in, 0)), 0),
		   COALESCE(SUM(COALESCE(bytes_out, 0)), 0)
		 FROM proxy_stats
		 WHERE proxy_id = $1 AND collected_at > NOW() - INTERVAL '1 hour'`,
		proxyID,
	).Scan(
		&s.ActiveConnections, &s.TotalConnections1h, &cacheHits, &cacheMisses,
		&s.TotalRequests1h, &s.Errors1h, &s.Responses2xx1h, &s.Responses4xx1h, &s.Responses5xx1h,
		&s.BytesIn1h, &s.BytesOut1h,
	)
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
