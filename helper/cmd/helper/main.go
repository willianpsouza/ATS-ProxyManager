package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ats-proxy/proxy-helper/internal/ats"
	"github.com/ats-proxy/proxy-helper/internal/config"
	"github.com/ats-proxy/proxy-helper/internal/sync"
)

var (
	version = "1.0.0"
	commit  = "dev"
)

func main() {
	// Flags
	backendURL := flag.String("backend-url", "", "URL do backend API (obrigatório)")
	configID := flag.String("config-id", "", "ID da configuração (obrigatório)")
	hostname := flag.String("hostname", "", "Hostname deste proxy (default: hostname do sistema)")
	syncInterval := flag.Duration("sync-interval", 30*time.Second, "Intervalo de sincronização")
	configDir := flag.String("config-dir", "/opt/etc/trafficserver", "Diretório de configuração do ATS")
	logLevel := flag.String("log-level", "info", "Nível de log (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Mostra versão e sai")

	flag.Parse()

	// Versão
	if *showVersion {
		log.Printf("proxy-helper version %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	// Validação
	if *backendURL == "" {
		log.Fatal("--backend-url é obrigatório")
	}
	if *configID == "" {
		log.Fatal("--config-id é obrigatório")
	}

	// Hostname padrão
	if *hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			log.Fatalf("Erro ao obter hostname: %v", err)
		}
		*hostname = h
	}

	// Configuração
	cfg := &config.Config{
		BackendURL:   *backendURL,
		ConfigID:     *configID,
		Hostname:     *hostname,
		SyncInterval: *syncInterval,
		ConfigDir:    *configDir,
		LogLevel:     *logLevel,
	}

	log.Printf("Iniciando proxy-helper v%s", version)
	log.Printf("Backend: %s", cfg.BackendURL)
	log.Printf("Config ID: %s", cfg.ConfigID)
	log.Printf("Hostname: %s", cfg.Hostname)
	log.Printf("Sync Interval: %s", cfg.SyncInterval)

	// Contexto com cancelamento
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Captura sinais de término
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Recebido sinal %v, encerrando...", sig)
		cancel()
	}()

	// Cliente de sincronização
	syncClient := sync.NewClient(cfg)

	// Gerenciador do ATS
	atsManager := ats.NewManager(cfg.ConfigDir)

	// Registra no backend
	if err := syncClient.Register(ctx); err != nil {
		log.Printf("WARN: Erro ao registrar no backend: %v (continuando...)", err)
	}

	// Loop principal
	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()

	// Sync inicial
	doSync(ctx, syncClient, atsManager)

	for {
		select {
		case <-ctx.Done():
			log.Println("Encerrando helper...")
			return

		case <-ticker.C:
			doSync(ctx, syncClient, atsManager)
		}
	}
}

func doSync(ctx context.Context, client *sync.Client, ats *ats.Manager) {
	// Obtém hash atual
	currentHash := ats.GetCurrentHash()

	// Busca config no backend
	resp, err := client.GetConfig(ctx, currentHash)
	if err != nil {
		log.Printf("WARN: Erro ao buscar config: %v", err)
		return
	}

	// Verifica se há captura de logs ativa
	if resp.CaptureLogs {
		go captureAndSendLogs(ctx, client, ats, resp.CaptureUntil)
	}

	// Se não mudou, apenas envia stats
	if resp.Unchanged {
		sendStats(ctx, client, ats)
		return
	}

	log.Printf("Config alterada (hash: %s -> %s), aplicando...", currentHash, resp.Hash)

	// Aplica nova config
	if err := ats.ApplyConfig(resp.Config); err != nil {
		log.Printf("ERROR: Erro ao aplicar config: %v", err)
		client.Ack(ctx, resp.Hash, "error", err.Error())
		return
	}

	// Reload do ATS
	if err := ats.Reload(); err != nil {
		log.Printf("ERROR: Erro ao recarregar ATS: %v", err)
		client.Ack(ctx, resp.Hash, "error", err.Error())
		return
	}

	// Salva hash
	ats.SaveHash(resp.Hash)

	// Confirma
	if err := client.Ack(ctx, resp.Hash, "ok", ""); err != nil {
		log.Printf("WARN: Erro ao confirmar config: %v", err)
	}

	log.Printf("Config aplicada com sucesso (hash: %s)", resp.Hash)

	// Envia stats
	sendStats(ctx, client, ats)
}

func sendStats(ctx context.Context, client *sync.Client, atsManager *ats.Manager) {
	stats, err := atsManager.CollectStats()
	if err != nil {
		log.Printf("WARN: Erro ao coletar stats: %v", err)
		return
	}

	if err := client.SendStats(ctx, stats); err != nil {
		log.Printf("WARN: Erro ao enviar stats: %v", err)
	}
}

func captureAndSendLogs(ctx context.Context, client *sync.Client, atsManager *ats.Manager, until time.Time) {
	log.Printf("Iniciando captura de logs até %s", until.Format(time.RFC3339))

	// Habilita debug no ATS
	if err := atsManager.EnableDebug(); err != nil {
		log.Printf("WARN: Erro ao habilitar debug: %v", err)
		return
	}

	defer func() {
		if err := atsManager.DisableDebug(); err != nil {
			log.Printf("WARN: Erro ao desabilitar debug: %v", err)
		}
		log.Println("Captura de logs finalizada")
	}()

	// Loop de captura
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if time.Now().After(until) {
				return
			}

			logs := atsManager.CaptureLogs()
			if len(logs) > 0 {
				if err := client.SendLogs(ctx, logs); err != nil {
					log.Printf("WARN: Erro ao enviar logs: %v", err)
				}
			}
		}
	}
}
