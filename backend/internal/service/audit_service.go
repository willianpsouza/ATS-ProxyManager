package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
)

type AuditService struct {
	audit *repository.AuditRepo
	users *repository.UserRepo
}

func NewAuditService(audit *repository.AuditRepo, users *repository.UserRepo) *AuditService {
	return &AuditService{audit: audit, users: users}
}

type AuditListItem struct {
	ID         string       `json:"id"`
	User       *UserResponse `json:"user,omitempty"`
	Action     string        `json:"action"`
	EntityType string        `json:"entity_type"`
	EntityID   *string       `json:"entity_id,omitempty"`
	Changes    interface{}   `json:"changes,omitempty"`
	IPAddress  *string       `json:"ip_address,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
}

func (s *AuditService) List(ctx context.Context, entityType *string, entityID, userID *uuid.UUID, from, to *time.Time, page, limit int) ([]AuditListItem, int, error) {
	offset := (page - 1) * limit
	filter := repository.AuditFilter{
		EntityType: entityType,
		EntityID:   entityID,
		UserID:     userID,
		From:       from,
		To:         to,
	}

	logs, total, err := s.audit.List(ctx, filter, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	items := make([]AuditListItem, 0, len(logs))
	for _, l := range logs {
		item := AuditListItem{
			ID:         l.ID.String(),
			Action:     l.Action,
			EntityType: l.EntityType,
			IPAddress:  l.IPAddress,
			CreatedAt:  l.CreatedAt,
		}
		if l.EntityID != nil {
			eid := l.EntityID.String()
			item.EntityID = &eid
		}
		if l.UserID != nil {
			user, err := s.users.GetByID(ctx, *l.UserID)
			if err == nil {
				item.User = &UserResponse{
					ID:       user.ID.String(),
					Email:    user.Email,
					Username: user.Username,
					Role:     user.Role,
				}
			}
		}
		if l.NewValue != nil {
			item.Changes = map[string]interface{}{
				"old": jsonRawOrNil(l.OldValue),
				"new": jsonRawOrNil(l.NewValue),
			}
		}
		items = append(items, item)
	}

	return items, total, nil
}

func jsonRawOrNil(data []byte) interface{} {
	if data == nil {
		return nil
	}
	return string(data)
}
