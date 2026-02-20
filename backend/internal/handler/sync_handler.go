package handler

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/ats-proxy/proxy-manager/backend/internal/service"
)

type SyncHandler struct {
	syncSvc *service.SyncService
}

func NewSyncHandler(syncSvc *service.SyncService) *SyncHandler {
	return &SyncHandler{syncSvc: syncSvc}
}

func (h *SyncHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req service.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	// Extract client IP
	req.RemoteIP = extractIP(r)

	resp, err := h.syncSvc.Register(r.Context(), req)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func extractIP(r *http.Request) string {
	// Check X-Real-IP first (set by nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Check X-Forwarded-For (first IP is the client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Fallback to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *SyncHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	hash := r.URL.Query().Get("hash")

	if hostname == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "hostname query param required")
		return
	}

	resp, err := h.syncSvc.GetConfig(r.Context(), hostname, hash)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (h *SyncHandler) Ack(w http.ResponseWriter, r *http.Request) {
	var req service.AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	if err := h.syncSvc.Ack(r.Context(), req); err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"acknowledged": true})
}

func (h *SyncHandler) Stats(w http.ResponseWriter, r *http.Request) {
	var req service.SyncStatsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	if err := h.syncSvc.Stats(r.Context(), req); err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"received": true})
}

func (h *SyncHandler) Logs(w http.ResponseWriter, r *http.Request) {
	var req service.SyncLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	resp, err := h.syncSvc.Logs(r.Context(), req)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
