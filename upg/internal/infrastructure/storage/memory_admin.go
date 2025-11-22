package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

type memoryAdminRepository struct {
	mu       sync.RWMutex
	sessions map[int64]entity.AdminSession
	actions  []entity.AdminAction
}

// NewMemoryAdminRepository in-memory admin repository yaratish
func NewMemoryAdminRepository() repository.AdminRepository {
	return &memoryAdminRepository{
		sessions: make(map[int64]entity.AdminSession),
		actions:  []entity.AdminAction{},
	}
}

// CreateSession admin sessiyasini yaratish
func (m *memoryAdminRepository) CreateSession(ctx context.Context, session entity.AdminSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session.LastActivity = time.Now()
	m.sessions[session.UserID] = session
	return nil
}

// GetSession sessiyani olish
func (m *memoryAdminRepository) GetSession(ctx context.Context, userID int64) (*entity.AdminSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[userID]
	if !exists {
		return nil, fmt.Errorf("session not found for user %d", userID)
	}

	return &session, nil
}

// DeleteSession sessiyani o'chirish (logout)
func (m *memoryAdminRepository) DeleteSession(ctx context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, userID)
	return nil
}

// IsAdmin foydalanuvchi admin ekanligini tekshirish
func (m *memoryAdminRepository) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[userID]
	if !exists {
		return false, nil
	}

	// Session timeout tekshirish (24 soat)
	if time.Since(session.LastActivity) > 24*time.Hour {
		return false, nil
	}

	return session.IsAdmin, nil
}

// LogAction admin harakatini loglash
func (m *memoryAdminRepository) LogAction(ctx context.Context, action entity.AdminAction) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.actions = append(m.actions, action)
	return nil
}
