package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func buildPostgresDSNFromEnv() string {
	host := strings.TrimSpace(os.Getenv("POSTGRES_HOST"))
	user := strings.TrimSpace(os.Getenv("POSTGRES_USER"))
	password := os.Getenv("POSTGRES_PASSWORD")
	db := strings.TrimSpace(os.Getenv("POSTGRES_DB"))
	port := strings.TrimSpace(os.Getenv("POSTGRES_PORT"))
	sslmode := strings.TrimSpace(os.Getenv("POSTGRES_SSLMODE"))

	if host == "" || user == "" || db == "" {
		return ""
	}
	if port == "" {
		port = "5432"
	}
	if sslmode == "" {
		sslmode = "disable"
	}

	db = strings.TrimPrefix(db, "/")
	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + db,
	}
	if password == "" {
		u.User = url.User(user)
	} else {
		u.User = url.UserPassword(user, password)
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

// OrderStore buyurtmalarni saqlash va olish uchun
type OrderStore interface {
	Save(ctx context.Context, ord orderStatusInfo) error
	UpdateStatus(ctx context.Context, orderID, status string) error
	Get(ctx context.Context, orderID string) (orderStatusInfo, bool, error)
	ListByUser(ctx context.Context, userID int64) ([]orderStatusInfo, error)
	ListRecent(ctx context.Context, limit int) ([]orderStatusInfo, error)
	DeleteByUser(ctx context.Context, userID int64) error
}

// memoryStore fallback (server ish davomida)
type memoryStore struct {
	data map[string]orderStatusInfo
}

func newMemoryStore() *memoryStore {
	return &memoryStore{data: make(map[string]orderStatusInfo)}
}

func (m *memoryStore) Save(_ context.Context, ord orderStatusInfo) error {
	if ord.CreatedAt.IsZero() {
		ord.CreatedAt = time.Now()
	}
	m.data[ord.OrderID] = ord
	return nil
}

func (m *memoryStore) UpdateStatus(_ context.Context, orderID, status string) error {
	ord, ok := m.data[orderID]
	if !ok {
		return nil
	}
	ord.Status = status
	m.data[orderID] = ord
	return nil
}

func (m *memoryStore) Get(_ context.Context, orderID string) (orderStatusInfo, bool, error) {
	ord, ok := m.data[orderID]
	return ord, ok, nil
}

func (m *memoryStore) ListByUser(_ context.Context, userID int64) ([]orderStatusInfo, error) {
	var res []orderStatusInfo
	for _, v := range m.data {
		if v.UserID == userID {
			res = append(res, v)
		}
	}
	return res, nil
}

func (m *memoryStore) ListRecent(_ context.Context, limit int) ([]orderStatusInfo, error) {
	var res []orderStatusInfo
	for _, v := range m.data {
		res = append(res, v)
	}
	// simple order by CreatedAt desc
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.After(res[j].CreatedAt) })
	if limit > 0 && len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func (m *memoryStore) DeleteByUser(_ context.Context, userID int64) error {
	for k, v := range m.data {
		if v.UserID == userID {
			delete(m.data, k)
		}
	}
	return nil
}

// postgresStore persistent saqlash
type postgresStore struct {
	db *sql.DB
}

func newPostgresStore(dsn string) (*postgresStore, error) {
	db, err := openPostgresWithRetry(dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	schema := `
CREATE TABLE IF NOT EXISTS orders (
	order_id TEXT PRIMARY KEY,
	user_id BIGINT NOT NULL,
	user_chat BIGINT NOT NULL,
	username TEXT,
	phone TEXT,
	location TEXT,
	summary TEXT,
	status_summary TEXT,
	total TEXT,
	delivery TEXT,
	status TEXT,
	is_single BOOLEAN,
	created_at TIMESTAMPTZ DEFAULT NOW()
);`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create orders table: %w", err)
	}
	// Backward compatibility: qo'shimcha ustunlar
	if _, err := db.Exec(`ALTER TABLE orders ADD COLUMN IF NOT EXISTS username TEXT`); err != nil {
		return nil, fmt.Errorf("alter orders add username: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE orders ADD COLUMN IF NOT EXISTS phone TEXT`); err != nil {
		return nil, fmt.Errorf("alter orders add phone: %w", err)
	}
	if _, err := db.Exec(`ALTER TABLE orders ADD COLUMN IF NOT EXISTS location TEXT`); err != nil {
		return nil, fmt.Errorf("alter orders add location: %w", err)
	}

	return &postgresStore{db: db}, nil
}

func (p *postgresStore) Save(ctx context.Context, ord orderStatusInfo) error {
	if ord.CreatedAt.IsZero() {
		ord.CreatedAt = time.Now()
	}
	_, err := p.db.ExecContext(ctx, `
	INSERT INTO orders (order_id, user_id, user_chat, username, phone, location, summary, status_summary, total, delivery, status, is_single)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	ON CONFLICT (order_id) DO UPDATE SET
		location=EXCLUDED.location,
		summary=EXCLUDED.summary,
		status_summary=EXCLUDED.status_summary,
		total=EXCLUDED.total,
		delivery=EXCLUDED.delivery,
		status=EXCLUDED.status,
		is_single=EXCLUDED.is_single
	`, ord.OrderID, ord.UserID, ord.UserChat, ord.Username, ord.Phone, ord.Location, ord.Summary, ord.StatusSummary, ord.Total, ord.Delivery, ord.Status, ord.IsSingleItem)
	return err
}

func (p *postgresStore) UpdateStatus(ctx context.Context, orderID, status string) error {
	_, err := p.db.ExecContext(ctx, `UPDATE orders SET status=$1 WHERE order_id=$2`, status, orderID)
	return err
}

func (p *postgresStore) Get(ctx context.Context, orderID string) (orderStatusInfo, bool, error) {
	row := p.db.QueryRowContext(ctx, `
	SELECT order_id, user_id, user_chat, username, phone, location, summary, status_summary, total, delivery, status, is_single, created_at
	FROM orders WHERE order_id=$1`, orderID)
	var ord orderStatusInfo
	var isSingle sql.NullBool
	var created time.Time
	err := row.Scan(&ord.OrderID, &ord.UserID, &ord.UserChat, &ord.Username, &ord.Phone, &ord.Location, &ord.Summary, &ord.StatusSummary, &ord.Total, &ord.Delivery, &ord.Status, &isSingle, &created)
	if err == sql.ErrNoRows {
		return orderStatusInfo{}, false, nil
	}
	if err != nil {
		return orderStatusInfo{}, false, err
	}
	if isSingle.Valid {
		ord.IsSingleItem = isSingle.Bool
	}
	ord.CreatedAt = created
	return ord, true, nil
}

func (p *postgresStore) ListByUser(ctx context.Context, userID int64) ([]orderStatusInfo, error) {
	rows, err := p.db.QueryContext(ctx, `
	SELECT order_id, user_id, user_chat, username, phone, location, summary, status_summary, total, delivery, status, is_single, created_at
	FROM orders WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []orderStatusInfo
	for rows.Next() {
		var ord orderStatusInfo
		var isSingle sql.NullBool
		var created time.Time
		if err := rows.Scan(&ord.OrderID, &ord.UserID, &ord.UserChat, &ord.Username, &ord.Phone, &ord.Location, &ord.Summary, &ord.StatusSummary, &ord.Total, &ord.Delivery, &ord.Status, &isSingle, &created); err != nil {
			return nil, err
		}
		if isSingle.Valid {
			ord.IsSingleItem = isSingle.Bool
		}
		ord.CreatedAt = created
		res = append(res, ord)
	}
	return res, nil
}

func (p *postgresStore) ListRecent(ctx context.Context, limit int) ([]orderStatusInfo, error) {
	rows, err := p.db.QueryContext(ctx, `
	SELECT order_id, user_id, user_chat, username, phone, location, summary, status_summary, total, delivery, status, is_single, created_at
	FROM orders ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []orderStatusInfo
	for rows.Next() {
		var ord orderStatusInfo
		var isSingle sql.NullBool
		var created time.Time
		if err := rows.Scan(&ord.OrderID, &ord.UserID, &ord.UserChat, &ord.Username, &ord.Phone, &ord.Location, &ord.Summary, &ord.StatusSummary, &ord.Total, &ord.Delivery, &ord.Status, &isSingle, &created); err != nil {
			return nil, err
		}
		if isSingle.Valid {
			ord.IsSingleItem = isSingle.Bool
		}
		ord.CreatedAt = created
		res = append(res, ord)
	}
	return res, nil
}

func (p *postgresStore) DeleteByUser(ctx context.Context, userID int64) error {
	_, err := p.db.ExecContext(ctx, `DELETE FROM orders WHERE user_id = $1`, userID)
	return err
}

// newOrderStoreFromEnv DSN berilsa Postgres, aks holda memory
func newOrderStoreFromEnv() (OrderStore, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if strings.TrimSpace(dsn) == "" {
		dsn = buildPostgresDSNFromEnv()
	}
	if strings.TrimSpace(dsn) == "" {
		return newMemoryStore(), nil
	}
	store, err := newPostgresStore(dsn)
	if err != nil {
		log.Printf("order store: Postgres ulanmadi, memoryStore ga qaytdi: %v", err)
		return newMemoryStore(), nil
	}
	return store, nil
}
