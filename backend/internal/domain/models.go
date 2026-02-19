package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Role         UserRole   `json:"role"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
	IsActive     bool       `json:"is_active"`
}

type Session struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	TokenHash        string     `json:"-"`
	RefreshTokenHash *string    `json:"-"`
	IPAddress        *string    `json:"ip_address,omitempty"`
	UserAgent        *string    `json:"user_agent,omitempty"`
	LastBeacon       time.Time  `json:"last_beacon"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
}

type Config struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name"`
	Description *string      `json:"description,omitempty"`
	Status      ConfigStatus `json:"status"`
	Version     int          `json:"version"`

	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ModifiedBy  *uuid.UUID `json:"modified_by,omitempty"`
	ModifiedAt  time.Time  `json:"modified_at"`
	SubmittedBy *uuid.UUID `json:"submitted_by,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	ApprovedBy  *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	ConfigHash  *string    `json:"config_hash,omitempty"`
}

type DomainRule struct {
	ID        uuid.UUID  `json:"id"`
	ConfigID  uuid.UUID  `json:"config_id"`
	Domain    string     `json:"domain"`
	Action    RuleAction `json:"action"`
	Priority  int        `json:"priority"`
	CreatedAt time.Time  `json:"created_at"`
}

type IPRangeRule struct {
	ID        uuid.UUID  `json:"id"`
	ConfigID  uuid.UUID  `json:"config_id"`
	CIDR      string     `json:"cidr"`
	Action    RuleAction `json:"action"`
	Priority  int        `json:"priority"`
	CreatedAt time.Time  `json:"created_at"`
}

type ParentProxy struct {
	ID        uuid.UUID `json:"id"`
	ConfigID  uuid.UUID `json:"config_id"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Priority  int       `json:"priority"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type Proxy struct {
	ID                uuid.UUID  `json:"id"`
	Hostname          string     `json:"hostname"`
	ConfigID          *uuid.UUID `json:"config_id,omitempty"`
	IsOnline          bool       `json:"is_online"`
	LastSeen          *time.Time `json:"last_seen,omitempty"`
	CurrentConfigHash *string    `json:"current_config_hash,omitempty"`
	RegisteredAt      time.Time  `json:"registered_at"`
	CaptureLogsUntil  *time.Time `json:"capture_logs_until,omitempty"`
}

type ConfigProxy struct {
	ConfigID   uuid.UUID  `json:"config_id"`
	ProxyID    uuid.UUID  `json:"proxy_id"`
	AssignedAt time.Time  `json:"assigned_at"`
	AssignedBy *uuid.UUID `json:"assigned_by,omitempty"`
}

type ProxyStat struct {
	ID                uuid.UUID `json:"id"`
	ProxyID           uuid.UUID `json:"proxy_id"`
	CollectedAt       time.Time `json:"collected_at"`
	ActiveConnections int       `json:"active_connections"`
	TotalConnections  int64     `json:"total_connections"`
	CacheHits         int64     `json:"cache_hits"`
	CacheMisses       int64     `json:"cache_misses"`
	Errors            int       `json:"errors"`
}

type ProxyLog struct {
	ID         uuid.UUID `json:"id"`
	ProxyID    uuid.UUID `json:"proxy_id"`
	CapturedAt time.Time `json:"captured_at"`
	LogLevel   *string   `json:"log_level,omitempty"`
	Message    *string   `json:"message,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type AuditLog struct {
	ID         uuid.UUID  `json:"id"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   *uuid.UUID `json:"entity_id,omitempty"`
	OldValue   []byte     `json:"old_value,omitempty"`
	NewValue   []byte     `json:"new_value,omitempty"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	UserAgent  *string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Pagination is used for paginated list responses.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}
