package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ats-proxy/proxy-helper/internal/ats"
	"github.com/ats-proxy/proxy-helper/internal/config"
	helpsync "github.com/ats-proxy/proxy-helper/internal/sync"
)

var (
	version = "1.0.0"
	commit  = "dev"
)

const (
	helloInterval    = 10 * time.Second
	registerInterval = 10 * time.Second
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

	if *showVersion {
		log.Printf("proxy-helper version %s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	if *backendURL == "" {
		log.Fatal("--backend-url é obrigatório")
	}
	if *configID == "" {
		log.Fatal("--config-id é obrigatório")
	}

	if *hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			log.Fatalf("Erro ao obter hostname: %v", err)
		}
		*hostname = h
	}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Recebido sinal %v, encerrando...", sig)
		cancel()
	}()

	syncClient := helpsync.NewClient(cfg)
	atsManager := ats.NewManager(cfg.ConfigDir)

	// Fase 1: Aguardar registro no backend (retry constante a cada 10s)
	if !waitForRegister(ctx, syncClient) {
		return // contexto cancelado
	}

	// Fase 2: Hello loop (goroutine) + Sync loop
	var connected atomic.Bool
	connected.Store(true) // acabou de registrar, está conectado

	go helloLoop(ctx, syncClient, &connected)

	// Sync loop
	ticker := time.NewTicker(cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Encerrando helper...")
			return

		case <-ticker.C:
			if !connected.Load() {
				log.Println("Sem conexão com o backend, aguardando reconexão...")
				continue
			}
			doSync(ctx, syncClient, atsManager, &connected)
		}
	}
}

// waitForRegister bloqueia até conseguir registrar no backend.
// Tenta a cada 10s com timeout de 4s por tentativa.
func waitForRegister(ctx context.Context, client *helpsync.Client) bool {
	log.Println("Aguardando registro no backend...")

	// Tenta imediatamente primeiro
	if err := client.Register(ctx); err == nil {
		return true
	}

	ticker := time.NewTicker(registerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if err := client.Register(ctx); err != nil {
				log.Printf("Registro falhou: %v — tentando novamente em %s", err, registerInterval)
				continue
			}
			return true
		}
	}
}

// helloLoop pinga /health a cada 10s para manter o estado de conectividade.
// Quando perde conexão, marca connected=false. Quando recupera, re-registra
// e marca connected=true.
func helloLoop(ctx context.Context, client *helpsync.Client, connected *atomic.Bool) {
	ticker := time.NewTicker(helloInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := client.Hello(ctx)
			if err != nil {
				if connected.Load() {
					log.Printf("Conexão com backend perdida: %v", err)
					connected.Store(false)
				}
				continue
			}

			// Backend acessível
			if !connected.Load() {
				log.Println("Conexão com backend restaurada, re-registrando...")
				if err := client.Register(ctx); err != nil {
					log.Printf("Re-registro falhou: %v", err)
					continue // mantém desconectado até conseguir registrar
				}
				connected.Store(true)
				log.Println("Re-registrado com sucesso, retomando sync")
			}
		}
	}
}

func doSync(ctx context.Context, client *helpsync.Client, ats *ats.Manager, connected *atomic.Bool) {
	currentHash := ats.GetCurrentHash()

	resp, err := client.GetConfig(ctx, currentHash)
	if err != nil {
		// Se 404, proxy sumiu do backend — marcar desconectado para forçar re-registro via hello
		if helpsync.IsHTTPStatus(err, 404) {
			log.Println("Proxy não encontrado no backend (404), aguardando re-registro...")
			connected.Store(false)
			return
		}
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

	if err := ats.ApplyConfig(resp.Config); err != nil {
		log.Printf("ERROR: Erro ao aplicar config: %v", err)
		client.Ack(ctx, resp.Hash, "error", err.Error())
		return
	}

	if err := ats.Reload(); err != nil {
		log.Printf("ERROR: Erro ao recarregar ATS: %v", err)
		client.Ack(ctx, resp.Hash, "error", err.Error())
		return
	}

	ats.SaveHash(resp.Hash)

	if err := client.Ack(ctx, resp.Hash, "ok", ""); err != nil {
		log.Printf("WARN: Erro ao confirmar config: %v", err)
	}

	log.Printf("Config aplicada com sucesso (hash: %s)", resp.Hash)

	sendStats(ctx, client, ats)
}

func sendStats(ctx context.Context, client *helpsync.Client, atsManager *ats.Manager) {
	stats, err := atsManager.CollectStats()
	if err != nil {
		log.Printf("WARN: Erro ao coletar stats: %v", err)
		return
	}

	if err := client.SendStats(ctx, stats); err != nil {
		log.Printf("WARN: Erro ao enviar stats: %v", err)
	}
}

func captureAndSendLogs(ctx context.Context, client *helpsync.Client, atsManager *ats.Manager, until time.Time) {
	log.Printf("Iniciando captura de logs até %s", until.Format(time.RFC3339))

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
