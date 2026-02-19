package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/service"
)

type ProxyHandler struct {
	proxySvc *service.ProxyService
}

func NewProxyHandler(proxySvc *service.ProxyService) *ProxyHandler {
	return &ProxyHandler{proxySvc: proxySvc}
}

func (h *ProxyHandler) List(w http.ResponseWriter, r *http.Request) {
	resp, err := h.proxySvc.List(r.Context())
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (h *ProxyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid proxy ID")
		return
	}

	detail, err := h.proxySvc.GetByID(r.Context(), id)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, detail)
}

func (h *ProxyHandler) StartLogCapture(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid proxy ID")
		return
	}

	var req struct {
		DurationMinutes int `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}
	if req.DurationMinutes == 0 {
		req.DurationMinutes = 5
	}

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	until, err := h.proxySvc.StartLogCapture(r.Context(), id, req.DurationMinutes, userID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "capturing",
		"capture_until": until,
	})
}

func (h *ProxyHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid proxy ID")
		return
	}

	logs, err := h.proxySvc.GetLogs(r.Context(), id)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	type logLine struct {
		Timestamp string  `json:"timestamp"`
		Level     *string `json:"level,omitempty"`
		Message   *string `json:"message,omitempty"`
	}

	lines := make([]logLine, 0, len(logs))
	for _, l := range logs {
		lines = append(lines, logLine{
			Timestamp: l.CapturedAt.Format("2006-01-02T15:04:05Z"),
			Level:     l.LogLevel,
			Message:   l.Message,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"proxy_id": id.String(),
		"lines":    lines,
	})
}
