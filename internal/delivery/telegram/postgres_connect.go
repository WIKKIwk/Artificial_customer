package telegram

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	postgresConnectAttemptsDefault = 20
	postgresConnectDelayDefault    = 2 * time.Second
)

type postgresDSNInfo struct {
	User     string
	Password string
	Host     string
	Port     string
	DBName   string
	SSLMode  string
}

func openPostgresWithRetry(dsn string) (*sql.DB, error) {
	attempts := getenvInt("POSTGRES_CONNECT_MAX_ATTEMPTS", postgresConnectAttemptsDefault)
	delaySeconds := getenvInt("POSTGRES_CONNECT_RETRY_SECONDS", int(postgresConnectDelayDefault/time.Second))
	delay := time.Duration(delaySeconds) * time.Second
	if attempts <= 0 {
		attempts = postgresConnectAttemptsDefault
	}
	if delay <= 0 {
		delay = postgresConnectDelayDefault
	}

	var lastErr error
	created := false
	for attempt := 1; attempt <= attempts; attempt++ {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			if pingErr := db.Ping(); pingErr == nil {
				return db, nil
			} else {
				err = pingErr
			}
		}
		if db != nil {
			_ = db.Close()
		}
		lastErr = err
		if !created && isDatabaseMissingError(err) {
			if createErr := ensurePostgresDatabase(dsn); createErr == nil {
				created = true
				continue
			} else {
				lastErr = createErr
			}
		}
		if attempt < attempts {
			time.Sleep(delay)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("postgres connection failed")
	}
	return nil, lastErr
}

func ensurePostgresDatabase(dsn string) error {
	info, ok := parsePostgresDSNInfo(dsn)
	if !ok || info.DBName == "" || info.Host == "" || info.User == "" {
		return fmt.Errorf("database info not found in dsn")
	}
	baseDSN := info.buildURL("postgres")
	if err := createPostgresDatabaseWithDSN(baseDSN, info.DBName); err == nil {
		return nil
	} else {
		adminDSN := adminPostgresDSNFromEnv(info)
		if adminDSN != "" && adminDSN != baseDSN {
			if err2 := createPostgresDatabaseWithDSN(adminDSN, info.DBName); err2 == nil {
				return nil
			} else {
				return err2
			}
		}
		return err
	}
}

func parsePostgresDSNInfo(dsn string) (postgresDSNInfo, bool) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return postgresDSNInfo{}, false
	}
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		if info, ok := parsePostgresURL(trimmed); ok {
			return info, true
		}
	}
	return parsePostgresKeyValue(trimmed)
}

func parsePostgresURL(raw string) (postgresDSNInfo, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return postgresDSNInfo{}, false
	}
	info := postgresDSNInfo{
		User:   "",
		Host:   u.Hostname(),
		Port:   u.Port(),
		DBName: strings.TrimPrefix(u.Path, "/"),
		SSLMode: func() string {
			if u.Query() != nil {
				return u.Query().Get("sslmode")
			}
			return ""
		}(),
	}
	if u.User != nil {
		info.User = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			info.Password = pass
		}
	}
	if info.Port == "" {
		info.Port = "5432"
	}
	if info.SSLMode == "" {
		info.SSLMode = "disable"
	}
	return info, true
}

func parsePostgresKeyValue(raw string) (postgresDSNInfo, bool) {
	info := postgresDSNInfo{}
	parts := strings.Fields(raw)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.Trim(kv[1], `"'`)
		switch key {
		case "user", "username":
			info.User = val
		case "password":
			info.Password = val
		case "host":
			info.Host = val
		case "port":
			info.Port = val
		case "dbname", "database":
			info.DBName = val
		case "sslmode":
			info.SSLMode = val
		}
	}
	if info.Port == "" {
		info.Port = "5432"
	}
	if info.SSLMode == "" {
		info.SSLMode = "disable"
	}
	if info.Host == "" && info.User == "" && info.DBName == "" {
		return postgresDSNInfo{}, false
	}
	return info, true
}

func (p postgresDSNInfo) buildURL(dbName string) string {
	host := p.Host
	port := p.Port
	if port == "" {
		port = "5432"
	}
	if host != "" && port != "" {
		host = net.JoinHostPort(host, port)
	}
	u := url.URL{
		Scheme: "postgres",
		Host:   host,
		Path:   "/" + dbName,
	}
	if p.User != "" {
		if p.Password != "" {
			u.User = url.UserPassword(p.User, p.Password)
		} else {
			u.User = url.User(p.User)
		}
	}
	q := u.Query()
	if strings.TrimSpace(p.SSLMode) != "" {
		q.Set("sslmode", p.SSLMode)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func adminPostgresDSNFromEnv(base postgresDSNInfo) string {
	if dsn := strings.TrimSpace(getenvAny("POSTGRES_ADMIN_DSN", "DB_ADMIN_DSN")); dsn != "" {
		return dsn
	}
	adminUser := strings.TrimSpace(getenvAny("POSTGRES_ADMIN_USER", "DB_ADMIN_USER"))
	if adminUser == "" {
		return ""
	}
	adminPassword := getenvAny("POSTGRES_ADMIN_PASSWORD", "DB_ADMIN_PASSWORD")
	adminHost := strings.TrimSpace(getenvAny("POSTGRES_ADMIN_HOST", "DB_ADMIN_HOST"))
	adminPort := strings.TrimSpace(getenvAny("POSTGRES_ADMIN_PORT", "DB_ADMIN_PORT"))
	adminSSL := strings.TrimSpace(getenvAny("POSTGRES_ADMIN_SSLMODE", "DB_ADMIN_SSLMODE"))
	adminDB := strings.TrimSpace(getenvAny("POSTGRES_ADMIN_DB", "DB_ADMIN_DB", "POSTGRES_ADMIN_DATABASE", "DB_ADMIN_DATABASE"))
	if adminDB == "" {
		adminDB = "postgres"
	}

	info := base
	info.User = adminUser
	info.Password = adminPassword
	if adminHost != "" {
		info.Host = adminHost
	}
	if adminPort != "" {
		info.Port = adminPort
	}
	if adminSSL != "" {
		info.SSLMode = adminSSL
	}
	return info.buildURL(adminDB)
}

func createPostgresDatabaseWithDSN(dsn, dbName string) error {
	if strings.TrimSpace(dsn) == "" || strings.TrimSpace(dbName) == "" {
		return fmt.Errorf("admin dsn or db name missing")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return err
	}
	query := fmt.Sprintf("CREATE DATABASE %s", quoteIdentifier(dbName))
	if _, err := db.Exec(query); err != nil {
		if isDatabaseExistsError(err) {
			return nil
		}
		return err
	}
	return nil
}

func isDatabaseMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") && strings.Contains(msg, "database")
}

func isDatabaseExistsError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") && strings.Contains(msg, "database")
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func getenvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return val
}
