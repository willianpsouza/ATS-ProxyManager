package sync

import (
	"sync"
	"time"

	"github.com/idf-experian/idf-proxy-helper/internal/config"
)

// Backoff implementa exponential backoff
type Backoff struct {
	cfg     config.BackoffConfig
	current time.Duration
	mu      sync.Mutex
}

// NewBackoff cria um novo Backoff
func NewBackoff(cfg config.BackoffConfig) *Backoff {
	return &Backoff{
		cfg:     cfg,
		current: cfg.InitialInterval,
	}
}

// Next retorna o próximo intervalo de espera e incrementa o backoff
func (b *Backoff) Next() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Salva valor atual para retornar
	wait := b.current

	// Calcula próximo valor
	next := time.Duration(float64(b.current) * b.cfg.Multiplier)
	if next > b.cfg.MaxInterval {
		next = b.cfg.MaxInterval
	}
	b.current = next

	return wait
}

// Reset volta o backoff para o valor inicial
func (b *Backoff) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.current = b.cfg.InitialInterval
}

// Current retorna o intervalo atual sem incrementar
func (b *Backoff) Current() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.current
}
