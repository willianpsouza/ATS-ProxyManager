package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/service"
)

type AuditHandler struct {
	auditSvc *service.AuditService
}

func NewAuditHandler(auditSvc *service.AuditService) *AuditHandler {
	return &AuditHandler{auditSvc: auditSvc}
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	var entityType *string
	if et := r.URL.Query().Get("entity_type"); et != "" {
		entityType = &et
	}

	var entityID *uuid.UUID
	if eid := r.URL.Query().Get("entity_id"); eid != "" {
		if id, err := uuid.Parse(eid); err == nil {
			entityID = &id
		}
	}

	var userID *uuid.UUID
	if uid := r.URL.Query().Get("user_id"); uid != "" {
		if id, err := uuid.Parse(uid); err == nil {
			userID = &id
		}
	}

	var from, to *time.Time
	if f := r.URL.Query().Get("from"); f != "" {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			from = &t
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = &parsed
		}
	}

	items, total, err := h.auditSvc.List(r.Context(), entityType, entityID, userID, from, to, page, limit)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	if items == nil {
		items = []service.AuditListItem{}
	}

	respondJSON(w, http.StatusOK, paginatedResponse{
		Data: items,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages(total, limit),
		},
	})
}
