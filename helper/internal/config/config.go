package config

import "time"

// Config armazena as configurações do helper
type Config struct {
	// Backend
	BackendURL string
	ConfigID   string
	Hostname   string

	// Sync
	SyncInterval time.Duration

	// ATS
	ConfigDir string

	// Logging
	LogLevel string
}

// Backoff configurações de retry
type BackoffConfig struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
}

// DefaultBackoff retorna configuração padrão de backoff
// Inicia em 30s, dobra a cada retry, máximo 3 minutos
func DefaultBackoff() BackoffConfig {
	return BackoffConfig{
		InitialInterval: 30 * time.Second,
		MaxInterval:     3 * time.Minute,
		Multiplier:      2.0,
	}
}
