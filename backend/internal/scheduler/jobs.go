package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
)

type Scheduler struct {
	pool   *pgxpool.Pool
	stop   chan struct{}
}

func New(pool *pgxpool.Pool) *Scheduler {
	return &Scheduler{
		pool: pool,
		stop: make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	go s.runProxyStatusCheck()
	go s.runLogCleanup()
	go s.runStatsCleanup()
	log.Println("Scheduler started")
}

func (s *Scheduler) Stop() {
	close(s.stop)
	log.Println("Scheduler stopped")
}

// runProxyStatusCheck marks proxies as offline if not seen in 2 minutes.
func (s *Scheduler) runProxyStatusCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			repo := repository.NewProxyRepo(s.pool)
			if err := repo.MarkOfflineStale(ctx); err != nil {
				log.Printf("Proxy status check error: %v", err)
			}
			cancel()
		}
	}
}

// runLogCleanup deletes expired logs every 5 minutes.
func (s *Scheduler) runLogCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			repo := repository.NewProxyLogsRepo(s.pool)
			if err := repo.CleanupExpired(ctx); err != nil {
				log.Printf("Log cleanup error: %v", err)
			}
			cancel()
		}
	}
}

// runStatsCleanup deletes stats older than 7 days, runs daily at 2am.
func (s *Scheduler) runStatsCleanup() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			repo := repository.NewProxyStatsRepo(s.pool)
			if err := repo.CleanupOld(ctx); err != nil {
				log.Printf("Stats cleanup error: %v", err)
			}
			cancel()
		}
	}
}
