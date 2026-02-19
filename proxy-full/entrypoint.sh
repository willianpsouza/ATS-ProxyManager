#!/bin/bash
set -e

export PATH="/opt/bin:${PATH}"

BACKEND_URL="${BACKEND_URL:-http://backend:8080}"
CONFIG_ID="${CONFIG_ID:-default}"
HOSTNAME="${PROXY_HOSTNAME:-$(hostname)}"
SYNC_INTERVAL="${SYNC_INTERVAL:-30s}"

echo "=== ATS Proxy Full Container ==="
echo "Hostname: $HOSTNAME"
echo "Backend:  $BACKEND_URL"
echo "Config:   $CONFIG_ID"
echo ""

# Inicia ATS em background (imagem oficial usa traffic_server como CMD)
echo "[ATS] Iniciando Apache Traffic Server..."
traffic_server &
ATS_PID=$!

# Aguarda ATS ficar pronto
echo "[ATS] Aguardando ATS ficar pronto..."
for i in $(seq 1 30); do
    if traffic_ctl server status 2>/dev/null | grep -q "initialized_done"; then
        echo "[ATS] ATS está rodando!"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "[ATS] WARN: Timeout aguardando ATS, continuando mesmo assim..."
    fi
    sleep 1
done

# Mostra versão
echo "[ATS] $(traffic_server --version 2>/dev/null | head -1 || echo 'versão desconhecida')"
echo ""

# Inicia helper apontando para config dir do ATS (/opt/etc/trafficserver)
echo "[Helper] Iniciando helper..."
exec /usr/local/bin/helper \
    --backend-url="$BACKEND_URL" \
    --config-id="$CONFIG_ID" \
    --hostname="$HOSTNAME" \
    --sync-interval="$SYNC_INTERVAL" \
    --config-dir="/opt/etc/trafficserver" \
    --log-level="info"
