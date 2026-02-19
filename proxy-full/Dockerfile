# ============================
# Stage 1: Build helper binary
# ============================
FROM golang:1.22-alpine AS helper-builder

WORKDIR /build
COPY helper/go.mod ./
COPY helper/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /helper ./cmd/helper

# ============================
# Stage 2: ATS oficial + helper
# ============================
FROM trafficserver/trafficserver:latest

# Copia helper
COPY --from=helper-builder /helper /usr/local/bin/helper

# Copia configs ATS customizados (sobrescreve os defaults)
COPY proxy-full/ats-config/records.yaml /opt/etc/trafficserver/records.yaml
COPY proxy-full/ats-config/remap.config /opt/etc/trafficserver/remap.config
COPY proxy-full/ats-config/storage.config /opt/etc/trafficserver/storage.config
COPY proxy-full/ats-config/logging.yaml /opt/etc/trafficserver/logging.yaml

# Garante que diretórios de runtime existem
RUN mkdir -p \
    /opt/var/log/trafficserver \
    /opt/var/cache/trafficserver \
    /opt/var/run/trafficserver

# Copia entrypoint
COPY proxy-full/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8080 8443

ENTRYPOINT ["/entrypoint.sh"]
