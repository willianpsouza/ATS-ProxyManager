package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
)

type ProxyService struct {
	proxies    *repository.ProxyRepo
	proxyStats *repository.ProxyStatsRepo
	proxyLogs  *repository.ProxyLogsRepo
	configs    *repository.ConfigRepo
	audit      *repository.AuditRepo
}

func NewProxyService(
	proxies *repository.ProxyRepo,
	proxyStats *repository.ProxyStatsRepo,
	proxyLogs *repository.ProxyLogsRepo,
	configs *repository.ConfigRepo,
	audit *repository.AuditRepo,
) *ProxyService {
	return &ProxyService{
		proxies:    proxies,
		proxyStats: proxyStats,
		proxyLogs:  proxyLogs,
		configs:    configs,
		audit:      audit,
	}
}

type ProxyListItem struct {
	ID                string                           `json:"id"`
	Hostname          string                           `json:"hostname"`
	Config            *ProxyConfigRef                  `json:"config,omitempty"`
	IsOnline          bool                             `json:"is_online"`
	LastSeen          *time.Time                       `json:"last_seen,omitempty"`
	RegisteredAt      time.Time                        `json:"registered_at"`
	CurrentConfigHash *string                          `json:"current_config_hash,omitempty"`
	Stats             *repository.ProxyStatsSummary    `json:"stats,omitempty"`
}

type ProxyConfigRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProxyListResponse struct {
	Data    []ProxyListItem `json:"data"`
	Summary ProxySummary    `json:"summary"`
}

type ProxySummary struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

func (s *ProxyService) List(ctx context.Context) (*ProxyListResponse, error) {
	proxies, err := s.proxies.List(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]ProxyListItem, 0, len(proxies))
	online := 0
	for _, p := range proxies {
		item := ProxyListItem{
			ID:                p.ID.String(),
			Hostname:          p.Hostname,
			IsOnline:          p.IsOnline,
			LastSeen:          p.LastSeen,
			RegisteredAt:      p.RegisteredAt,
			CurrentConfigHash: p.CurrentConfigHash,
		}

		if p.ConfigID != nil {
			cfg, err := s.configs.GetByID(ctx, *p.ConfigID)
			if err == nil {
				item.Config = &ProxyConfigRef{ID: cfg.ID.String(), Name: cfg.Name}
			}
		}

		stats, err := s.proxyStats.SummaryForProxy(ctx, p.ID)
		if err == nil {
			item.Stats = stats
		}

		if p.IsOnline {
			online++
		}
		items = append(items, item)
	}

	return &ProxyListResponse{
		Data: items,
		Summary: ProxySummary{
			Total:   len(proxies),
			Online:  online,
			Offline: len(proxies) - online,
		},
	}, nil
}

type ProxyDetail struct {
	ProxyListItem
	StatsHistory []domain.ProxyStat `json:"stats_history"`
}

func (s *ProxyService) GetByID(ctx context.Context, id uuid.UUID) (*ProxyDetail, error) {
	proxy, err := s.proxies.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	detail := &ProxyDetail{
		ProxyListItem: ProxyListItem{
			ID:                proxy.ID.String(),
			Hostname:          proxy.Hostname,
			IsOnline:          proxy.IsOnline,
			LastSeen:          proxy.LastSeen,
			RegisteredAt:      proxy.RegisteredAt,
			CurrentConfigHash: proxy.CurrentConfigHash,
		},
	}

	if proxy.ConfigID != nil {
		cfg, err := s.configs.GetByID(ctx, *proxy.ConfigID)
		if err == nil {
			detail.Config = &ProxyConfigRef{ID: cfg.ID.String(), Name: cfg.Name}
		}
	}

	stats, err := s.proxyStats.SummaryForProxy(ctx, proxy.ID)
	if err == nil {
		detail.Stats = stats
	}

	history, err := s.proxyStats.ListByProxy(ctx, proxy.ID, 60)
	if err == nil {
		detail.StatsHistory = history
	}
	if detail.StatsHistory == nil {
		detail.StatsHistory = []domain.ProxyStat{}
	}

	return detail, nil
}

func (s *ProxyService) StartLogCapture(ctx context.Context, id uuid.UUID, durationMinutes int, userID uuid.UUID, ip, ua string) (time.Time, error) {
	if durationMinutes <= 0 || durationMinutes > 5 {
		return time.Time{}, fmt.Errorf("%w: duration must be between 1 and 5 minutes", domain.ErrBadRequest)
	}

	until := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
	if err := s.proxies.SetCaptureLogsUntil(ctx, id, until); err != nil {
		return time.Time{}, err
	}

	_ = s.audit.Create(ctx, &domain.AuditLog{
		UserID:     &userID,
		Action:     "proxy.capture_logs",
		EntityType: "proxy",
		EntityID:   &id,
		IPAddress:  &ip,
		UserAgent:  &ua,
	})

	return until, nil
}

func (s *ProxyService) GetLogs(ctx context.Context, id uuid.UUID) ([]domain.ProxyLog, error) {
	return s.proxyLogs.ListByProxy(ctx, id)
}
