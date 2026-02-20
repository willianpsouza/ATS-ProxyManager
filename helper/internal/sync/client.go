package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/ats-proxy/proxy-helper/internal/config"
)

// Client gerencia comunicação com o backend
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	ctrlClient *http.Client // timeout curto para hello/register
	backoff    *Backoff
	proxyID    string // ID retornado no primeiro registro, usado para re-registro
}

// NewClient cria um novo cliente de sincronização
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		ctrlClient: &http.Client{
			Timeout: 4 * time.Second,
		},
		backoff: NewBackoff(config.DefaultBackoff()),
	}
}

// ========== Request/Response Types ==========

// RegisterRequest payload de registro
type RegisterRequest struct {
	Hostname string `json:"hostname"`
	ConfigID string `json:"config_id"`
	ProxyID  string `json:"proxy_id,omitempty"`
}

// RegisterResponse resposta do registro
type RegisterResponse struct {
	ProxyID  string `json:"proxy_id"`
	ConfigID string `json:"config_id"`
	Status   string `json:"status"`
}

// ConfigResponse resposta do sync
type ConfigResponse struct {
	Unchanged    bool         `json:"unchanged"`
	Hash         string       `json:"hash,omitempty"`
	Config       *ConfigFiles `json:"config,omitempty"`
	CaptureLogs  bool         `json:"capture_logs"`
	CaptureUntil time.Time    `json:"capture_until,omitempty"`
}

// ConfigFiles arquivos de configuração
type ConfigFiles struct {
	ParentConfig string `json:"parent_config"`
	SNIYaml      string `json:"sni_yaml"`
	IPAllowYaml  string `json:"ip_allow_yaml,omitempty"`
}

// AckRequest confirmação de aplicação
type AckRequest struct {
	Hostname string `json:"hostname"`
	Hash     string `json:"hash"`
	Status   string `json:"status"` // "ok" ou "error"
	Message  string `json:"message,omitempty"`
}

// StatsRequest métricas do proxy
type StatsRequest struct {
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
	Metrics   Metrics   `json:"metrics"`
}

// Metrics métricas coletadas do ATS
type Metrics struct {
	ActiveConnections int64 `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	CacheHits         int64 `json:"cache_hits"`
	CacheMisses       int64 `json:"cache_misses"`
	Errors            int64 `json:"errors"`

	// Métricas expandidas
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

// LogsRequest logs capturados
type LogsRequest struct {
	Hostname string    `json:"hostname"`
	Lines    []LogLine `json:"lines"`
}

// LogLine linha de log
type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// ========== Client Methods ==========

// Hello verifica conectividade com o backend via /health (timeout 4s)
func (c *Client) Hello(ctx context.Context) error {
	fullURL := c.cfg.BackendURL + "/api/v1/health"

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar request: %w", err)
	}

	resp, err := c.ctrlClient.Do(req)
	if err != nil {
		return fmt.Errorf("backend unreachable: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("backend unhealthy: HTTP %d", resp.StatusCode)
	}

	return nil
}

// Register registra este proxy no backend (timeout 4s).
// Envia o proxy_id obtido em registros anteriores para permitir re-registro
// do mesmo proxy sem conflito com proxies online.
func (c *Client) Register(ctx context.Context) error {
	req := RegisterRequest{
		Hostname: c.cfg.Hostname,
		ConfigID: c.cfg.ConfigID,
		ProxyID:  c.proxyID,
	}

	var resp RegisterResponse
	err := c.doRequestWith(ctx, c.ctrlClient, "POST", "/sync/register", req, &resp)
	if err != nil {
		return err
	}

	c.proxyID = resp.ProxyID
	log.Printf("Registrado com sucesso (proxy_id: %s)", resp.ProxyID)
	return nil
}

// GetConfig busca configuração do backend
func (c *Client) GetConfig(ctx context.Context, currentHash string) (*ConfigResponse, error) {
	params := url.Values{}
	params.Add("hostname", c.cfg.Hostname)
	params.Add("hash", currentHash)

	var resp ConfigResponse
	err := c.doRequest(ctx, "GET", "/sync?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// Ack confirma aplicação da configuração
func (c *Client) Ack(ctx context.Context, hash, status, message string) error {
	req := AckRequest{
		Hostname: c.cfg.Hostname,
		Hash:     hash,
		Status:   status,
		Message:  message,
	}

	return c.doRequest(ctx, "POST", "/sync/ack", req, nil)
}

// SendStats envia métricas do proxy
func (c *Client) SendStats(ctx context.Context, metrics Metrics) error {
	req := StatsRequest{
		Hostname:  c.cfg.Hostname,
		Timestamp: time.Now(),
		Metrics:   metrics,
	}

	return c.doRequest(ctx, "POST", "/sync/stats", req, nil)
}

// SendLogs envia logs capturados
func (c *Client) SendLogs(ctx context.Context, lines []LogLine) error {
	req := LogsRequest{
		Hostname: c.cfg.Hostname,
		Lines:    lines,
	}

	return c.doRequest(ctx, "POST", "/sync/logs", req, nil)
}

// ========== HTTP Helpers ==========

// doRequest executa uma requisição HTTP simples com o client padrão (30s timeout)
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	return c.doRequestWith(ctx, c.httpClient, method, path, body, response)
}

// doRequestWith executa uma requisição HTTP com um http.Client específico
func (c *Client) doRequestWith(ctx context.Context, hc *http.Client, method, path string, body interface{}, response interface{}) error {
	fullURL := c.cfg.BackendURL + "/api/v1" + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("erro ao serializar body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("erro ao criar request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "proxy-helper/1.0")

	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao ler response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if response != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("erro ao deserializar response: %w", err)
		}
	}

	return nil
}

// HTTPError representa um erro HTTP com status code acessível
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("erro HTTP %d: %s", e.StatusCode, e.Body)
}

// IsHTTPStatus verifica se um erro é um HTTPError com o status code dado
func IsHTTPStatus(err error, code int) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == code
	}
	return false
}
