package ats

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	stdsync "sync"
	"time"

	"github.com/ats-proxy/proxy-helper/internal/sync"
)

// Manager gerencia configuração e operações do ATS
type Manager struct {
	configDir string
	hashFile  string

	// Para captura de logs
	logBuffer []sync.LogLine
	logMu     stdsync.Mutex
}

// NewManager cria um novo Manager
func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
		hashFile:  filepath.Join(configDir, ".config_hash"),
		logBuffer: make([]sync.LogLine, 0),
	}
}

// ========== Config Management ==========

// ApplyConfig escreve os arquivos de configuração
func (m *Manager) ApplyConfig(cfg *sync.ConfigFiles) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Escreve parent.config
	if cfg.ParentConfig != "" {
		path := filepath.Join(m.configDir, "parent.config")
		if err := m.writeFile(path, cfg.ParentConfig); err != nil {
			return fmt.Errorf("erro ao escrever parent.config: %w", err)
		}
	}

	// Escreve sni.yaml
	if cfg.SNIYaml != "" {
		path := filepath.Join(m.configDir, "sni.yaml")
		if err := m.writeFile(path, cfg.SNIYaml); err != nil {
			return fmt.Errorf("erro ao escrever sni.yaml: %w", err)
		}
	}

	// Escreve ip_allow.yaml (opcional)
	if cfg.IPAllowYaml != "" {
		path := filepath.Join(m.configDir, "ip_allow.yaml")
		if err := m.writeFile(path, cfg.IPAllowYaml); err != nil {
			return fmt.Errorf("erro ao escrever ip_allow.yaml: %w", err)
		}
	}

	return nil
}

// writeFile escreve conteúdo em um arquivo de forma atômica
func (m *Manager) writeFile(path, content string) error {
	// Escreve em arquivo temporário primeiro
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
		return err
	}

	// Move para destino final (atômico)
	return os.Rename(tmpPath, path)
}

// ========== Reload ==========

// Reload executa traffic_ctl config reload
func (m *Manager) Reload() error {
	cmd := exec.Command("traffic_ctl", "config", "reload")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("erro ao recarregar ATS: %w, output: %s", err, string(output))
	}
	return nil
}

// ========== Hash Management ==========

// GetCurrentHash retorna o hash da configuração atual
func (m *Manager) GetCurrentHash() string {
	data, err := os.ReadFile(m.hashFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SaveHash salva o hash da configuração atual
func (m *Manager) SaveHash(hash string) error {
	return os.WriteFile(m.hashFile, []byte(hash), 0644)
}

// CalculateLocalHash calcula hash dos arquivos locais
func (m *Manager) CalculateLocalHash() (string, error) {
	files := []string{
		filepath.Join(m.configDir, "parent.config"),
		filepath.Join(m.configDir, "sni.yaml"),
	}

	hasher := sha256.New()
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		hasher.Write(data)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ========== Stats Collection ==========

// CollectStats coleta métricas do ATS via traffic_ctl
func (m *Manager) CollectStats() (sync.Metrics, error) {
	metrics := sync.Metrics{}

	// Conexões ativas de clientes
	val, err := m.getMetric("proxy.process.http.current_client_connections")
	if err == nil {
		metrics.ActiveConnections = val
	}

	// Total de conexões de clientes
	val, err = m.getMetric("proxy.process.http.total_client_connections")
	if err == nil {
		metrics.TotalConnections = val
	}

	// Cache hits
	val, err = m.getMetric("proxy.process.cache.ram_cache.hits")
	if err == nil {
		metrics.CacheHits = val
	}

	// Cache misses
	val, err = m.getMetric("proxy.process.cache.ram_cache.misses")
	if err == nil {
		metrics.CacheMisses = val
	}

	// Erros de conexão
	val, err = m.getMetric("proxy.process.http.err_connect_fail_count_stat")
	if err == nil {
		metrics.Errors = val
	}

	return metrics, nil
}

// getMetric obtém uma métrica específica do ATS
func (m *Manager) getMetric(name string) (int64, error) {
	cmd := exec.Command("traffic_ctl", "metric", "get", name)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// Formato: "proxy.process.http... 12345"
	parts := strings.Fields(string(output))
	if len(parts) < 2 {
		return 0, fmt.Errorf("formato inesperado: %s", string(output))
	}

	return strconv.ParseInt(parts[len(parts)-1], 10, 64)
}

// ========== Debug/Logs ==========

// EnableDebug habilita logs de debug do ATS
func (m *Manager) EnableDebug() error {
	if err := m.setConfig("proxy.config.diags.debug.enabled", "1"); err != nil {
		return err
	}
	return m.setConfig("proxy.config.diags.debug.tags", "parent_select")
}

// DisableDebug desabilita logs de debug do ATS
func (m *Manager) DisableDebug() error {
	return m.setConfig("proxy.config.diags.debug.enabled", "0")
}

// setConfig define uma configuração via traffic_ctl
func (m *Manager) setConfig(name, value string) error {
	cmd := exec.Command("traffic_ctl", "config", "set", name, value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("erro ao definir %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// CaptureLogs captura logs recentes e retorna
func (m *Manager) CaptureLogs() []sync.LogLine {
	m.logMu.Lock()
	defer m.logMu.Unlock()

	// Lê logs do processo ATS (docker logs ou arquivo)
	// Por simplicidade, vamos ler das últimas linhas do diags.log
	logPath := "/opt/var/log/trafficserver/diags.log"
	lines := m.readLastLines(logPath, 100)

	result := make([]sync.LogLine, 0, len(lines))
	for _, line := range lines {
		// Filtra apenas linhas relevantes
		if strings.Contains(line, "Result for") || strings.Contains(line, "parent") {
			result = append(result, sync.LogLine{
				Timestamp: time.Now(),
				Level:     "DEBUG",
				Message:   line,
			})
		}
	}

	return result
}

// readLastLines lê as últimas N linhas de um arquivo
func (m *Manager) readLastLines(path string, n int) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}

	return lines
}

// ========== Health Check ==========

// IsHealthy verifica se o ATS está funcionando
func (m *Manager) IsHealthy() bool {
	cmd := exec.Command("traffic_ctl", "status")
	err := cmd.Run()
	return err == nil
}
