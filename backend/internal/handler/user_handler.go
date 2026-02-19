package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/service"
)

type UserHandler struct {
	userSvc *service.UserService
}

func NewUserHandler(userSvc *service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	var roleFilter *domain.UserRole
	if roleStr := r.URL.Query().Get("role"); roleStr != "" {
		role := domain.UserRole(roleStr)
		if role.IsValid() {
			roleFilter = &role
		}
	}

	users, total, err := h.userSvc.List(r.Context(), roleFilter, page, limit)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	type userItem struct {
		ID        string          `json:"id"`
		Username  string          `json:"username"`
		Email     string          `json:"email"`
		Role      domain.UserRole `json:"role"`
		CreatedAt string          `json:"created_at"`
		LastLogin *string         `json:"last_login,omitempty"`
	}

	data := make([]userItem, 0, len(users))
	for _, u := range users {
		item := userItem{
			ID:        u.ID.String(),
			Username:  u.Username,
			Email:     u.Email,
			Role:      u.Role,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		if u.LastLogin != nil {
			ll := u.LastLogin.Format("2006-01-02T15:04:05Z")
			item.LastLogin = &ll
		}
		data = append(data, item)
	}

	respondJSON(w, http.StatusOK, paginatedResponse{
		Data: data,
		Pagination: domain.Pagination{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages(total, limit),
		},
	})
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	callerRole := getUserRole(r.Context())
	callerID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	user, err := h.userSvc.Create(r.Context(), req, callerRole, callerID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         user.ID.String(),
		"username":   user.Username,
		"email":      user.Email,
		"role":       user.Role,
		"created_at": user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	var req service.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	callerRole := getUserRole(r.Context())
	callerID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	user, err := h.userSvc.Update(r.Context(), id, req, callerRole, callerID, ip, ua)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":         user.ID.String(),
		"username":   user.Username,
		"email":      user.Email,
		"role":       user.Role,
		"updated_at": user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "Invalid user ID")
		return
	}

	callerRole := getUserRole(r.Context())
	callerID := getUserID(r.Context())
	ip := clientIP(r)
	ua := r.UserAgent()

	if err := h.userSvc.Delete(r.Context(), id, callerRole, callerID, ip, ua); err != nil {
		respondDomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
