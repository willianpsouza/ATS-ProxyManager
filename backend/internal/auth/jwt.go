package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
)

const (
	TokenExpiry   = 30 * time.Minute
	RefreshExpiry = 24 * time.Hour
)

type Claims struct {
	UserID   uuid.UUID       `json:"user_id"`
	Username string          `json:"username"`
	Email    string          `json:"email"`
	Role     domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(secret string, user *domain.User) (token string, refreshToken string, err error) {
	now := time.Now()

	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = t.SignedString([]byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("sign token: %w", err)
	}

	refreshClaims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(RefreshExpiry)),
		IssuedAt:  jwt.NewNumericDate(now),
		Subject:   user.ID.String(),
		ID:        uuid.New().String(),
	}

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = rt.SignedString([]byte(secret))
	if err != nil {
		return "", "", fmt.Errorf("sign refresh token: %w", err)
	}

	return token, refreshToken, nil
}

func ParseToken(secret string, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

func ParseRefreshToken(secret string, tokenStr string) (userID uuid.UUID, err error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse refresh token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid refresh token")
	}

	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid subject in refresh token: %w", err)
	}

	return uid, nil
}

// HashToken returns SHA256 hex digest of a token string for DB storage.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
