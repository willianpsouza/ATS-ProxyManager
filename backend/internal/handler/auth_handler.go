package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ats-proxy/proxy-manager/backend/internal/auth"
	"github.com/ats-proxy/proxy-manager/backend/internal/service"
)

type AuthHandler struct {
	authSvc *service.AuthService
}

func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	ip := clientIP(r)
	ua := r.UserAgent()

	resp, err := h.authSvc.Login(r.Context(), req.Email, req.Password, ip, ua)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid_credentials", "Email ou senha inválidos")
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	resp, err := h.authSvc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid_refresh_token", "Token de refresh inválido")
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) Beacon(w http.ResponseWriter, r *http.Request) {
	tokenRaw := getTokenRaw(r.Context())
	tokenHash := auth.HashToken(tokenRaw)

	if err := h.authSvc.Beacon(r.Context(), tokenHash); err != nil {
		respondError(w, http.StatusUnauthorized, "token_expired", "Token expirado, faça login novamente")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"server_time": time.Now().UTC(),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	tokenRaw := getTokenRaw(r.Context())
	tokenHash := auth.HashToken(tokenRaw)

	_ = h.authSvc.Logout(r.Context(), tokenHash)

	respondJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}
