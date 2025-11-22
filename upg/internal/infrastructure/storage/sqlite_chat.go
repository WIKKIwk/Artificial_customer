package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

type sqliteChatRepository struct {
	db      *sql.DB
	maxSize int
}

// NewSQLiteChatRepository SQLite asosidagi chat repository
func NewSQLiteChatRepository(dbPath string, maxContextSize int) (repository.ChatRepository, error) {
	if dbPath == "" {
		return nil, errors.New("db path bo'sh bo'lmasligi kerak")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("db papkasini yaratib bo'lmadi: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite ochilmadi: %w", err)
	}

	if err := createChatSchema(db); err != nil {
		return nil, err
	}

	return &sqliteChatRepository{db: db, maxSize: maxContextSize}, nil
}

func createChatSchema(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS messages (
	id TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL,
	username TEXT,
	text TEXT,
	response TEXT,
	ts TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_user_ts ON messages (user_id, ts);
`
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("schema yaratib bo'lmadi: %w", err)
	}
	return nil
}

// SaveMessage xabarni saqlash
func (s *sqliteChatRepository) SaveMessage(ctx context.Context, message entity.Message) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO messages (id, user_id, username, text, response, ts) VALUES (?, ?, ?, ?, ?, ?)`,
		message.ID, message.UserID, message.Username, message.Text, message.Response, message.Timestamp)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Eski xabarlarni kesish
	_, err = tx.ExecContext(ctx, `
DELETE FROM messages
WHERE id IN (
  SELECT id FROM messages
  WHERE user_id = ?
  ORDER BY ts DESC
  LIMIT -1 OFFSET ?
)`, message.UserID, s.maxSize)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// GetHistory foydalanuvchi chat tarixini olish
func (s *sqliteChatRepository) GetHistory(ctx context.Context, userID int64, limit int) ([]entity.Message, error) {
	query := `SELECT id, user_id, username, text, response, ts FROM messages WHERE user_id = ? ORDER BY ts DESC`
	args := []any{userID}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tmp []entity.Message
	for rows.Next() {
		var msg entity.Message
		var ts time.Time
		if err := rows.Scan(&msg.ID, &msg.UserID, &msg.Username, &msg.Text, &msg.Response, &ts); err != nil {
			return nil, err
		}
		msg.Timestamp = ts
		tmp = append(tmp, msg)
	}

	// Reverse to ASC to keep eski->yangi tartib
	for i, j := 0, len(tmp)-1; i < j; i, j = i+1, j-1 {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}

	return tmp, rows.Err()
}

// GetAllMessages barcha xabarlarni olish (admin ko'rishi uchun)
func (s *sqliteChatRepository) GetAllMessages(ctx context.Context, limit int) ([]entity.Message, error) {
	query := `SELECT id, user_id, username, text, response, ts FROM messages ORDER BY ts DESC`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []entity.Message
	for rows.Next() {
		var msg entity.Message
		var ts time.Time
		if err := rows.Scan(&msg.ID, &msg.UserID, &msg.Username, &msg.Text, &msg.Response, &ts); err != nil {
			return nil, err
		}
		msg.Timestamp = ts
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// ClearHistory foydalanuvchi tarixini tozalash
func (s *sqliteChatRepository) ClearHistory(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM messages WHERE user_id = ?`, userID)
	return err
}

// ClearAll barcha chat tarixlarini tozalash
func (s *sqliteChatRepository) ClearAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM messages`)
	return err
}

// GetContext foydalanuvchi chat kontekstini olish
func (s *sqliteChatRepository) GetContext(ctx context.Context, userID int64) (*entity.ChatContext, error) {
	msgs, err := s.GetHistory(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("context not found for user %d", userID)
	}
	return &entity.ChatContext{
		UserID:   userID,
		Messages: msgs,
		LastUsed: msgs[len(msgs)-1].Timestamp,
	}, nil
}
