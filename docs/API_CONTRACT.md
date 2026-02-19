# IDF Proxy Manager - API Contract

**Versão**: 1.0  
**Base URL**: `/api/v1`

---

## Autenticação

Todas as rotas (exceto `/auth/*` e `/sync/*`) requerem header:
```
Authorization: Bearer <jwt_token>
```

---

## 1. Auth

### POST /auth/login

**Request:**
```json
{
  "email": "admin@idf-experian.com",
  "password": "secret123"
}
```

**Response 200:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 1800,
  "user": {
    "id": "uuid",
    "email": "admin@idf-experian.com",
    "username": "admin",
    "role": "admin"
  }
}
```

**Response 401:**
```json
{
  "error": "invalid_credentials",
  "message": "Email ou senha inválidos"
}
```

---

### POST /auth/refresh

**Request:**
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Response 200:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 1800
}
```

---

### POST /auth/beacon

Keep-alive para manter sessão ativa. Chamar a cada 30 segundos.

**Response 200:**
```json
{
  "status": "ok",
  "server_time": "2025-02-03T22:00:00Z"
}
```

**Response 401:**
```json
{
  "error": "token_expired",
  "message": "Token expirado, faça login novamente"
}
```

---

## 2. Users

### GET /users

**Query params:**
- `role` (optional): filtrar por role (root|admin|regular)
- `page` (optional): página (default: 1)
- `limit` (optional): itens por página (default: 20)

**Response 200:**
```json
{
  "data": [
    {
      "id": "uuid",
      "username": "admin",
      "email": "admin@idf-experian.com",
      "role": "admin",
      "created_at": "2025-02-01T10:00:00Z",
      "last_login": "2025-02-03T21:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 5,
    "total_pages": 1
  }
}
```

---

### POST /users

**Roles permitidos:** admin, root

**Request:**
```json
{
  "username": "newuser",
  "email": "newuser@idf-experian.com",
  "password": "secret123",
  "role": "regular"
}
```

**Regras:**
- `admin` pode criar `regular` 
- `root` pode criar `regular` e `admin`
- Ninguém pode criar `root`

**Response 201:**
```json
{
  "id": "uuid",
  "username": "newuser",
  "email": "newuser@idf-experian.com",
  "role": "regular",
  "created_at": "2025-02-03T22:00:00Z"
}
```

---

### PUT /users/{id}

**Request:**
```json
{
  "username": "updateduser",
  "email": "updated@idf-experian.com",
  "role": "admin"
}
```

**Response 200:**
```json
{
  "id": "uuid",
  "username": "updateduser",
  "email": "updated@idf-experian.com",
  "role": "admin",
  "updated_at": "2025-02-03T22:00:00Z"
}
```

---

### DELETE /users/{id}

**Roles permitidos:** root

**Response 204:** No content

---

## 3. Configs

### GET /configs

**Query params:**
- `status` (optional): draft|pending_approval|approved|active
- `page`, `limit`

**Response 200:**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "Production Config",
      "description": "Configuração principal de produção",
      "status": "active",
      "version": 3,
      "proxy_count": 5,
      "modified_by": {
        "id": "uuid",
        "username": "admin"
      },
      "modified_at": "2025-02-03T20:00:00Z",
      "approved_by": {
        "id": "uuid",
        "username": "admin"
      },
      "approved_at": "2025-02-03T20:05:00Z"
    }
  ],
  "pagination": {...}
}
```

---

### GET /configs/{id}

**Response 200:**
```json
{
  "id": "uuid",
  "name": "Production Config",
  "description": "Configuração principal de produção",
  "status": "active",
  "version": 3,
  
  "domains": [
    {
      "id": "uuid",
      "domain": ".eec",
      "action": "direct",
      "priority": 10
    },
    {
      "id": "uuid",
      "domain": ".eeca",
      "action": "direct",
      "priority": 20
    }
  ],
  
  "ip_ranges": [
    {
      "id": "uuid",
      "cidr": "10.0.0.0/8",
      "action": "direct",
      "priority": 10
    }
  ],
  
  "parent_proxies": [
    {
      "id": "uuid",
      "address": "10.96.215.26",
      "port": 3128,
      "priority": 1,
      "enabled": true
    },
    {
      "id": "uuid",
      "address": "10.253.16.93",
      "port": 3128,
      "priority": 2,
      "enabled": true
    }
  ],
  
  "proxies": [
    {
      "id": "uuid",
      "hostname": "proxy-01",
      "is_online": true,
      "last_seen": "2025-02-03T21:59:30Z"
    }
  ],
  
  "modified_by": {...},
  "modified_at": "2025-02-03T20:00:00Z",
  "approved_by": {...},
  "approved_at": "2025-02-03T20:05:00Z"
}
```

---

### POST /configs

**Request:**
```json
{
  "name": "New Config",
  "description": "Descrição da config",
  
  "domains": [
    {"domain": ".eec", "action": "direct", "priority": 10},
    {"domain": ".eeca", "action": "direct", "priority": 20}
  ],
  
  "ip_ranges": [
    {"cidr": "10.0.0.0/8", "action": "direct", "priority": 10}
  ],
  
  "parent_proxies": [
    {"address": "10.96.215.26", "port": 3128, "priority": 1, "enabled": true}
  ],
  
  "proxy_ids": ["uuid-proxy-1", "uuid-proxy-2"]
}
```

**Response 201:**
```json
{
  "id": "uuid",
  "name": "New Config",
  "status": "draft",
  "version": 1,
  ...
}
```

---

### PUT /configs/{id}

Só permite edição se `status == draft`

**Request:** (mesmo formato do POST)

**Response 200:** Config atualizada

**Response 400:**
```json
{
  "error": "invalid_status",
  "message": "Só é possível editar configs em status draft"
}
```

---

### POST /configs/{id}/submit

Submete config para aprovação.

**Response 200:**
```json
{
  "id": "uuid",
  "status": "pending_approval",
  "submitted_at": "2025-02-03T22:00:00Z",
  "submitted_by": {...}
}
```

---

### POST /configs/{id}/approve

Aprova config. Deve ser o mesmo usuário que submeteu.

**Response 200:**
```json
{
  "id": "uuid",
  "status": "active",
  "approved_at": "2025-02-03T22:05:00Z",
  "approved_by": {...}
}
```

**Response 400:**
```json
{
  "error": "same_user_required",
  "message": "A aprovação deve ser feita pelo mesmo usuário que submeteu"
}
```

---

### POST /configs/{id}/reject

Rejeita e volta para draft.

**Request:**
```json
{
  "reason": "Motivo da rejeição"
}
```

**Response 200:**
```json
{
  "id": "uuid",
  "status": "draft",
  "rejected_at": "2025-02-03T22:05:00Z"
}
```

---

## 4. Proxies

### GET /proxies

**Response 200:**
```json
{
  "data": [
    {
      "id": "uuid",
      "hostname": "proxy-01",
      "config": {
        "id": "uuid",
        "name": "Production Config"
      },
      "is_online": true,
      "last_seen": "2025-02-03T21:59:30Z",
      "registered_at": "2025-02-01T10:00:00Z",
      "current_config_hash": "abc123",
      "stats": {
        "active_connections": 150,
        "total_connections_1h": 45000,
        "cache_hit_rate": 0.85
      }
    }
  ],
  "summary": {
    "total": 10,
    "online": 9,
    "offline": 1
  }
}
```

---

### GET /proxies/{id}

**Response 200:**
```json
{
  "id": "uuid",
  "hostname": "proxy-01",
  "config": {...},
  "is_online": true,
  "last_seen": "2025-02-03T21:59:30Z",
  "registered_at": "2025-02-01T10:00:00Z",
  "current_config_hash": "abc123",
  
  "stats_history": [
    {
      "timestamp": "2025-02-03T21:00:00Z",
      "active_connections": 120,
      "total_connections": 3500,
      "cache_hits": 2800,
      "cache_misses": 700
    }
  ]
}
```

---

### POST /proxies/{id}/logs

Inicia captura de logs por até 5 minutos.

**Request:**
```json
{
  "duration_minutes": 5
}
```

**Response 200:**
```json
{
  "status": "capturing",
  "capture_until": "2025-02-03T22:05:00Z"
}
```

---

### GET /proxies/{id}/logs

Retorna logs capturados.

**Response 200:**
```json
{
  "proxy_id": "uuid",
  "capture_started": "2025-02-03T22:00:00Z",
  "capture_ended": "2025-02-03T22:05:00Z",
  "lines": [
    {
      "timestamp": "2025-02-03T22:00:01Z",
      "level": "INFO",
      "message": "Result for api.eeca was PARENT_DIRECT"
    }
  ]
}
```

---

## 5. Sync (Helper - Sem Auth)

### POST /sync/register

Registra novo proxy no backend.

**Request:**
```json
{
  "hostname": "proxy-01",
  "config_id": "config-prod-01"
}
```

**Response 200:**
```json
{
  "proxy_id": "uuid",
  "config_id": "uuid",
  "status": "registered"
}
```

---

### GET /sync

**Query params:**
- `hostname`: hostname do proxy
- `hash`: hash da config atual

**Response 200 (config mudou):**
```json
{
  "unchanged": false,
  "hash": "newhash123",
  "config": {
    "parent_config": "dest_ip=10.0.0.0-10.255.255.255 go_direct=true\n...",
    "sni_yaml": "sni:\n  - fqdn: '*.eec'\n    tunnel_route: direct\n...",
    "ip_allow_yaml": "..."
  },
  "capture_logs": false
}
```

**Response 200 (sem mudança):**
```json
{
  "unchanged": true
}
```

**Response 200 (com captura de logs):**
```json
{
  "unchanged": true,
  "capture_logs": true,
  "capture_until": "2025-02-03T22:05:00Z"
}
```

---

### POST /sync/ack

Confirma aplicação da config.

**Request:**
```json
{
  "hostname": "proxy-01",
  "hash": "newhash123",
  "status": "ok"
}
```

**Response 200:**
```json
{
  "acknowledged": true
}
```

---

### POST /sync/stats

Envia métricas do proxy.

**Request:**
```json
{
  "hostname": "proxy-01",
  "timestamp": "2025-02-03T22:00:00Z",
  "metrics": {
    "active_connections": 150,
    "total_connections": 3500,
    "cache_hits": 2800,
    "cache_misses": 700,
    "errors": 5
  }
}
```

**Response 200:**
```json
{
  "received": true
}
```

---

### POST /sync/logs

Envia logs capturados.

**Request:**
```json
{
  "hostname": "proxy-01",
  "lines": [
    {
      "timestamp": "2025-02-03T22:00:01Z",
      "level": "INFO",
      "message": "Result for api.eeca was PARENT_DIRECT"
    }
  ]
}
```

**Response 200:**
```json
{
  "received": true,
  "continue_capture": true
}
```

---

## 6. Audit

### GET /audit

**Query params:**
- `entity_type`: user|config|proxy
- `entity_id`: UUID específico
- `user_id`: filtrar por usuário
- `from`, `to`: range de datas
- `page`, `limit`

**Response 200:**
```json
{
  "data": [
    {
      "id": "uuid",
      "user": {
        "id": "uuid",
        "username": "admin"
      },
      "action": "config.approve",
      "entity_type": "config",
      "entity_id": "uuid",
      "changes": {
        "status": {
          "old": "pending_approval",
          "new": "active"
        }
      },
      "ip_address": "10.16.0.100",
      "created_at": "2025-02-03T22:05:00Z"
    }
  ],
  "pagination": {...}
}
```

---

## Códigos de Erro

| Código | Descrição |
|--------|-----------|
| 400 | Bad Request - Dados inválidos |
| 401 | Unauthorized - Token inválido/expirado |
| 403 | Forbidden - Sem permissão |
| 404 | Not Found - Recurso não encontrado |
| 409 | Conflict - Conflito de estado |
| 500 | Internal Server Error |

**Formato de erro:**
```json
{
  "error": "error_code",
  "message": "Mensagem legível",
  "details": {} // opcional
}
```
