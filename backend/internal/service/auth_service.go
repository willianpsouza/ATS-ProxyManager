package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ats-proxy/proxy-manager/backend/internal/auth"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
)

type AuthService struct {
	users    *repository.UserRepo
	sessions *repository.SessionRepo
	secret   string
}

func NewAuthService(users *repository.UserRepo, sessions *repository.SessionRepo, secret string) *AuthService {
	return &AuthService{users: users, sessions: sessions, secret: secret}
}

type LoginResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	User         UserResponse `json:"user"`
}

type UserResponse struct {
	ID       string          `json:"id"`
	Email    string          `json:"email"`
	Username string          `json:"username"`
	Role     domain.UserRole `json:"role"`
}

func (s *AuthService) Login(ctx context.Context, email, password, ip, ua string) (*LoginResponse, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
	}
	if !user.IsActive {
		return nil, fmt.Errorf("%w: account disabled", domain.ErrUnauthorized)
	}
	if !auth.VerifyPassword(user.PasswordHash, password) {
		return nil, fmt.Errorf("%w: invalid credentials", domain.ErrUnauthorized)
	}

	token, refreshToken, err := auth.GenerateToken(s.secret, user)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	tokenHash := auth.HashToken(token)
	refreshHash := auth.HashToken(refreshToken)

	session := &domain.Session{
		UserID:           user.ID,
		TokenHash:        tokenHash,
		RefreshTokenHash: &refreshHash,
		IPAddress:        &ip,
		UserAgent:        &ua,
		ExpiresAt:        time.Now().Add(auth.TokenExpiry),
	}
	if err := s.sessions.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	_ = s.users.UpdateLastLogin(ctx, user.ID)

	return &LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresIn:    int(auth.TokenExpiry.Seconds()),
		User: UserResponse{
			ID:       user.ID.String(),
			Email:    user.Email,
			Username: user.Username,
			Role:     user.Role,
		},
	}, nil
}

type RefreshResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

func (s *AuthService) Refresh(ctx context.Context, refreshTokenStr string) (*RefreshResponse, error) {
	userID, err := auth.ParseRefreshToken(s.secret, refreshTokenStr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid refresh token", domain.ErrUnauthorized)
	}

	refreshHash := auth.HashToken(refreshTokenStr)
	session, err := s.sessions.GetByRefreshTokenHash(ctx, refreshHash)
	if err != nil {
		return nil, fmt.Errorf("%w: session not found", domain.ErrUnauthorized)
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: user not found", domain.ErrUnauthorized)
	}

	newToken, _, err := auth.GenerateToken(s.secret, user)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	newTokenHash := auth.HashToken(newToken)
	newExpiry := time.Now().Add(auth.TokenExpiry)
	if err := s.sessions.UpdateTokens(ctx, session.ID, newTokenHash, newExpiry); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	return &RefreshResponse{
		Token:     newToken,
		ExpiresIn: int(auth.TokenExpiry.Seconds()),
	}, nil
}

func (s *AuthService) Beacon(ctx context.Context, tokenHash string) error {
	session, err := s.sessions.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("%w: session not found", domain.ErrUnauthorized)
	}
	return s.sessions.UpdateBeacon(ctx, session.ID)
}

func (s *AuthService) Logout(ctx context.Context, tokenHash string) error {
	session, err := s.sessions.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil // already logged out
	}
	return s.sessions.Revoke(ctx, session.ID)
}
