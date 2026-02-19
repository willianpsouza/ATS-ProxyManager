# IDF Proxy Manager - Documento de Arquitetura

**Versão**: 1.0  
**Data**: Fevereiro 2025  
**Status**: Em Refinamento

---

## 1. Visão Geral

Sistema de gerenciamento centralizado para configurações dos proxies IDF Local Proxy, composto por:

| Componente | Tecnologia | Função |
|------------|------------|--------|
| **Frontend** | Nuxt.js | Interface de gerenciamento |
| **Backend** | Go / Node.js | API REST + lógica de merge |
| **Helper** | Go | Sincroniza configs nos proxies |
| **Database** | PostgreSQL | Persistência |

---

## 2. Arquitetura Geral

```mermaid
flowchart TB
    subgraph users["👥 Usuários"]
        admin["🔑 Admin/Root<br/>Full Access"]
        regular["👤 Regular<br/>Read-Only"]
    end
    
    subgraph frontend["🖥️ Frontend (Nuxt.js)"]
        ui["Dashboard + CRUD"]
        auth_ui["Login/Auth"]
    end
    
    subgraph backend["⚙️ Backend (Go)"]
        api["REST API"]
        auth["Auth (JWT)"]
        merger["Config Merger"]
        approval["Approval Engine"]
        audit["Audit Log"]
        db[(PostgreSQL)]
    end
    
    subgraph proxies["🔶 Proxy Fleet"]
        subgraph p1["Proxy 1"]
            h1["Helper"]
            ats1["ATS"]
        end
        subgraph p2["Proxy 2"]
            h2["Helper"]
            ats2["ATS"]
        end
        subgraph pn["Proxy N"]
            hn["Helper"]
            atsn["ATS"]
        end
    end
    
    users --> frontend
    frontend -->|"JWT Auth"| backend
    
    h1 & h2 & hn -->|"Polling 30s<br/>(sem auth)"| api
    api --> merger --> db
    api --> approval --> audit --> db
    
    h1 --> ats1
    h2 --> ats2
    hn --> atsn
```

---

## 3. Modelo de Dados

### 3.1 Entidades

```mermaid
erDiagram
    USER ||--o{ AUDIT_LOG : creates
    USER ||--o{ CONFIG : modifies
    USER ||--o{ CONFIG : approves
    
    CONFIG ||--o{ CONFIG_PROXY : has
    PROXY ||--o{ CONFIG_PROXY : belongs_to
    PROXY ||--o{ PROXY_STATUS : reports
    
    CONFIG ||--o{ DOMAIN_RULE : contains
    CONFIG ||--o{ IP_RANGE_RULE : contains
    CONFIG ||--o{ PARENT_PROXY : contains
    
    USER {
        uuid id PK
        string username
        string email
        string password_hash
        enum role "root|admin|regular"
        timestamp created_at
        timestamp last_login
    }
    
    CONFIG {
        uuid id PK
        string name
        string description
        enum status "draft|pending_approval|approved|active"
        uuid modified_by FK
        uuid approved_by FK
        timestamp modified_at
        timestamp approved_at
        int version
    }
    
    PROXY {
        uuid id PK
        string hostname
        string config_id FK
        timestamp last_seen
        timestamp registered_at
        string current_config_hash
        bool is_online
    }
    
    CONFIG_PROXY {
        uuid config_id FK
        uuid proxy_id FK
    }
    
    DOMAIN_RULE {
        uuid id PK
        uuid config_id FK
        string domain
        enum action "direct|parent"
        int priority
    }
    
    IP_RANGE_RULE {
        uuid id PK
        uuid config_id FK
        string cidr
        enum action "direct|parent"
        int priority
    }
    
    PARENT_PROXY {
        uuid id PK
        uuid config_id FK
        string address
        int port
        int priority
        bool enabled
    }
    
    PROXY_STATUS {
        uuid id PK
        uuid proxy_id FK
        timestamp collected_at
        bigint total_connections
        bigint cache_hits
        bigint cache_misses
        int active_connections
    }
    
    AUDIT_LOG {
        uuid id PK
        uuid user_id FK
        string action
        string entity_type
        uuid entity_id
        jsonb old_value
        jsonb new_value
        timestamp created_at
        string ip_address
    }
```

### 3.2 Status de Configuração (State Machine)

```mermaid
stateDiagram-v2
    [*] --> draft: Criar config
    
    draft --> draft: Editar
    draft --> pending_approval: Submeter para aprovação
    
    pending_approval --> approved: Aprovar (mesmo usuário, 2ª confirmação)
    pending_approval --> draft: Rejeitar/Cancelar
    
    approved --> active: Deploy automático
    
    active --> draft: Nova versão (cria draft)
    
    note right of draft: Editável
    note right of pending_approval: Aguardando aprovação
    note right of approved: Pronta para deploy
    note right of active: Em uso pelos proxies
```

---

## 4. Fluxo de Configuração

### 4.1 Criação e Aprovação

```mermaid
sequenceDiagram
    autonumber
    actor Admin as 🔑 Admin
    participant UI as Frontend
    participant API as Backend
    participant DB as Database
    
    Admin->>UI: Cria/Edita configuração
    UI->>API: POST /configs
    API->>DB: Salva com status=draft
    API-->>UI: Config criada (draft)
    
    Admin->>UI: Submete para aprovação
    UI->>API: POST /configs/{id}/submit
    API->>DB: status=pending_approval
    API-->>UI: Aguardando aprovação
    
    Note over Admin,UI: Mesmo usuário, ação separada
    
    Admin->>UI: Confirma aprovação
    UI->>API: POST /configs/{id}/approve
    API->>API: Verifica: mesmo usuário que submeteu?
    API->>DB: status=approved, approved_by, approved_at
    API->>DB: Log em AUDIT_LOG
    API-->>UI: Config aprovada!
    
    Note over API,DB: Deploy automático no próximo sync
```

### 4.2 Sync com Proxies

```mermaid
sequenceDiagram
    autonumber
    participant Helper as Helper (Go)
    participant API as Backend
    participant DB as Database
    participant ATS as ATS Proxy
    
    loop A cada 30 segundos
        Helper->>API: GET /sync<br/>{hostname, config_hash}
        
        API->>DB: Busca config ativa para este proxy
        API->>API: Gera config mergeada
        API->>API: Calcula hash
        
        alt Hash diferente (config mudou)
            API-->>Helper: {new_hash, parent.config, sni.yaml, ...}
            Helper->>Helper: Escreve arquivos
            Helper->>ATS: traffic_ctl config reload
            Helper->>API: POST /sync/ack<br/>{hostname, hash, status: ok}
            API->>DB: Atualiza PROXY.current_config_hash
        else Hash igual (sem mudança)
            API-->>Helper: {unchanged: true}
        end
        
        Helper->>API: POST /stats<br/>{connections, hits, misses, ...}
        API->>DB: Insere em PROXY_STATUS
    end
```

### 4.3 Retry com Exponential Backoff

```mermaid
flowchart TD
    start(["Sync"]) --> try["Tentar conexão<br/>com Backend"]
    
    try --> success{"Sucesso?"}
    
    success -->|"Sim"| process["Processar config"]
    success -->|"Não"| retry1["Aguardar 30s"]
    
    retry1 --> try2["Retry #1"]
    try2 --> success2{"Sucesso?"}
    success2 -->|"Sim"| process
    success2 -->|"Não"| retry2["Aguardar 60s"]
    
    retry2 --> try3["Retry #2"]
    try3 --> success3{"Sucesso?"}
    success3 -->|"Sim"| process
    success3 -->|"Não"| retry3["Aguardar 120s"]
    
    retry3 --> try4["Retry #3"]
    try4 --> success4{"Sucesso?"}
    success4 -->|"Sim"| process
    success4 -->|"Não"| maxwait["Aguardar 180s (max)"]
    
    maxwait --> try5["Continua tentando<br/>a cada 180s"]
    try5 --> success4
    
    process --> done(["Config aplicada"])
    
    note1["⚠️ Proxy continua<br/>funcionando com<br/>última config válida"]
    retry1 -.-> note1
```

---

## 5. API Endpoints

### 5.1 Autenticação

| Método | Endpoint | Descrição | Auth |
|--------|----------|-----------|------|
| POST | `/auth/login` | Login, retorna JWT | - |
| POST | `/auth/refresh` | Renova token | JWT |
| POST | `/auth/beacon` | Keep-alive (30s) | JWT |
| POST | `/auth/logout` | Invalida token | JWT |

### 5.2 Usuários (Admin/Root)

| Método | Endpoint | Descrição | Role |
|--------|----------|-----------|------|
| GET | `/users` | Lista usuários | admin |
| POST | `/users` | Cria usuário | admin |
| PUT | `/users/{id}` | Edita usuário | admin |
| DELETE | `/users/{id}` | Remove usuário | root |

### 5.3 Configurações

| Método | Endpoint | Descrição | Role |
|--------|----------|-----------|------|
| GET | `/configs` | Lista configs | all |
| GET | `/configs/{id}` | Detalhe config | all |
| POST | `/configs` | Cria config | admin |
| PUT | `/configs/{id}` | Edita config (draft) | admin |
| DELETE | `/configs/{id}` | Remove config | admin |
| POST | `/configs/{id}/submit` | Submete para aprovação | admin |
| POST | `/configs/{id}/approve` | Aprova config | admin |
| POST | `/configs/{id}/reject` | Rejeita/volta para draft | admin |

### 5.4 Proxies

| Método | Endpoint | Descrição | Role |
|--------|----------|-----------|------|
| GET | `/proxies` | Lista proxies registrados | all |
| GET | `/proxies/{id}` | Detalhe + stats | all |
| GET | `/proxies/{id}/logs` | Ativa captura de logs (5min) | admin |
| DELETE | `/proxies/{id}` | Remove proxy | admin |

### 5.5 Sync (Helper - Sem Auth)

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| GET | `/sync?hostname=X&hash=Y` | Busca config |
| POST | `/sync/ack` | Confirma aplicação |
| POST | `/sync/stats` | Envia métricas |
| POST | `/sync/register` | Registra novo proxy |

### 5.6 Audit

| Método | Endpoint | Descrição | Role |
|--------|----------|-----------|------|
| GET | `/audit` | Lista audit log | admin |
| GET | `/audit?entity={id}` | Histórico de entidade | admin |

---

## 6. Frontend (Nuxt.js)

### 6.1 Páginas

```mermaid
flowchart TB
    subgraph public["🌐 Público"]
        login["/login"]
    end
    
    subgraph auth["🔒 Autenticado"]
        dashboard["/dashboard<br/>━━━━━━━━━━<br/>• Proxies online/offline<br/>• Stats agregadas<br/>• Configs pendentes"]
        
        configs["/configs<br/>━━━━━━━━━━<br/>• Lista configs<br/>• Status (draft/pending/active)<br/>• Criar/Editar"]
        
        config_detail["/configs/:id<br/>━━━━━━━━━━<br/>• Domínios<br/>• IPs<br/>• Parent Proxies<br/>• Proxies associados"]
        
        proxies["/proxies<br/>━━━━━━━━━━<br/>• Lista proxies<br/>• Status online/offline<br/>• Última sincronização"]
        
        proxy_detail["/proxies/:id<br/>━━━━━━━━━━<br/>• Stats detalhadas<br/>• Config atual<br/>• Logs (tempo real)"]
        
        users["/users<br/>━━━━━━━━━━<br/>• Lista usuários<br/>• Criar/Editar<br/>• (admin only)"]
        
        audit["/audit<br/>━━━━━━━━━━<br/>• Histórico completo<br/>• Filtros por entidade<br/>• Quem/quando/o quê"]
    end
    
    login --> dashboard
    dashboard --> configs --> config_detail
    dashboard --> proxies --> proxy_detail
    dashboard --> users
    dashboard --> audit
```

### 6.2 Componentes de Dashboard

```mermaid
flowchart LR
    subgraph cards["Cards de Status"]
        c1["🟢 Proxies Online<br/>12"]
        c2["🔴 Proxies Offline<br/>1"]
        c3["⏳ Configs Pendentes<br/>2"]
        c4["📊 Conexões/min<br/>45.2k"]
    end
    
    subgraph chart["Gráficos"]
        g1["📈 Conexões (24h)"]
        g2["📊 Hit Rate (24h)"]
    end
    
    subgraph table["Tabelas"]
        t1["Últimas Modificações"]
        t2["Proxies com Problemas"]
    end
```

---

## 7. Helper (Go)

### 7.1 Estrutura do Projeto

```
idf-proxy-helper/
├── cmd/
│   └── helper/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go        # Estrutura de configuração
│   │   └── loader.go        # Carrega flags/env
│   ├── sync/
│   │   ├── client.go        # HTTP client para backend
│   │   ├── sync.go          # Lógica de sincronização
│   │   └── backoff.go       # Exponential backoff
│   ├── ats/
│   │   ├── writer.go        # Escreve parent.config, sni.yaml
│   │   ├── reload.go        # Executa traffic_ctl reload
│   │   └── stats.go         # Coleta métricas do ATS
│   ├── logs/
│   │   └── capture.go       # Captura logs temporária
│   └── models/
│       └── types.go         # Tipos compartilhados
├── go.mod
├── go.sum
└── Dockerfile
```

### 7.2 Flags e Configuração

```bash
./helper \
  --backend-url http://backend.api:8080 \
  --config-id config-prod-01 \
  --hostname $(hostname) \
  --sync-interval 30s \
  --config-dir /opt/etc/trafficserver \
  --log-level info
```

### 7.3 Fluxo Principal

```go
// Pseudo-código do loop principal
func main() {
    cfg := config.Load()
    client := sync.NewClient(cfg.BackendURL)
    
    // Registra no backend
    client.Register(cfg.Hostname, cfg.ConfigID)
    
    ticker := time.NewTicker(cfg.SyncInterval)
    for range ticker.C {
        err := syncConfig(client, cfg)
        if err != nil {
            handleErrorWithBackoff(err)
            continue
        }
        
        // Envia stats
        stats := ats.CollectStats()
        client.SendStats(cfg.Hostname, stats)
    }
}

func syncConfig(client *sync.Client, cfg *config.Config) error {
    currentHash := getCurrentConfigHash()
    
    resp, err := client.GetConfig(cfg.Hostname, currentHash)
    if err != nil {
        return err // Será tratado com backoff
    }
    
    if resp.Unchanged {
        return nil // Nada a fazer
    }
    
    // Escreve novos arquivos
    ats.WriteParentConfig(resp.ParentConfig)
    ats.WriteSNIConfig(resp.SNIConfig)
    
    // Reload
    if err := ats.Reload(); err != nil {
        return err
    }
    
    // Confirma
    client.Ack(cfg.Hostname, resp.Hash, "ok")
    saveConfigHash(resp.Hash)
    
    return nil
}
```

### 7.4 Captura de Logs (Opcional)

```mermaid
sequenceDiagram
    participant Admin as Admin UI
    participant API as Backend
    participant Helper as Helper
    participant ATS as ATS
    
    Admin->>API: POST /proxies/{id}/logs<br/>{duration: 5min}
    API->>API: Marca proxy para captura
    API-->>Admin: OK, captura iniciada
    
    loop Durante sync
        Helper->>API: GET /sync
        API-->>Helper: {capture_logs: true, capture_until: timestamp}
        
        Helper->>ATS: Habilita debug logs
        Helper->>Helper: Captura stdout/stderr
        Helper->>API: POST /sync/logs<br/>{log_lines: [...]}
    end
    
    Note over Helper: Após 5 min ou timestamp
    Helper->>ATS: Desabilita debug logs
    
    Admin->>API: GET /proxies/{id}/logs
    API-->>Admin: Logs capturados
```

---

## 8. Segurança

### 8.1 Autenticação

```mermaid
sequenceDiagram
    participant User as Usuário
    participant UI as Frontend
    participant API as Backend
    
    User->>UI: Login (email, senha)
    UI->>API: POST /auth/login
    API->>API: Valida credenciais
    API->>API: Gera JWT (exp: 30min)
    API-->>UI: {token, refresh_token, user}
    UI->>UI: Armazena tokens
    
    loop A cada 30 segundos
        UI->>API: POST /auth/beacon
        API-->>UI: OK (ou 401 se expirado)
    end
    
    alt Token próximo de expirar
        UI->>API: POST /auth/refresh
        API-->>UI: {new_token}
    end
```

### 8.2 Autorização (RBAC)

| Role | Configs | Proxies | Users | Audit |
|------|---------|---------|-------|-------|
| **root** | CRUD + Approve | View + Logs | CRUD (incl. admins) | View |
| **admin** | CRUD + Approve | View + Logs | CRUD (exceto root/admin) | View |
| **regular** | Read | Read | - | Read |

### 8.3 Helper (Sem Auth)

O Helper não autentica porque:
- Roda dentro do container do proxy
- Backend valida por hostname registrado
- Não expõe dados sensíveis
- Config é read-only do ponto de vista do helper

---

## 9. Métricas Coletadas

### 9.1 Do ATS (via traffic_ctl)

```bash
# Conexões
proxy.process.http.current_client_connections
proxy.process.http.current_server_connections
proxy.process.http.total_client_connections
proxy.process.http.total_server_connections

# Cache
proxy.process.cache.ram_cache.hits
proxy.process.cache.ram_cache.misses

# Erros
proxy.process.http.err_client_abort_count_stat
proxy.process.http.err_connect_fail_count_stat
```

### 9.2 Agregações no Backend

| Métrica | Agregação | Retenção |
|---------|-----------|----------|
| Conexões por proxy | Por minuto | 7 dias |
| Hit rate por proxy | Por minuto | 7 dias |
| Total conexões fleet | Por minuto | 30 dias |
| Erros por proxy | Por minuto | 7 dias |

---

## 10. Deploy

### 10.1 Docker Compose (Dev/Homolog)

```yaml
version: '3.8'

services:
  # PostgreSQL
  db:
    image: postgres:15
    environment:
      POSTGRES_DB: idf_proxy_manager
      POSTGRES_USER: idf
      POSTGRES_PASSWORD: secret
    volumes:
      - pgdata:/var/lib/postgresql/data

  # Backend (Go)
  backend:
    build: ./backend
    environment:
      DATABASE_URL: postgres://idf:secret@db:5432/idf_proxy_manager
      JWT_SECRET: super-secret-key
    ports:
      - "8080:8080"
    depends_on:
      - db

  # Frontend (Nuxt.js)
  frontend:
    build: ./frontend
    environment:
      API_URL: http://backend:8080
    ports:
      - "3000:3000"
    depends_on:
      - backend

  # Proxy com Helper (exemplo)
  proxy-01:
    build: ./proxy
    environment:
      HELPER_BACKEND_URL: http://backend:8080
      HELPER_CONFIG_ID: config-prod-01
      HELPER_HOSTNAME: proxy-01
    ports:
      - "8153:8153"
    depends_on:
      - backend

volumes:
  pgdata:
```

---

## 11. Próximos Passos

### Fase 1: MVP
- [ ] Backend: Auth + CRUD Configs + Sync endpoint
- [ ] Helper: Sync básico + reload
- [ ] Frontend: Login + Dashboard + Configs

### Fase 2: Aprovação
- [ ] Backend: Workflow de aprovação
- [ ] Frontend: UI de aprovação
- [ ] Audit log

### Fase 3: Métricas
- [ ] Helper: Coleta de stats
- [ ] Backend: Agregação
- [ ] Frontend: Gráficos

### Fase 4: Logs
- [ ] Helper: Captura de logs
- [ ] Frontend: Visualização real-time

---

## 12. Decisões Técnicas

| Decisão | Escolha | Motivo |
|---------|---------|--------|
| Comunicação Helper↔Backend | Polling 30s | Resiliente a falhas de rede |
| Auth Helper | Nenhuma | Simplicidade, roda em ambiente controlado |
| Auth Frontend/Backend | JWT + Beacon 30s | Stateless, detecta sessão inativa |
| Retry | Exponential backoff até 3min | Evita sobrecarga em falhas |
| Aprovação | 2 passos, mesmo usuário | Confirmação consciente |
| Logs temporários | Max 5 min | Evita excesso de dados |
