package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type ConfigRepo struct {
	db DBTX
}

func NewConfigRepo(db DBTX) *ConfigRepo {
	return &ConfigRepo{db: db}
}

func (r *ConfigRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Config, error) {
	var c domain.Config
	err := r.db.QueryRow(ctx,
		`SELECT id, name, description, status, version,
		        created_by, created_at, modified_by, modified_at,
		        submitted_by, submitted_at, approved_by, approved_at, config_hash, default_action
		 FROM configs WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Description, &c.Status, &c.Version,
		&c.CreatedBy, &c.CreatedAt, &c.ModifiedBy, &c.ModifiedAt,
		&c.SubmittedBy, &c.SubmittedAt, &c.ApprovedBy, &c.ApprovedAt, &c.ConfigHash, &c.DefaultAction)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get config by id: %w", err)
	}
	return &c, nil
}

func (r *ConfigRepo) List(ctx context.Context, status *domain.ConfigStatus, limit, offset int) ([]domain.Config, int, error) {
	var total int
	countQuery := `SELECT COUNT(*) FROM configs`
	listQuery := `SELECT c.id, c.name, c.description, c.status, c.version,
	              c.created_by, c.created_at, c.modified_by, c.modified_at,
	              c.submitted_by, c.submitted_at, c.approved_by, c.approved_at, c.config_hash,
	              c.default_action, COUNT(cp.proxy_id) AS proxy_count
	              FROM configs c
	              LEFT JOIN config_proxies cp ON c.id = cp.config_id`

	args := []interface{}{}
	if status != nil {
		countQuery += ` WHERE status = $1`
		listQuery += ` WHERE c.status = $1`
		args = append(args, *status)
	}

	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count configs: %w", err)
	}

	listQuery += ` GROUP BY c.id`

	argIdx := len(args) + 1
	listQuery += fmt.Sprintf(` ORDER BY c.modified_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list configs: %w", err)
	}
	defer rows.Close()

	var configs []domain.Config
	for rows.Next() {
		var c domain.Config
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Status, &c.Version,
			&c.CreatedBy, &c.CreatedAt, &c.ModifiedBy, &c.ModifiedAt,
			&c.SubmittedBy, &c.SubmittedAt, &c.ApprovedBy, &c.ApprovedAt, &c.ConfigHash,
			&c.DefaultAction, &c.ProxyCount); err != nil {
			return nil, 0, fmt.Errorf("scan config: %w", err)
		}
		configs = append(configs, c)
	}

	return configs, total, nil
}

func (r *ConfigRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM configs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ConfigRepo) Create(ctx context.Context, c *domain.Config) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO configs (name, description, status, default_action, created_by, modified_by)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 RETURNING id, version, created_at, modified_at`,
		c.Name, c.Description, domain.StatusDraft, c.DefaultAction, c.CreatedBy,
	).Scan(&c.ID, &c.Version, &c.CreatedAt, &c.ModifiedAt)
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	c.Status = domain.StatusDraft
	return nil
}

func (r *ConfigRepo) CreateWithVersion(ctx context.Context, c *domain.Config, version int) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO configs (name, description, status, version, default_action, created_by, modified_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $6)
		 RETURNING id, created_at, modified_at`,
		c.Name, c.Description, domain.StatusDraft, version, c.DefaultAction, c.CreatedBy,
	).Scan(&c.ID, &c.CreatedAt, &c.ModifiedAt)
	if err != nil {
		return fmt.Errorf("create config with version: %w", err)
	}
	c.Status = domain.StatusDraft
	c.Version = version
	return nil
}

func (r *ConfigRepo) Update(ctx context.Context, c *domain.Config) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE configs SET name = $1, description = $2, default_action = $3, modified_by = $4, modified_at = NOW()
		 WHERE id = $5 AND status = 'draft'`,
		c.Name, c.Description, c.DefaultAction, c.ModifiedBy, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidStatus
	}
	return nil
}

func (r *ConfigRepo) Submit(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE configs SET status = 'pending_approval', submitted_by = $1, submitted_at = NOW(), modified_by = $1, modified_at = NOW()
		 WHERE id = $2 AND status = 'draft'`,
		userID, id,
	)
	if err != nil {
		return fmt.Errorf("submit config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidStatus
	}
	return nil
}

func (r *ConfigRepo) Approve(ctx context.Context, id, userID uuid.UUID, hash string) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE configs SET status = 'active', approved_by = $1, approved_at = NOW(),
		        modified_by = $1, modified_at = NOW(), config_hash = $3
		 WHERE id = $2 AND status = 'pending_approval'`,
		userID, id, hash,
	)
	if err != nil {
		return fmt.Errorf("approve config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidStatus
	}
	return nil
}

func (r *ConfigRepo) Reject(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE configs SET status = 'draft', submitted_by = NULL, submitted_at = NULL, modified_at = NOW()
		 WHERE id = $1 AND status = 'pending_approval'`,
		id,
	)
	if err != nil {
		return fmt.Errorf("reject config: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidStatus
	}
	return nil
}

func (r *ConfigRepo) UpdateHash(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE configs SET config_hash = $1 WHERE id = $2`, hash, id,
	)
	return err
}

func (r *ConfigRepo) GetActiveForProxy(ctx context.Context, hostname string) (*domain.Config, error) {
	var c domain.Config
	err := r.db.QueryRow(ctx,
		`SELECT c.id, c.name, c.description, c.status, c.version,
		        c.created_by, c.created_at, c.modified_by, c.modified_at,
		        c.submitted_by, c.submitted_at, c.approved_by, c.approved_at, c.config_hash, c.default_action
		 FROM configs c
		 JOIN config_proxies cp ON c.id = cp.config_id
		 JOIN proxies p ON cp.proxy_id = p.id
		 WHERE p.hostname = $1 AND c.status = 'active'
		 LIMIT 1`, hostname,
	).Scan(&c.ID, &c.Name, &c.Description, &c.Status, &c.Version,
		&c.CreatedBy, &c.CreatedAt, &c.ModifiedBy, &c.ModifiedAt,
		&c.SubmittedBy, &c.SubmittedAt, &c.ApprovedBy, &c.ApprovedAt, &c.ConfigHash, &c.DefaultAction)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get active config for proxy: %w", err)
	}
	return &c, nil
}

// DeactivateOthers sets other active configs that share proxies with the given config to 'approved'.
func (r *ConfigRepo) DeactivateOthers(ctx context.Context, activeID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE configs SET status = 'approved'
		 WHERE status = 'active' AND id != $1
		   AND id IN (
		     SELECT cp2.config_id FROM config_proxies cp2
		     WHERE cp2.proxy_id IN (
		       SELECT cp1.proxy_id FROM config_proxies cp1 WHERE cp1.config_id = $1
		     )
		   )`,
		activeID,
	)
	return err
}
