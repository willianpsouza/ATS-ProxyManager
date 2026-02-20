package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
	"github.com/redis/go-redis/v9"
)

type SyncService struct {
	proxies      *repository.ProxyRepo
	configs      *repository.ConfigRepo
	configProxy  *repository.ConfigProxyRepo
	proxyStats   *repository.ProxyStatsRepo
	proxyLogs    *repository.ProxyLogsRepo
	configSvc    *ConfigService
	rdb          *redis.Client
}

func NewSyncService(
	proxies *repository.ProxyRepo,
	configs *repository.ConfigRepo,
	configProxy *repository.ConfigProxyRepo,
	proxyStats *repository.ProxyStatsRepo,
	proxyLogs *repository.ProxyLogsRepo,
	configSvc *ConfigService,
	rdb *redis.Client,
) *SyncService {
	return &SyncService{
		proxies:     proxies,
		configs:     configs,
		configProxy: configProxy,
		proxyStats:  proxyStats,
		proxyLogs:   proxyLogs,
		configSvc:   configSvc,
		rdb:         rdb,
	}
}

// RegisterRequest mirrors helper's RegisterRequest
type RegisterRequest struct {
	Hostname string `json:"hostname"`
	ConfigID string `json:"config_id"`
	ProxyID  string `json:"proxy_id,omitempty"`
	RemoteIP string `json:"-"` // extracted from request by handler
}

type RegisterResponse struct {
	ProxyID  string `json:"proxy_id"`
	ConfigID string `json:"config_id"`
	Status   string `json:"status"`
}

func (s *SyncService) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	if req.Hostname == "" {
		return nil, fmt.Errorf("%w: hostname is required", domain.ErrBadRequest)
	}

	// Check if a proxy with this hostname already exists
	existing, err := s.proxies.GetByHostname(ctx, req.Hostname)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("register proxy: %w", err)
	}

	if existing != nil {
		if existing.IsOnline {
			// Proxy is online — only allow re-register if same identity (IP or proxy_id)
			sameIP := existing.RegisteredIP != nil && *existing.RegisteredIP == req.RemoteIP
			sameID := req.ProxyID != "" && req.ProxyID == existing.ID.String()

			if !sameIP && !sameID {
				return nil, fmt.Errorf("%w: hostname '%s' is already registered by an active proxy", domain.ErrConflict, req.Hostname)
			}
		}

		// Same proxy re-registering (recovery) or offline proxy — update IP and mark online
		_ = s.proxies.UpdateRegisteredIP(ctx, existing.ID, req.RemoteIP)
		_ = s.proxies.UpdateLastSeen(ctx, existing.ID)

		configID := ""
		if existing.ConfigID != nil {
			configID = existing.ConfigID.String()
		}

		return &RegisterResponse{
			ProxyID:  existing.ID.String(),
			ConfigID: configID,
			Status:   "registered",
		}, nil
	}

	// New proxy — create it
	proxy := &domain.Proxy{
		Hostname:     req.Hostname,
		RegisteredIP: &req.RemoteIP,
	}

	if req.ConfigID != "" {
		if cfgID, err := uuid.Parse(req.ConfigID); err == nil {
			proxy.ConfigID = &cfgID
		}
	}

	if err := s.proxies.Create(ctx, proxy); err != nil {
		return nil, fmt.Errorf("register proxy: %w", err)
	}

	_ = s.proxies.UpdateLastSeen(ctx, proxy.ID)

	configID := ""
	if proxy.ConfigID != nil {
		configID = proxy.ConfigID.String()
	}

	return &RegisterResponse{
		ProxyID:  proxy.ID.String(),
		ConfigID: configID,
		Status:   "registered",
	}, nil
}

// ConfigResponse mirrors helper's ConfigResponse
type ConfigResponse struct {
	Unchanged    bool         `json:"unchanged"`
	Hash         string       `json:"hash,omitempty"`
	Config       *ConfigFiles `json:"config,omitempty"`
	CaptureLogs  bool         `json:"capture_logs"`
	CaptureUntil *time.Time   `json:"capture_until,omitempty"`
}

type ConfigFiles struct {
	ParentConfig string `json:"parent_config"`
	SNIYaml      string `json:"sni_yaml"`
	IPAllowYaml  string `json:"ip_allow_yaml,omitempty"`
}

func (s *SyncService) GetConfig(ctx context.Context, hostname, currentHash string) (*ConfigResponse, error) {
	proxy, err := s.proxies.GetByHostname(ctx, hostname)
	if err != nil {
		return nil, fmt.Errorf("proxy not found: %w", err)
	}

	// Update last_seen
	_ = s.proxies.UpdateLastSeen(ctx, proxy.ID)

	// Find active config for this proxy
	cfg, err := s.configs.GetActiveForProxy(ctx, hostname)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// No active config, return unchanged with capture_logs info
			resp := &ConfigResponse{Unchanged: true}
			if proxy.CaptureLogsUntil != nil && proxy.CaptureLogsUntil.After(time.Now()) {
				resp.CaptureLogs = true
				resp.CaptureUntil = proxy.CaptureLogsUntil
			}
			return resp, nil
		}
		return nil, err
	}

	// Check capture_logs
	captureLogs := false
	var captureUntil *time.Time
	if proxy.CaptureLogsUntil != nil && proxy.CaptureLogsUntil.After(time.Now()) {
		captureLogs = true
		captureUntil = proxy.CaptureLogsUntil
	}

	// Check if config has changed
	configHash := ""
	if cfg.ConfigHash != nil {
		configHash = *cfg.ConfigHash
	}

	if configHash != "" && configHash == currentHash {
		return &ConfigResponse{
			Unchanged:    true,
			CaptureLogs:  captureLogs,
			CaptureUntil: captureUntil,
		}, nil
	}

	// Generate config files
	parentConfig, sniYaml, ipAllowYaml, err := s.configSvc.GenerateConfigFiles(ctx, cfg.ID)
	if err != nil {
		return nil, fmt.Errorf("generate config files: %w", err)
	}

	// If hash was empty, compute it now
	if configHash == "" {
		configHash, err = s.configSvc.GenerateConfigHash(ctx, cfg.ID)
		if err != nil {
			return nil, err
		}
		_ = s.configs.UpdateHash(ctx, cfg.ID, configHash)
	}

	return &ConfigResponse{
		Unchanged: false,
		Hash:      configHash,
		Config: &ConfigFiles{
			ParentConfig: parentConfig,
			SNIYaml:      sniYaml,
			IPAllowYaml:  ipAllowYaml,
		},
		CaptureLogs:  captureLogs,
		CaptureUntil: captureUntil,
	}, nil
}

// AckRequest mirrors helper's AckRequest
type AckRequest struct {
	Hostname string `json:"hostname"`
	Hash     string `json:"hash"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

func (s *SyncService) Ack(ctx context.Context, req AckRequest) error {
	proxy, err := s.proxies.GetByHostname(ctx, req.Hostname)
	if err != nil {
		return fmt.Errorf("proxy not found: %w", err)
	}

	if req.Status == "ok" {
		if err := s.proxies.UpdateConfigHash(ctx, proxy.ID, req.Hash); err != nil {
			return err
		}
	}

	return nil
}

// StatsRequest mirrors helper's StatsRequest
type SyncStatsRequest struct {
	Hostname  string       `json:"hostname"`
	Timestamp time.Time    `json:"timestamp"`
	Metrics   SyncMetrics  `json:"metrics"`
}

type SyncMetrics struct {
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	CacheHits         int64 `json:"cache_hits"`
	CacheMisses       int64 `json:"cache_misses"`
	Errors            int64 `json:"errors"`

	TotalRequests     int64 `json:"total_requests"`
	ConnectRequests   int64 `json:"connect_requests"`
	Responses2xx      int64 `json:"responses_2xx"`
	Responses3xx      int64 `json:"responses_3xx"`
	Responses4xx      int64 `json:"responses_4xx"`
	Responses5xx      int64 `json:"responses_5xx"`
	ErrConnectFail    int64 `json:"err_connect_fail"`
	ErrClientAbort    int64 `json:"err_client_abort"`
	BrokenServerConns int64 `json:"broken_server_conns"`
	BytesIn           int64 `json:"bytes_in"`
	BytesOut          int64 `json:"bytes_out"`
}

func (s *SyncService) Stats(ctx context.Context, req SyncStatsRequest) error {
	proxy, err := s.proxies.GetByHostname(ctx, req.Hostname)
	if err != nil {
		return fmt.Errorf("proxy not found: %w", err)
	}

	_ = s.proxies.UpdateLastSeen(ctx, proxy.ID)

	stat := &domain.ProxyStat{
		ProxyID:           proxy.ID,
		ActiveConnections: int(req.Metrics.ActiveConnections),
		TotalConnections:  req.Metrics.TotalConnections,
		CacheHits:         req.Metrics.CacheHits,
		CacheMisses:       req.Metrics.CacheMisses,
		Errors:            int(req.Metrics.Errors),
		TotalRequests:     req.Metrics.TotalRequests,
		ConnectRequests:   req.Metrics.ConnectRequests,
		Responses2xx:      req.Metrics.Responses2xx,
		Responses3xx:      req.Metrics.Responses3xx,
		Responses4xx:      req.Metrics.Responses4xx,
		Responses5xx:      req.Metrics.Responses5xx,
		ErrConnectFail:    int(req.Metrics.ErrConnectFail),
		ErrClientAbort:    int(req.Metrics.ErrClientAbort),
		BrokenServerConns: int(req.Metrics.BrokenServerConns),
		BytesIn:           req.Metrics.BytesIn,
		BytesOut:          req.Metrics.BytesOut,
	}

	return s.proxyStats.Create(ctx, stat)
}

// LogsRequest mirrors helper's LogsRequest
type SyncLogsRequest struct {
	Hostname string        `json:"hostname"`
	Lines    []SyncLogLine `json:"lines"`
}

type SyncLogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type LogsResponse struct {
	Received        bool `json:"received"`
	ContinueCapture bool `json:"continue_capture"`
}

func (s *SyncService) Logs(ctx context.Context, req SyncLogsRequest) (*LogsResponse, error) {
	proxy, err := s.proxies.GetByHostname(ctx, req.Hostname)
	if err != nil {
		return nil, fmt.Errorf("proxy not found: %w", err)
	}

	for _, line := range req.Lines {
		level := line.Level
		msg := line.Message
		log := domain.ProxyLog{
			ProxyID:  proxy.ID,
			LogLevel: &level,
			Message:  &msg,
		}
		_ = s.proxyLogs.Create(ctx, &log)
	}

	continueCapture := false
	if proxy.CaptureLogsUntil != nil && proxy.CaptureLogsUntil.After(time.Now()) {
		continueCapture = true
	}

	return &LogsResponse{
		Received:        true,
		ContinueCapture: continueCapture,
	}, nil
}
