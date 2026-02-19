package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type UserRepo struct {
	db DBTX
}

func NewUserRepo(db DBTX) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, username, email, password_hash, role, created_at, updated_at, last_login, is_active
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLogin, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx,
		`SELECT id, username, email, password_hash, role, created_at, updated_at, last_login, is_active
		 FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLogin, &u.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepo) List(ctx context.Context, role *domain.UserRole, limit, offset int) ([]domain.User, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM users WHERE is_active = true`
	listQuery := `SELECT id, username, email, password_hash, role, created_at, updated_at, last_login, is_active
	              FROM users WHERE is_active = true`

	args := []interface{}{}
	if role != nil {
		countQuery += ` AND role = $1`
		listQuery += ` AND role = $1`
		args = append(args, *role)
	}

	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	argIdx := len(args) + 1
	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastLogin, &u.IsActive); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, total, nil
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at, updated_at, is_active`,
		u.Username, u.Email, u.PasswordHash, u.Role,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt, &u.IsActive)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *UserRepo) Update(ctx context.Context, u *domain.User) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET username = $1, email = $2, role = $3 WHERE id = $4`,
		u.Username, u.Email, u.Role, u.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`, hash, id,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET is_active = false WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET last_login = NOW() WHERE id = $1`, id,
	)
	return err
}
