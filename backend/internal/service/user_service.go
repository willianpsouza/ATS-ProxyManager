package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/auth"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
)

type UserService struct {
	users *repository.UserRepo
	audit *repository.AuditRepo
}

func NewUserService(users *repository.UserRepo, audit *repository.AuditRepo) *UserService {
	return &UserService{users: users, audit: audit}
}

func (s *UserService) List(ctx context.Context, role *domain.UserRole, page, limit int) ([]domain.User, int, error) {
	offset := (page - 1) * limit
	return s.users.List(ctx, role, limit, offset)
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

type CreateUserRequest struct {
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Password string          `json:"password"`
	Role     domain.UserRole `json:"role"`
}

func (s *UserService) Create(ctx context.Context, req CreateUserRequest, callerRole domain.UserRole, callerID uuid.UUID, ip, ua string) (*domain.User, error) {
	if !req.Role.IsValid() {
		return nil, fmt.Errorf("%w: invalid role", domain.ErrBadRequest)
	}
	if !callerRole.CanCreate(req.Role) {
		return nil, fmt.Errorf("%w: cannot create user with role %s", domain.ErrForbidden, req.Role)
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("%w: username, email and password are required", domain.ErrBadRequest)
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         req.Role,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	s.logAudit(ctx, &callerID, "user.create", "user", &user.ID, nil, nil, ip, ua)

	return user, nil
}

type UpdateUserRequest struct {
	Username *string          `json:"username,omitempty"`
	Email    *string          `json:"email,omitempty"`
	Role     *domain.UserRole `json:"role,omitempty"`
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, req UpdateUserRequest, callerRole domain.UserRole, callerID uuid.UUID, ip, ua string) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Role != nil && !callerRole.CanCreate(*req.Role) {
		return nil, fmt.Errorf("%w: cannot set role %s", domain.ErrForbidden, *req.Role)
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Role != nil {
		user.Role = *req.Role
	}

	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}

	s.logAudit(ctx, &callerID, "user.update", "user", &id, nil, nil, ip, ua)

	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID, callerRole domain.UserRole, callerID uuid.UUID, ip, ua string) error {
	if callerRole != domain.RoleRoot {
		return fmt.Errorf("%w: only root can delete users", domain.ErrForbidden)
	}
	if id == callerID {
		return fmt.Errorf("%w: cannot delete yourself", domain.ErrBadRequest)
	}

	if err := s.users.Delete(ctx, id); err != nil {
		return err
	}

	s.logAudit(ctx, &callerID, "user.delete", "user", &id, nil, nil, ip, ua)
	return nil
}

func (s *UserService) logAudit(ctx context.Context, userID *uuid.UUID, action, entityType string, entityID *uuid.UUID, oldVal, newVal []byte, ip, ua string) {
	_ = s.audit.Create(ctx, &domain.AuditLog{
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		OldValue:   oldVal,
		NewValue:   newVal,
		IPAddress:  &ip,
		UserAgent:  &ua,
	})
}
