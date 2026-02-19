package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type AuditRepo struct {
	db DBTX
}

func NewAuditRepo(db DBTX) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Create(ctx context.Context, a *domain.AuditLog) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO audit_logs (user_id, action, entity_type, entity_id, old_value, new_value, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, created_at`,
		a.UserID, a.Action, a.EntityType, a.EntityID, a.OldValue, a.NewValue, a.IPAddress, a.UserAgent,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

type AuditFilter struct {
	EntityType *string
	EntityID   *uuid.UUID
	UserID     *uuid.UUID
	From       *time.Time
	To         *time.Time
}

func (r *AuditRepo) List(ctx context.Context, f AuditFilter, limit, offset int) ([]domain.AuditLog, int, error) {
	where := " WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if f.EntityType != nil {
		where += fmt.Sprintf(" AND entity_type = $%d", argIdx)
		args = append(args, *f.EntityType)
		argIdx++
	}
	if f.EntityID != nil {
		where += fmt.Sprintf(" AND entity_id = $%d", argIdx)
		args = append(args, *f.EntityID)
		argIdx++
	}
	if f.UserID != nil {
		where += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, *f.UserID)
		argIdx++
	}
	if f.From != nil {
		where += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, *f.From)
		argIdx++
	}
	if f.To != nil {
		where += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, *f.To)
		argIdx++
	}

	var total int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	query := `SELECT id, user_id, action, entity_type, entity_id, old_value, new_value, ip_address, user_agent, created_at
	          FROM audit_logs` + where +
		fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.EntityType, &a.EntityID, &a.OldValue, &a.NewValue, &a.IPAddress, &a.UserAgent, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, a)
	}
	return logs, total, nil
}
