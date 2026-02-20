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

type SessionRepo struct {
	db DBTX
}

func NewSessionRepo(db DBTX) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO sessions (user_id, token_hash, refresh_token_hash, ip_address, user_agent, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, last_beacon, created_at`,
		s.UserID, s.TokenHash, s.RefreshTokenHash, s.IPAddress, s.UserAgent, s.ExpiresAt,
	).Scan(&s.ID, &s.LastBeacon, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, refresh_token_hash, ip_address, user_agent,
		        last_beacon, expires_at, created_at, revoked_at
		 FROM sessions
		 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		tokenHash,
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.RefreshTokenHash, &s.IPAddress, &s.UserAgent,
		&s.LastBeacon, &s.ExpiresAt, &s.CreatedAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session by token: %w", err)
	}
	return &s, nil
}

func (r *SessionRepo) GetByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	var s domain.Session
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, token_hash, refresh_token_hash, ip_address, user_agent,
		        last_beacon, expires_at, created_at, revoked_at
		 FROM sessions
		 WHERE refresh_token_hash = $1 AND revoked_at IS NULL`,
		hash,
	).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.RefreshTokenHash, &s.IPAddress, &s.UserAgent,
		&s.LastBeacon, &s.ExpiresAt, &s.CreatedAt, &s.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session by refresh token: %w", err)
	}
	return &s, nil
}

func (r *SessionRepo) UpdateBeacon(ctx context.Context, id uuid.UUID, newExpiry time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET last_beacon = NOW(), expires_at = $1 WHERE id = $2`,
		newExpiry, id,
	)
	return err
}

func (r *SessionRepo) UpdateTokens(ctx context.Context, id uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET token_hash = $1, expires_at = $2 WHERE id = $3`,
		tokenHash, expiresAt, id,
	)
	return err
}

func (r *SessionRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE id = $1`, id,
	)
	return err
}

func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE sessions SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID,
	)
	return err
}
