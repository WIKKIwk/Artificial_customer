package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

type memoryChatRepository struct {
	mu       sync.RWMutex
	contexts map[int64]*entity.ChatContext
	maxSize  int
}

// NewMemoryChatRepository in-memory chat repository yaratish
func NewMemoryChatRepository(maxContextSize int) repository.ChatRepository {
	return &memoryChatRepository{
		contexts: make(map[int64]*entity.ChatContext),
		maxSize:  maxContextSize,
	}
}

// SaveMessage xabarni saqlash
func (m *memoryChatRepository) SaveMessage(ctx context.Context, message entity.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	chatCtx, exists := m.contexts[message.UserID]
	if !exists {
		chatCtx = &entity.ChatContext{
			UserID:   message.UserID,
			Messages: []entity.Message{},
			LastUsed: time.Now(),
		}
		m.contexts[message.UserID] = chatCtx
	}

	chatCtx.Messages = append(chatCtx.Messages, message)
	chatCtx.LastUsed = time.Now()

	// Maksimal hajmni nazorat qilish
	if len(chatCtx.Messages) > m.maxSize {
		chatCtx.Messages = chatCtx.Messages[len(chatCtx.Messages)-m.maxSize:]
	}

	return nil
}

// GetHistory foydalanuvchi chat tarixini olish
func (m *memoryChatRepository) GetHistory(ctx context.Context, userID int64, limit int) ([]entity.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chatCtx, exists := m.contexts[userID]
	if !exists {
		return []entity.Message{}, nil
	}

	messages := chatCtx.Messages
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	// Return a defensive copy so callers can safely iterate without holding the lock.
	out := make([]entity.Message, len(messages))
	copy(out, messages)
	return out, nil
}

// ClearHistory foydalanuvchi tarixini tozalash
func (m *memoryChatRepository) ClearHistory(ctx context.Context, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.contexts, userID)
	return nil
}

// ClearAll barcha chat tarixlarini tozalash
func (m *memoryChatRepository) ClearAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.contexts = make(map[int64]*entity.ChatContext)
	return nil
}

// GetContext foydalanuvchi chat kontekstini olish
func (m *memoryChatRepository) GetContext(ctx context.Context, userID int64) (*entity.ChatContext, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chatCtx, exists := m.contexts[userID]
	if !exists {
		return nil, fmt.Errorf("context not found for user %d", userID)
	}

	return chatCtx, nil
}
