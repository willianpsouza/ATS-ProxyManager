package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/idf-experian/idf-proxy-helper/internal/config"
)

// Client gerencia comunicação com o backend
type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	backoff    *Backoff
}

// NewClient cria um novo cliente de sincronização
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		backoff: NewBackoff(config.DefaultBackoff()),
	}
}

// ========== Request/Response Types ==========

// RegisterRequest payload de registro
type RegisterRequest struct {
	Hostname string `json:"hostname"`
	ConfigID string `json:"config_id"`
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

// Register registra este proxy no backend
func (c *Client) Register(ctx context.Context) error {
	req := RegisterRequest{
		Hostname: c.cfg.Hostname,
		ConfigID: c.cfg.ConfigID,
	}

	var resp RegisterResponse
	err := c.doRequest(ctx, "POST", "/sync/register", req, &resp)
	if err != nil {
		return fmt.Errorf("erro ao registrar: %w", err)
	}

	log.Printf("Registrado com sucesso (proxy_id: %s)", resp.ProxyID)
	return nil
}

// GetConfig busca configuração do backend
func (c *Client) GetConfig(ctx context.Context, currentHash string) (*ConfigResponse, error) {
	// Constrói URL com query params
	params := url.Values{}
	params.Add("hostname", c.cfg.Hostname)
	params.Add("hash", currentHash)

	var resp ConfigResponse
	err := c.doRequestWithRetry(ctx, "GET", "/sync?"+params.Encode(), nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar config: %w", err)
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

// doRequest executa uma requisição HTTP simples
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	return c.doRequestInternal(ctx, method, path, body, response)
}

// doRequestWithRetry executa uma requisição HTTP com retry e backoff
func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	var lastErr error

	for {
		err := c.doRequestInternal(ctx, method, path, body, response)
		if err == nil {
			// Sucesso - reset backoff
			c.backoff.Reset()
			return nil
		}

		lastErr = err
		waitTime := c.backoff.Next()

		log.Printf("Erro na requisição: %v. Retry em %s", err, waitTime)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continua para próxima tentativa
		}
	}
}

func (c *Client) doRequestInternal(ctx context.Context, method, path string, body interface{}, response interface{}) error {
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
	req.Header.Set("User-Agent", "idf-proxy-helper/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro na requisição: %w", err)
	}
	defer resp.Body.Close()

	// Lê body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("erro ao ler response: %w", err)
	}

	// Verifica status
	if resp.StatusCode >= 400 {
		return fmt.Errorf("erro HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	// Deserializa response se necessário
	if response != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("erro ao deserializar response: %w", err)
		}
	}

	return nil
}
