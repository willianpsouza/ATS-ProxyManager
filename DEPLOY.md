# ATS Proxy Manager - Deploy from Scratch

Guia para deploy em uma VM Ubuntu com Docker e Docker Compose instalados.

## Pre-requisitos

- Ubuntu 20.04+ com acesso SSH
- Docker Engine 24+
- Docker Compose v2+
- Git

```bash
# Verificar
docker --version
docker compose version
git --version
```

## 1. Clonar o repositorio

```bash
cd /opt
git clone https://github.com/willianpsouza/ATS-ProxyManager.git
cd ATS-ProxyManager
```

## 2. Gerar o arquivo .env

O script abaixo gera senhas aleatorias automaticamente:

```bash
# Gerar senhas
DB_PASS=$(openssl rand -base64 24 | tr -dc 'a-zA-Z0-9' | head -c 32)
JWT_SECRET=$(openssl rand -base64 48 | tr -dc 'a-zA-Z0-9' | head -c 64)
ROOT_PASS=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)

cat > .env << EOF
# Database
POSTGRES_USER=proxymanager
POSTGRES_PASSWORD=${DB_PASS}
POSTGRES_DB=proxymanager
DATABASE_URL=postgres://proxymanager:${DB_PASS}@postgres:5432/proxymanager?sslmode=disable

# Redis
REDIS_URL=redis://redis:6379/0

# JWT
JWT_SECRET=${JWT_SECRET}

# Backend
PORT=8080
EOF

echo ""
echo "========================================="
echo "  Credenciais geradas (ANOTE-AS!)"
echo "========================================="
echo "PostgreSQL:  proxymanager / ${DB_PASS}"
echo "App ROOT:    root@proxy-manager.local / changeme"
echo "========================================="
echo ""
echo "IMPORTANTE: Troque a senha do usuario root"
echo "apos o primeiro login!"
```

## 3. Configurar docker-compose para producao

Crie um arquivo `docker-compose.prod.yml` para sobrescrever configuracoes de dev:

```bash
cat > docker-compose.prod.yml << 'EOF'
services:
  postgres:
    environment:
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    ports: []

  redis:
    ports: []

  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    environment:
      DATABASE_URL: ${DATABASE_URL}
      REDIS_URL: ${REDIS_URL}
      JWT_SECRET: ${JWT_SECRET}
      PORT: ${PORT}
    volumes: []
    restart: unless-stopped

  proxy-01:
    restart: unless-stopped
EOF
```

## 4. Subir a infraestrutura (Postgres + Redis)

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d postgres redis
```

Aguardar os healthchecks ficarem healthy:

```bash
docker compose ps
# Espere ate STATUS mostrar (healthy) para postgres e redis
```

O schema do banco eh aplicado automaticamente na primeira subida via `docker-entrypoint-initdb.d`.

## 5. Build de todos os servicos

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml build
```

## 6. Subir todos os servicos

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## 7. Verificar

```bash
# Status de todos os containers
docker compose ps

# Logs do backend
docker compose logs backend --tail 20

# Testar health check
curl -s http://localhost:8080/api/v1/health | python3 -m json.tool
```

Todos os 4 servicos devem estar `Up`:

| Servico | Container | Porta |
|---------|-----------|-------|
| PostgreSQL 16 | ats-proxymanager-postgres-1 | 5432 (interna) |
| Redis 7 | ats-proxymanager-redis-1 | 6379 (interna) |
| Backend (Go) | ats-proxymanager-backend-1 | **8080** |
| Helper (proxy-01) | ats-proxymanager-proxy-01-1 | - |

## 8. Testar login

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"root@proxy-manager.local","password":"changeme"}' \
  | python3 -m json.tool
```

Resposta esperada:
```json
{
    "token": "eyJhbGciOi...",
    "refresh_token": "eyJhbGciOi...",
    "expires_in": 1800,
    "user": {
        "id": "uuid",
        "email": "root@proxy-manager.local",
        "username": "root",
        "role": "root"
    }
}
```

## 9. Fluxo basico pos-deploy

```bash
# 1. Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"root@proxy-manager.local","password":"changeme"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# 2. Criar config
CONFIG_ID=$(curl -s -X POST http://localhost:8080/api/v1/configs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production",
    "description": "Config de producao",
    "domains": [
      {"domain": ".internal.local", "action": "direct", "priority": 10},
      {"domain": ".svc.cluster.local", "action": "direct", "priority": 20}
    ],
    "ip_ranges": [
      {"cidr": "10.0.0.0/8", "action": "direct", "priority": 10},
      {"cidr": "172.16.0.0/12", "action": "direct", "priority": 20},
      {"cidr": "192.168.0.0/16", "action": "direct", "priority": 30}
    ],
    "parent_proxies": [
      {"address": "proxy-corp-01.example.com", "port": 3128, "priority": 1, "enabled": true},
      {"address": "proxy-corp-02.example.com", "port": 3128, "priority": 2, "enabled": true}
    ],
    "proxy_ids": []
  }' | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Config criada: $CONFIG_ID"

# 3. Submit para aprovacao
curl -s -X POST "http://localhost:8080/api/v1/configs/$CONFIG_ID/submit" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])"

# 4. Aprovar (ativa a config)
curl -s -X POST "http://localhost:8080/api/v1/configs/$CONFIG_ID/approve" \
  -H "Authorization: Bearer $TOKEN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(f'Status: {d[\"status\"]}, Hash: {d[\"config_hash\"]}')"

# 5. Verificar que o helper recebeu
docker compose logs proxy-01 --tail 10
```

## Deploy em desenvolvimento

Para desenvolvimento local com hot-reload:

```bash
# Usa Dockerfile.dev (air hot-reload) e expoe portas
docker compose up --build
```

## Adicionando mais proxies (Helpers)

Para cada instancia de ATS proxy, adicione um servico no `docker-compose.yml`:

```yaml
  proxy-02:
    build:
      context: ./helper
      dockerfile: Dockerfile
    command:
      - "--backend-url=http://backend:8080"
      - "--config-id=<UUID_DA_CONFIG>"
      - "--hostname=proxy-02"
      - "--sync-interval=30s"
      - "--config-dir=/opt/etc/trafficserver"
      - "--log-level=info"
    depends_on:
      - backend
    restart: unless-stopped
```

Em producao, o Helper roda como sidecar ou DaemonSet junto ao ATS real.

## Deploy de proxy ATS standalone (sem backend)

Para ambientes com dezenas de proxies, o backend/frontend/postgres/redis ficam em uma unica VM e cada proxy ATS roda em sua propria VM (ou container separado) apontando para o backend remoto.

### Pre-requisitos no host do proxy

- Docker Engine 24+
- Conectividade de rede com o backend (porta 8080)

### 1. Criar diretorio e Dockerfile

```bash
mkdir -p /opt/ats-proxy && cd /opt/ats-proxy
```

Crie o arquivo `Dockerfile`:

```bash
cat > Dockerfile << 'DOCKERFILE'
# Build helper
FROM golang:1.26-alpine AS helper-builder
WORKDIR /build
COPY helper/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /helper ./cmd/helper

# ATS + helper
FROM trafficserver/trafficserver:latest
COPY --from=helper-builder /helper /usr/local/bin/helper
COPY ats-config/records.yaml /opt/etc/trafficserver/records.yaml
COPY ats-config/remap.config /opt/etc/trafficserver/remap.config
COPY ats-config/storage.config /opt/etc/trafficserver/storage.config
COPY ats-config/logging.yaml /opt/etc/trafficserver/logging.yaml
RUN mkdir -p /opt/var/log/trafficserver /opt/var/run/trafficserver
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080 8443
ENTRYPOINT ["/entrypoint.sh"]
DOCKERFILE
```

### 2. Copiar arquivos do repositorio

No servidor do backend, copie para o host do proxy:

```bash
# No servidor do backend
cd /opt/ATS-ProxyManager
scp -r helper/ usuario@PROXY_HOST:/opt/ats-proxy/helper/
scp -r proxy-full/ats-config/ usuario@PROXY_HOST:/opt/ats-proxy/ats-config/
scp proxy-full/entrypoint.sh usuario@PROXY_HOST:/opt/ats-proxy/entrypoint.sh
```

Ou clone o repositorio inteiro e use os arquivos:

```bash
cd /opt
git clone https://github.com/willianpsouza/ATS-ProxyManager.git
cd ats-proxy
cp -r /opt/ATS-ProxyManager/helper/ .
cp -r /opt/ATS-ProxyManager/proxy-full/ats-config/ .
cp /opt/ATS-ProxyManager/proxy-full/entrypoint.sh .
cp /opt/ATS-ProxyManager/proxy-full/Dockerfile .
```

### 3. Criar docker-compose.yml do proxy

```bash
cat > docker-compose.yml << 'EOF'
services:
  ats-proxy:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
      - "8443:8443"
    environment:
      BACKEND_URL: http://BACKEND_IP:8080
      PROXY_HOSTNAME: proxy-DATACENTER-01.empresa.local
      SYNC_INTERVAL: "30s"
    volumes:
      - ats_logs:/opt/var/log/trafficserver
    healthcheck:
      test: ["CMD", "traffic_ctl", "server", "status"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 15s
    restart: unless-stopped

volumes:
  ats_logs:
EOF
```

Substitua:
- `BACKEND_IP` pelo IP/hostname do servidor do backend
- `proxy-DATACENTER-01.empresa.local` pelo hostname unico deste proxy

### 4. Build e start

```bash
docker compose up -d --build
```

### 5. Verificar

```bash
# Container rodando
docker compose ps

# Logs do helper + ATS
docker compose logs -f

# Deve mostrar:
#   [ATS] ATS está rodando!
#   Iniciando proxy-helper v1.0.0
#   Backend: http://BACKEND_IP:8080
#   Registro OK — proxy_id: <uuid>
```

### 6. Associar config no backend

O proxy aparece automaticamente no painel ao se registrar. Para associar uma config:

**Via frontend:** Proxies > selecionar proxy > Atribuir Config

**Via API:**

```bash
# No servidor do backend
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"root@proxy-manager.local","password":"changeme"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Listar proxies registrados
curl -s http://localhost:8080/api/v1/proxies \
  -H "Authorization: Bearer $TOKEN" \
  | python3 -c "import sys,json; [print(f\"  {p['id']} - {p['hostname']} ({'online' if p['is_online'] else 'offline'})\") for p in json.load(sys.stdin)['data']]"

# Atribuir config ao proxy
curl -s -X PUT http://localhost:8080/api/v1/proxies/PROXY_UUID/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"config_id": "CONFIG_UUID"}'
```

### 7. Deploy em escala (multiplos proxies)

Para dezenas de proxies, o padrao recomendado:

```
                    ┌──────────────┐
                    │   Backend    │
                    │  + Frontend  │
                    │  + Postgres  │
                    │  + Redis     │
                    │  + Nginx     │
                    └──────┬───────┘
                           │ :8080
          ┌────────────────┼────────────────┐
          │                │                │
   ┌──────┴───────┐ ┌─────┴────────┐ ┌─────┴────────┐
   │  ATS+Helper  │ │  ATS+Helper  │ │  ATS+Helper  │
   │  proxy-sp-01 │ │  proxy-rj-01 │ │  proxy-us-01 │
   │  :8080/:8443 │ │  :8080/:8443 │ │  :8080/:8443 │
   └──────────────┘ └──────────────┘ └──────────────┘
```

Cada proxy:
- Roda em sua propria VM/container
- Se registra automaticamente no backend via hostname unico
- Recebe a config (parent.config, sni.yaml, ip_allow.yaml) do backend
- Reporta metricas e status a cada 30s
- Reconecta automaticamente se o backend ficar indisponivel

**Convencao de hostnames:**

```
proxy-{datacenter}-{numero}.{dominio}
```

Exemplos: `proxy-sp-01.empresa.local`, `proxy-rj-01.empresa.local`, `proxy-aws-us-01.empresa.local`

### 8. Atualizando os proxies

```bash
# Em cada host de proxy
cd /opt/ats-proxy
# Atualizar helper (se houve mudanca no codigo)
cp -r /caminho/novo/helper/ .
docker compose up -d --build
```

O proxy re-registra automaticamente e recebe a config vigente.

## Comandos uteis

```bash
# Ver logs em tempo real
docker compose logs -f

# Logs de um servico especifico
docker compose logs -f backend

# Reiniciar um servico
docker compose restart backend

# Rebuild e redeploy de um servico
docker compose up -d --build backend

# Parar tudo
docker compose down

# Parar e remover volumes (APAGA DADOS!)
docker compose down -v

# Ver metricas dos proxies
curl -s http://localhost:8080/api/v1/proxies \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

# Ver audit trail
curl -s http://localhost:8080/api/v1/audit \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

## Atualizando para nova versao

```bash
cd /opt/ATS-ProxyManager
git pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

## Troubleshooting

| Problema | Solucao |
|----------|---------|
| Backend nao conecta no Postgres | Verificar se postgres esta `healthy`: `docker compose ps` |
| Helper nao recebe config | Verificar se existe config `active` com proxy associado via `config_proxies` |
| Login retorna 401 | Senha padrao eh `changeme`, email eh `root@proxy-manager.local` |
| Schema nao foi aplicado | O init script so roda na primeira subida. Recrie o volume: `docker compose down -v && docker compose up -d` |
| Proxy aparece offline | Helper precisa fazer polling a cada 30s. Proxies sem contato em 2 min sao marcados offline |
