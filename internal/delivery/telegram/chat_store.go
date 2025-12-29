package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type chatLogMessage struct {
	ID        int64
	UserID    int64
	ChatID    int64
	Username  string
	Direction string
	Text      string
	MessageID int
	CreatedAt time.Time
}

type chatUserEntry struct {
	UserID   int64
	Username string
	LastSeen time.Time
}

type ChatStore interface {
	Save(ctx context.Context, msg chatLogMessage) error
	ListByUser(ctx context.Context, userID int64, limit int) ([]chatLogMessage, error)
	ListByUsername(ctx context.Context, username string, limit int) ([]chatLogMessage, error)
}

type memoryChatStore struct {
	data []chatLogMessage
}

func newMemoryChatStore() *memoryChatStore {
	return &memoryChatStore{data: make([]chatLogMessage, 0, 256)}
}

func (m *memoryChatStore) Save(_ context.Context, msg chatLogMessage) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	m.data = append(m.data, msg)
	return nil
}

func (m *memoryChatStore) ListByUser(_ context.Context, userID int64, limit int) ([]chatLogMessage, error) {
	var res []chatLogMessage
	for i := len(m.data) - 1; i >= 0; i-- {
		item := m.data[i]
		if item.UserID == userID {
			res = append(res, item)
			if limit > 0 && len(res) >= limit {
				break
			}
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.Before(res[j].CreatedAt) })
	return res, nil
}

func (m *memoryChatStore) ListByUsername(_ context.Context, username string, limit int) ([]chatLogMessage, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		return nil, nil
	}
	var res []chatLogMessage
	for i := len(m.data) - 1; i >= 0; i-- {
		item := m.data[i]
		if strings.ToLower(strings.TrimSpace(item.Username)) == username {
			res = append(res, item)
			if limit > 0 && len(res) >= limit {
				break
			}
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.Before(res[j].CreatedAt) })
	return res, nil
}

func (m *memoryChatStore) SearchUsers(_ context.Context, query string, limit int) ([]chatUserEntry, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = userHistoryInlineResultsLimit
	}
	entries := make(map[int64]chatUserEntry)
	for _, item := range m.data {
		entry := entries[item.UserID]
		if entry.UserID == 0 || item.CreatedAt.After(entry.LastSeen) {
			entry.UserID = item.UserID
			entry.LastSeen = item.CreatedAt
		}
		if entry.Username == "" && strings.TrimSpace(item.Username) != "" {
			entry.Username = strings.TrimSpace(item.Username)
		}
		entries[item.UserID] = entry
	}

	res := make([]chatUserEntry, 0, limit)
	for _, entry := range entries {
		if !matchUserQuery(entry.UserID, entry.Username, q) {
			continue
		}
		res = append(res, entry)
	}

	sortChatUserEntries(res)
	if len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

type postgresChatStore struct {
	db *sql.DB
}

func newPostgresChatStore(dsn string) (*postgresChatStore, error) {
	db, err := openPostgresWithRetry(dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	schema := `
CREATE TABLE IF NOT EXISTS chat_messages (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL,
	chat_id BIGINT NOT NULL,
	username TEXT,
	direction TEXT NOT NULL,
	message_id BIGINT,
	text TEXT,
	created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_time ON chat_messages (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_username_time ON chat_messages (lower(username), created_at DESC);
`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create chat_messages table: %w", err)
	}

	return &postgresChatStore{db: db}, nil
}

func (p *postgresChatStore) Save(ctx context.Context, msg chatLogMessage) error {
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	_, err := p.db.ExecContext(ctx, `
	INSERT INTO chat_messages (user_id, chat_id, username, direction, message_id, text, created_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, msg.UserID, msg.ChatID, msg.Username, msg.Direction, msg.MessageID, msg.Text, msg.CreatedAt)
	return err
}

func (p *postgresChatStore) ListByUser(ctx context.Context, userID int64, limit int) ([]chatLogMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.db.QueryContext(ctx, `
	SELECT id, user_id, chat_id, username, direction, message_id, text, created_at
	FROM chat_messages
	WHERE user_id = $1
	ORDER BY created_at DESC
	LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []chatLogMessage
	for rows.Next() {
		var msg chatLogMessage
		if err := rows.Scan(&msg.ID, &msg.UserID, &msg.ChatID, &msg.Username, &msg.Direction, &msg.MessageID, &msg.Text, &msg.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, msg)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.Before(res[j].CreatedAt) })
	return res, nil
}

func (p *postgresChatStore) ListByUsername(ctx context.Context, username string, limit int) ([]chatLogMessage, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.db.QueryContext(ctx, `
	SELECT id, user_id, chat_id, username, direction, message_id, text, created_at
	FROM chat_messages
	WHERE lower(username) = lower($1)
	ORDER BY created_at DESC
	LIMIT $2`, username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []chatLogMessage
	for rows.Next() {
		var msg chatLogMessage
		if err := rows.Scan(&msg.ID, &msg.UserID, &msg.ChatID, &msg.Username, &msg.Direction, &msg.MessageID, &msg.Text, &msg.CreatedAt); err != nil {
			return nil, err
		}
		res = append(res, msg)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.Before(res[j].CreatedAt) })
	return res, nil
}

func (p *postgresChatStore) SearchUsers(ctx context.Context, query string, limit int) ([]chatUserEntry, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = userHistoryInlineResultsLimit
	}
	like := "%" + strings.ToLower(q) + "%"
	idLike := "%" + q + "%"

	rows, err := p.db.QueryContext(ctx, `
	SELECT user_id, username, created_at
	FROM chat_messages
	WHERE lower(username) LIKE $1 OR CAST(user_id as text) LIKE $2
	ORDER BY created_at DESC
	LIMIT $3
	`, like, idLike, limit*5)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[int64]struct{})
	res := make([]chatUserEntry, 0, limit)
	for rows.Next() {
		var entry chatUserEntry
		if err := rows.Scan(&entry.UserID, &entry.Username, &entry.LastSeen); err != nil {
			return nil, err
		}
		if _, ok := seen[entry.UserID]; ok {
			continue
		}
		seen[entry.UserID] = struct{}{}
		if entry.Username != "" {
			entry.Username = strings.TrimSpace(entry.Username)
		}
		if !matchUserQuery(entry.UserID, entry.Username, strings.ToLower(q)) {
			continue
		}
		res = append(res, entry)
		if len(res) >= limit {
			break
		}
	}

	sortChatUserEntries(res)
	return res, nil
}

func newChatStoreFromEnv() (ChatStore, error) {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		dsn = buildPostgresDSNFromEnv()
	}
	if strings.TrimSpace(dsn) == "" {
		return newMemoryChatStore(), nil
	}
	store, err := newPostgresChatStore(dsn)
	if err != nil {
		log.Printf("chat store: Postgres ulanmadi, memoryStore ga qaytdi: %v", err)
		return newMemoryChatStore(), nil
	}
	return store, nil
}
