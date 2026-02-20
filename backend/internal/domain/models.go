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
	ID            uuid.UUID    `json:"id"`
	Name          string       `json:"name"`
	Description   *string      `json:"description,omitempty"`
	Status        ConfigStatus `json:"status"`
	Version       int          `json:"version"`
	DefaultAction RuleAction   `json:"default_action"`

	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ModifiedBy  *uuid.UUID `json:"modified_by,omitempty"`
	ModifiedAt  time.Time  `json:"modified_at"`
	SubmittedBy *uuid.UUID `json:"submitted_by,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	ApprovedBy  *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	ConfigHash  *string    `json:"config_hash,omitempty"`
	ProxyCount  int        `json:"proxy_count,omitempty"`
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

type ClientACLRule struct {
	ID        uuid.UUID `json:"id"`
	ConfigID  uuid.UUID `json:"config_id"`
	CIDR      string    `json:"cidr"`
	Action    ACLAction `json:"action"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
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
	RegisteredIP      *string    `json:"registered_ip,omitempty"`
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

	// Métricas expandidas
	TotalRequests     int64 `json:"total_requests"`
	ConnectRequests   int64 `json:"connect_requests"`
	Responses2xx      int64 `json:"responses_2xx"`
	Responses3xx      int64 `json:"responses_3xx"`
	Responses4xx      int64 `json:"responses_4xx"`
	Responses5xx      int64 `json:"responses_5xx"`
	ErrConnectFail    int   `json:"err_connect_fail"`
	ErrClientAbort    int   `json:"err_client_abort"`
	BrokenServerConns int   `json:"broken_server_conns"`
	BytesIn           int64 `json:"bytes_in"`
	BytesOut          int64 `json:"bytes_out"`
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
