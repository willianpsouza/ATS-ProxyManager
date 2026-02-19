package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/auth"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type contextKey string

const (
	ctxUserID   contextKey = "user_id"
	ctxUsername  contextKey = "username"
	ctxEmail    contextKey = "email"
	ctxRole     contextKey = "role"
	ctxTokenRaw contextKey = "token_raw"
)

func getUserID(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(ctxUserID).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func getUserRole(ctx context.Context) domain.UserRole {
	if v, ok := ctx.Value(ctxRole).(domain.UserRole); ok {
		return v
	}
	return ""
}

func getTokenRaw(ctx context.Context) string {
	if v, ok := ctx.Value(ctxTokenRaw).(string); ok {
		return v
	}
	return ""
}

// AuthMiddleware validates JWT tokens and populates context.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				respondError(w, http.StatusUnauthorized, "unauthorized", "Missing authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			if tokenStr == header {
				respondError(w, http.StatusUnauthorized, "unauthorized", "Invalid authorization format")
				return
			}

			claims, err := auth.ParseToken(secret, tokenStr)
			if err != nil {
				respondError(w, http.StatusUnauthorized, "token_expired", "Token expirado, faça login novamente")
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
			ctx = context.WithValue(ctx, ctxUsername, claims.Username)
			ctx = context.WithValue(ctx, ctxEmail, claims.Email)
			ctx = context.WithValue(ctx, ctxRole, claims.Role)
			ctx = context.WithValue(ctx, ctxTokenRaw, tokenStr)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole checks that the caller has at least one of the given roles.
func RequireRole(roles ...domain.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := getUserRole(r.Context())
			for _, allowed := range roles {
				if role == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}
			respondError(w, http.StatusForbidden, "forbidden", "Sem permissão")
		})
	}
}

// CORS middleware
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequestID middleware
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts client IP from request.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.SplitN(fwd, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return strings.SplitN(r.RemoteAddr, ":", 2)[0]
}
