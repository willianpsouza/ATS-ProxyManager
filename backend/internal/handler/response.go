package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type paginatedResponse struct {
	Data       interface{}       `json:"data"`
	Pagination domain.Pagination `json:"pagination"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, errorResponse{Error: code, Message: message})
}

func respondDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		respondError(w, http.StatusForbidden, "forbidden", err.Error())
	case errors.Is(err, domain.ErrUnauthorized):
		respondError(w, http.StatusUnauthorized, "unauthorized", err.Error())
	case errors.Is(err, domain.ErrInvalidStatus):
		respondError(w, http.StatusBadRequest, "invalid_status", err.Error())
	case errors.Is(err, domain.ErrConflict):
		respondError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, domain.ErrBadRequest):
		respondError(w, http.StatusBadRequest, "bad_request", err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}

func parsePagination(r *http.Request) (page, limit int) {
	page = 1
	limit = 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	return
}

func paginationOffset(page, limit int) int {
	return (page - 1) * limit
}

func totalPages(total, limit int) int {
	if limit == 0 {
		return 0
	}
	tp := total / limit
	if total%limit > 0 {
		tp++
	}
	return tp
}
