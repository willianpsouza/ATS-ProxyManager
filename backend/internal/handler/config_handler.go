package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/service"
)

type ConfigHandler struct {
	configSvc *service.ConfigService
}

func NewConfigHandler(configSvc *service.ConfigService) *ConfigHandler {
	return &ConfigHandler{configSvc: configSvc}
}

func (h *ConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	var statusFilter *domain.ConfigStatus
	if s := r.URL.Query().Get("status"); s != "" {
		status := domain.ConfigStatus(s)
		if status.IsValid() {
			statusFilter = &status
		}
	}

	configs, total, err := h.configSvc.List(r.Context(), statusFilter, page, limit)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	if configs == nil {
		configs = []domain.Config{}
	}

	respondJSON(w, http.StatusOK, paginatedResponse{
		Data: configs,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages(total, limit),
		},
	})
}

func (h *ConfigHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	detail, err := h.configSvc.GetByID(r.Context(), id)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, detail)
}

func (h *ConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	if err := h.configSvc.Delete(r.Context(), id, userID, ip, ua); err != nil {
		respondDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	detail, err := h.configSvc.Create(r.Context(), req, userID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, detail)
}

func (h *ConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	var req service.CreateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	detail, err := h.configSvc.Update(r.Context(), id, req, userID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, detail)
}

func (h *ConfigHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	cfg, err := h.configSvc.Submit(r.Context(), id, userID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, cfg)
}

func (h *ConfigHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	cfg, err := h.configSvc.Approve(r.Context(), id, userID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, cfg)
}

func (h *ConfigHandler) Clone(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	detail, err := h.configSvc.Clone(r.Context(), id, userID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, detail)
}

func (h *ConfigHandler) Preview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	parentConfig, sniYaml, ipAllowYaml, err := h.configSvc.GenerateConfigFiles(r.Context(), id)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"parent_config": parentConfig,
		"sni_yaml":      sniYaml,
		"ip_allow_yaml": ipAllowYaml,
	})
}

func (h *ConfigHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid config ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	userID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	cfg, err := h.configSvc.Reject(r.Context(), id, userID, req.Reason, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, cfg)
}
