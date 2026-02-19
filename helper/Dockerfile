# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copia go.mod
COPY go.mod ./
# Se tiver go.sum, descomentar:
# COPY go.sum ./
# RUN go mod download

# Copia código fonte
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /helper ./cmd/helper

# Final stage
FROM alpine:3.19

# Instala ca-certificates para HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copia binário
COPY --from=builder /helper /app/helper

# Usuário não-root
RUN adduser -D -u 1000 helper
USER helper

ENTRYPOINT ["/app/helper"]
