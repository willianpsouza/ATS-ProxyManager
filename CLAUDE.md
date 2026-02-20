# ATS Proxy Manager — Project Context

## Overview

Sistema de gerenciamento centralizado para configurações de proxies Apache Traffic Server (ATS). Composto por três partes: **Backend** (API REST em Go), **Helper** (agente que roda junto ao ATS) e **Frontend** (futuro, Next.js).

## Tech Stack

| Componente | Tecnologia |
|-----------|------------|
| Backend API | Go 1.26, chi router, pgx (PostgreSQL), golang-jwt, bcrypt, go-redis |
| Helper | Go 1.26 (stdlib only) |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Containers | Docker Compose (dev), Dockerfiles (prod) |
| Hot-reload | air (dev) |

## Project Structure

```
├── CLAUDE.md                          # Este arquivo
├── docker-compose.yml                 # Stack local: postgres, redis, backend, proxy-01
├── database/schema.sql                # Schema definitivo do PostgreSQL
├── docs/
│   ├── API_CONTRACT.md                # Contrato de endpoints REST
│   ├── ARCHITECTURE.md                # Documento de arquitetura
│   └── SEQUENCES.md                   # Diagramas de sequência (Mermaid)
├── proxy-config/parent.config         # Formato de referência ATS parent.config
├── helper/                            # Agente Go que roda junto ao ATS
│   ├── cmd/helper/main.go
│   ├── internal/
│   │   ├── ats/manager.go             # Gerencia config files, reload, stats, logs do ATS
│   │   ├── config/config.go           # Flags e configuração
│   │   └── sync/
│   │       ├── client.go              # HTTP client com retry/backoff
│   │       └── backoff.go             # Exponential backoff
│   ├── Dockerfile
│   └── go.mod                         # module github.com/ats-proxy/proxy-helper
└── backend/                           # API REST Go
    ├── cmd/server/main.go             # Entry point
    ├── internal/
    │   ├── config/config.go           # Env vars: DATABASE_URL, REDIS_URL, JWT_SECRET, PORT
    │   ├── domain/
    │   │   ├── models.go              # Structs: User, Config, DomainRule, IPRangeRule, ParentProxy, Proxy, etc.
    │   │   ├── errors.go              # ErrNotFound, ErrForbidden, ErrInvalidStatus, ErrConflict
    │   │   └── enums.go               # UserRole, ConfigStatus, RuleAction + RBAC logic
    │   ├── auth/
    │   │   ├── jwt.go                 # GenerateToken, ParseToken, HashToken (HS256, 30min access, 24h refresh)
    │   │   └── password.go            # bcrypt hash/verify
    │   ├── repository/                # Data access layer (pgx, DBTX interface, WithTx)
    │   │   ├── db.go                  # Pool init, DBTX interface, WithTx helper
    │   │   ├── user_repo.go
    │   │   ├── session_repo.go
    │   │   ├── config_repo.go         # Includes GetActiveForProxy, DeactivateOthers
    │   │   ├── domain_rule_repo.go
    │   │   ├── ip_range_rule_repo.go
    │   │   ├── parent_proxy_repo.go
    │   │   ├── proxy_repo.go          # Includes Upsert, MarkOfflineStale
    │   │   ├── config_proxy_repo.go
    │   │   ├── proxy_stats_repo.go    # Includes SummaryForProxy, CleanupOld
    │   │   ├── proxy_logs_repo.go     # Includes CleanupExpired
    │   │   └── audit_repo.go          # Filtros dinâmicos por entity/user/date
    │   ├── service/
    │   │   ├── auth_service.go        # Login, Refresh, Beacon, Logout
    │   │   ├── user_service.go        # CRUD + RBAC enforcement
    │   │   ├── config_service.go      # CRUD + state machine + config merge (parent.config/sni.yaml generation)
    │   │   ├── sync_service.go        # Register, GetConfig, Ack, Stats, Logs
    │   │   ├── proxy_service.go       # List with stats, detail, log capture
    │   │   └── audit_service.go       # List with user resolution
    │   ├── handler/
    │   │   ├── router.go              # chi router, todas as rotas montadas aqui
    │   │   ├── middleware.go          # AuthMiddleware (JWT), RequireRole, CORS, RequestID
    │   │   ├── response.go           # respondJSON, respondError, pagination helpers
    │   │   ├── auth_handler.go
    │   │   ├── user_handler.go
    │   │   ├── config_handler.go
    │   │   ├── sync_handler.go
    │   │   ├── proxy_handler.go
    │   │   └── audit_handler.go
    │   └── scheduler/jobs.go          # Goroutines: proxy offline (1min), log cleanup (5min), stats cleanup (24h)
    ├── migrations/001_initial.sql     # Cópia do schema
    ├── Dockerfile                     # Produção (multi-stage)
    ├── Dockerfile.dev                 # Dev com air hot-reload
    ├── .air.toml
    ├── go.mod                         # module github.com/ats-proxy/proxy-manager/backend
    └── go.sum
```

## API Routes

Base: `/api/v1`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | /health | No | Health check |
| POST | /auth/login | No | Login (email + password) → JWT |
| POST | /auth/refresh | No | Refresh token |
| POST | /auth/beacon | JWT | Keep-alive (30s) |
| POST | /auth/logout | JWT | Revoke session |
| GET | /users | JWT (admin+) | List users (filtro: role, page, limit) |
| POST | /users | JWT (admin+) | Create user (RBAC enforced) |
| PUT | /users/{id} | JWT (admin+) | Update user |
| DELETE | /users/{id} | JWT (root) | Soft delete user |
| GET | /configs | JWT | List configs (filtro: status) |
| POST | /configs | JWT | Create config (transactional: rules + proxies) |
| GET | /configs/{id} | JWT | Detail with domains, ip_ranges, parent_proxies, proxies |
| PUT | /configs/{id} | JWT | Update (only draft) |
| POST | /configs/{id}/submit | JWT | draft → pending_approval |
| POST | /configs/{id}/approve | JWT | pending_approval → active (same user, generates hash) |
| POST | /configs/{id}/reject | JWT | pending_approval → draft |
| GET | /proxies | JWT | List with stats summary |
| GET | /proxies/{id} | JWT | Detail with stats history |
| POST | /proxies/{id}/logs | JWT | Start log capture (1-5 min) |
| GET | /proxies/{id}/logs | JWT | Get captured logs |
| POST | /sync/register | No | Helper registers proxy |
| GET | /sync | No | Helper polls config (?hostname=X&hash=Y) |
| POST | /sync/ack | No | Helper confirms config applied |
| POST | /sync/stats | No | Helper sends metrics |
| POST | /sync/logs | No | Helper sends captured logs |
| GET | /audit | JWT (admin+) | Audit trail (filtros: entity_type, entity_id, user_id, from, to) |

## Key Architecture Decisions

- **Config State Machine**: draft → pending_approval → active. Approval must be by same user who submitted.
- **Config Merge**: `config_service.go` generates `parent.config` (CIDR→IP range format) and `sni.yaml` from DB rules.
- **Hash**: SHA256(parent_config + sni_yaml) for change detection between backend and helpers.
- **Helper Communication**: Polling-based (30s interval), no auth (controlled environment). Wire-compatible structs in `sync_service.go`.
- **RBAC**: root > admin > regular. Root creates admin/regular, admin creates regular, nobody creates root.
- **Soft Delete**: Users are deactivated (`is_active = false`), not removed.
- **Scheduler**: Background goroutines for proxy offline detection, log expiry cleanup, stats retention (7 days).

## Development

```bash
# Start full stack
docker compose up --build

# Health check
curl http://localhost:8080/api/v1/health

# Login (default root user)
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"root@proxy-manager.local","password":"changeme"}'
```

## Environment Variables (Backend)

| Variable | Default | Description |
|----------|---------|-------------|
| DATABASE_URL | postgres://proxymanager:proxymanager@localhost:5432/proxymanager?sslmode=disable | PostgreSQL connection string |
| REDIS_URL | redis://localhost:6379/0 | Redis connection string |
| JWT_SECRET | dev-secret-change-in-production | Secret for JWT signing |
| PORT | 8080 | HTTP server port |

## Database

- PostgreSQL 16 with extensions: uuid-ossp, pgcrypto
- Schema in `database/schema.sql` (auto-mounted as init script via docker-compose)
- Root user seed: `root@proxy-manager.local` / `changeme`
- Tables: users, sessions, configs, domain_rules, ip_range_rules, parent_proxies, proxies, config_proxies, proxy_stats, proxy_logs, audit_logs
- Views: v_proxies_with_stats, v_configs_summary, v_dashboard_summary
- Functions: update_updated_at (trigger), calculate_config_hash, update_proxy_online_status, cleanup_expired_logs, cleanup_old_stats

## What's NOT Implemented Yet

- **Frontend** (Next.js) — planned but not started
- **Unit/integration tests** for the backend
- **Rate limiting** on auth endpoints
- **Webhook/SSE** for real-time updates to frontend
- **TLS** termination (expected to be handled by reverse proxy in production)
