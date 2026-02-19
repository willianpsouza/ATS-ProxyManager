package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type ProxyLogsRepo struct {
	db DBTX
}

func NewProxyLogsRepo(db DBTX) *ProxyLogsRepo {
	return &ProxyLogsRepo{db: db}
}

func (r *ProxyLogsRepo) Create(ctx context.Context, l *domain.ProxyLog) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO proxy_logs (proxy_id, log_level, message)
		 VALUES ($1, $2, $3)
		 RETURNING id, captured_at, expires_at`,
		l.ProxyID, l.LogLevel, l.Message,
	).Scan(&l.ID, &l.CapturedAt, &l.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create proxy log: %w", err)
	}
	return nil
}

func (r *ProxyLogsRepo) BulkCreate(ctx context.Context, proxyID uuid.UUID, logs []domain.ProxyLog) error {
	for i := range logs {
		logs[i].ProxyID = proxyID
		if err := r.Create(ctx, &logs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProxyLogsRepo) ListByProxy(ctx context.Context, proxyID uuid.UUID) ([]domain.ProxyLog, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, proxy_id, captured_at, log_level, message, expires_at
		 FROM proxy_logs WHERE proxy_id = $1 AND expires_at > NOW()
		 ORDER BY captured_at DESC`, proxyID,
	)
	if err != nil {
		return nil, fmt.Errorf("list proxy logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.ProxyLog
	for rows.Next() {
		var l domain.ProxyLog
		if err := rows.Scan(&l.ID, &l.ProxyID, &l.CapturedAt, &l.LogLevel, &l.Message, &l.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan proxy log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (r *ProxyLogsRepo) CleanupExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM proxy_logs WHERE expires_at < NOW()`,
	)
	return err
}
