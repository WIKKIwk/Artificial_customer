package telegram

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	sheetMasterAboutUserName          = "about user"
	sheetMasterAboutUserSelectionFile = "data/sheetmaster_about_user.json"
	aboutUserSyncDebounce             = 3 * time.Second
	aboutUserSyncMinInterval           = 15 * time.Second
)

type sheetMasterAboutUserSelection struct {
	FileID    uint      `json:"file_id"`
	Name      string    `json:"name,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type sheetMasterSaveFileInput struct {
	ID    *uint           `json:"id,omitempty"`
	Name  string          `json:"name"`
	State json.RawMessage `json:"state"`
}

type sheetMasterSaveFileResponse struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	AccessRole string `json:"access_role"`
}

type sheetMasterFileDetailResponse struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	UpdatedAt  time.Time `json:"updated_at"`
	AccessRole string    `json:"access_role"`
}

func (h *BotHandler) ensureAboutUserSheetOnStart(ctx context.Context) {
	if h == nil {
		return
	}
	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := h.ensureAboutUserSheet(startCtx); err != nil {
		log.Printf("[about_user] ensure sheet on start failed: %v", err)
	}
}

func (h *BotHandler) syncAboutUserSheet(ctx context.Context, rows []userExportRow) error {
	if cfg, err := h.resolveSheetMasterConfig(); err == nil {
		if apiErr := h.syncAboutUserSheetAPI(ctx, cfg, rows); apiErr == nil {
			return nil
		} else {
			log.Printf("[about_user] api sync error: %v", apiErr)
		}
	}
	return h.syncAboutUserSheetDB(ctx, rows)
}

func (h *BotHandler) ensureAboutUserSheet(ctx context.Context) (sheetMasterFileMeta, error) {
	if cfg, err := h.resolveSheetMasterConfig(); err == nil {
		if meta, apiErr := h.ensureAboutUserSheetAPI(ctx, cfg); apiErr == nil {
			return meta, nil
		} else {
			log.Printf("[about_user] api ensure error: %v", apiErr)
		}
	}
	return h.ensureAboutUserSheetDB(ctx)
}

func (h *BotHandler) scheduleAboutUserSheetSync(reason string) {
	if h == nil {
		return
	}
	if !h.aboutUserSyncEnabled() {
		return
	}
	h.aboutUserSyncMu.Lock()
	h.aboutUserSyncPending = true
	if h.aboutUserSyncTimer != nil {
		h.aboutUserSyncMu.Unlock()
		return
	}
	h.aboutUserSyncTimer = time.AfterFunc(aboutUserSyncDebounce, func() {
		h.runAboutUserSheetSync(reason)
	})
	h.aboutUserSyncMu.Unlock()
}

func (h *BotHandler) runAboutUserSheetSync(reason string) {
	if h == nil {
		return
	}
	h.aboutUserSyncMu.Lock()
	if !h.aboutUserSyncPending {
		h.aboutUserSyncTimer = nil
		h.aboutUserSyncMu.Unlock()
		return
	}
	now := time.Now()
	if !h.aboutUserSyncLast.IsZero() {
		wait := aboutUserSyncMinInterval - now.Sub(h.aboutUserSyncLast)
		if wait > 0 {
			h.aboutUserSyncTimer = time.AfterFunc(wait, func() {
				h.runAboutUserSheetSync(reason)
			})
			h.aboutUserSyncMu.Unlock()
			return
		}
	}
	h.aboutUserSyncPending = false
	h.aboutUserSyncLast = now
	h.aboutUserSyncTimer = nil
	h.aboutUserSyncMu.Unlock()

	rows := h.buildUserExportRows()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	h.sheetMasterSyncMu.Lock()
	err := h.syncAboutUserSheet(ctx, rows)
	h.sheetMasterSyncMu.Unlock()
	if err != nil {
		log.Printf("[about_user] auto sync failed (%s): %v", reason, err)
	}
}

func (h *BotHandler) aboutUserSyncEnabled() bool {
	if h == nil {
		return false
	}
	if _, err := h.resolveSheetMasterConfig(); err == nil {
		return true
	}
	if _, err := sheetMasterDBDSNFromEnv(); err == nil {
		return true
	}
	return false
}

func (h *BotHandler) syncAboutUserSheetAPI(ctx context.Context, cfg sheetMasterConfig, rows []userExportRow) error {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(cfg.APIKey) == "" {
		return errSheetMasterNotConfigured
	}

	syncCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	meta, err := h.ensureAboutUserSheetAPI(syncCtx, cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = sheetMasterAboutUserName
	}

	state, err := buildAboutUserSheetState(rows)
	if err != nil {
		return err
	}

	if _, err := sheetMasterSaveFile(syncCtx, baseURL, cfg.APIKey, meta.ID, meta.Name, state); err != nil {
		if isSheetMasterNotFoundErr(err) {
			_ = clearSheetMasterAboutUserSelection()
			meta, metaErr := h.ensureAboutUserSheetAPI(syncCtx, cfg)
			if metaErr != nil {
				return metaErr
			}
			if _, err := sheetMasterSaveFile(syncCtx, baseURL, cfg.APIKey, meta.ID, meta.Name, state); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func (h *BotHandler) ensureAboutUserSheetAPI(ctx context.Context, cfg sheetMasterConfig) (sheetMasterFileMeta, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" || strings.TrimSpace(cfg.APIKey) == "" {
		return sheetMasterFileMeta{}, errSheetMasterNotConfigured
	}

	if sel, ok, err := loadSheetMasterAboutUserSelection(); err == nil && ok && sel.FileID != 0 {
		if meta, err := sheetMasterGetFileMeta(ctx, baseURL, cfg.APIKey, sel.FileID); err == nil {
			return meta, nil
		}
		_ = clearSheetMasterAboutUserSelection()
	}

	files, err := sheetMasterListFilesWithLimit(ctx, cfg, 100)
	if err == nil {
		for _, f := range files {
			if normalizeSheetName(f.Name) == normalizeSheetName(sheetMasterAboutUserName) {
				_ = saveSheetMasterAboutUserSelection(sheetMasterAboutUserSelection{
					FileID:    f.ID,
					Name:      f.Name,
					UpdatedAt: f.UpdatedAt,
				})
				return f, nil
			}
		}
	}

	state, err := buildAboutUserSheetState(nil)
	if err != nil {
		return sheetMasterFileMeta{}, err
	}
	meta, err := sheetMasterSaveFile(ctx, baseURL, cfg.APIKey, 0, sheetMasterAboutUserName, state)
	if err != nil {
		return sheetMasterFileMeta{}, err
	}
	_ = saveSheetMasterAboutUserSelection(sheetMasterAboutUserSelection{
		FileID:    meta.ID,
		Name:      meta.Name,
		UpdatedAt: meta.UpdatedAt,
	})
	return meta, nil
}

func (h *BotHandler) syncAboutUserSheetDB(ctx context.Context, rows []userExportRow) error {
	state, err := buildAboutUserSheetState(rows)
	if err != nil {
		return err
	}

	meta, err := h.ensureAboutUserSheetDB(ctx)
	if err != nil {
		return err
	}

	return sheetMasterUpdateFileStateInDB(ctx, meta.ID, state)
}

func (h *BotHandler) ensureAboutUserSheetDB(ctx context.Context) (sheetMasterFileMeta, error) {
	dsn, err := sheetMasterDBDSNFromEnv()
	if err != nil {
		return sheetMasterFileMeta{}, err
	}

	dbCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	if sel, ok, err := loadSheetMasterAboutUserSelection(); err == nil && ok && sel.FileID != 0 {
		if meta, err := sheetMasterGetDBFileMeta(dbCtx, dsn, sel.FileID); err == nil {
			return meta, nil
		}
		_ = clearSheetMasterAboutUserSelection()
	}

	if meta, ok, err := sheetMasterFindAboutUserFileInDB(dbCtx, dsn); err == nil && ok {
		_ = saveSheetMasterAboutUserSelection(sheetMasterAboutUserSelection{
			FileID:    meta.ID,
			Name:      meta.Name,
			UpdatedAt: meta.UpdatedAt,
		})
		return meta, nil
	}

	ownerID, err := sheetMasterResolveOwnerID(dbCtx, dsn)
	if err != nil {
		return sheetMasterFileMeta{}, err
	}

	state, err := buildAboutUserSheetState(nil)
	if err != nil {
		return sheetMasterFileMeta{}, err
	}

	meta, err := sheetMasterCreateFileInDB(dbCtx, dsn, ownerID, sheetMasterAboutUserName, state)
	if err != nil {
		return sheetMasterFileMeta{}, err
	}
	_ = saveSheetMasterAboutUserSelection(sheetMasterAboutUserSelection{
		FileID:    meta.ID,
		Name:      meta.Name,
		UpdatedAt: meta.UpdatedAt,
	})
	return meta, nil
}

func buildAboutUserSheetState(rows []userExportRow) (json.RawMessage, error) {
	headers := userExportHeaders()
	data := make(map[string]any, len(headers)+len(rows)*len(headers))

	for c, h := range headers {
		key := fmt.Sprintf("0,%d", c)
		data[key] = map[string]any{"value": h}
	}

	for r, row := range rows {
		values := userExportRowStrings(row)
		rowIdx := r + 1
		for c, val := range values {
			if strings.TrimSpace(val) == "" {
				continue
			}
			key := fmt.Sprintf("%d,%d", rowIdx, c)
			data[key] = map[string]any{"value": val}
		}
	}

	rowCount := len(rows) + 1
	if rowCount < 1 {
		rowCount = 1
	}
	state := map[string]any{
		"data":         data,
		"rowCount":     rowCount,
		"activeCell":   map[string]int{"row": 0, "col": 0},
		"selection":    map[string]any{"start": map[string]int{"row": 0, "col": 0}, "end": map[string]int{"row": 0, "col": 0}},
		"columnWidths": map[string]any{},
		"rowHeights":   map[string]any{},
		"mergedCells":  []any{},
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func sheetMasterSaveFile(ctx context.Context, baseURL, apiKey string, fileID uint, name string, state json.RawMessage) (sheetMasterFileMeta, error) {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(apiKey) == "" {
		return sheetMasterFileMeta{}, errSheetMasterNotConfigured
	}
	payload := sheetMasterSaveFileInput{
		Name:  strings.TrimSpace(name),
		State: state,
	}
	if fileID != 0 {
		payload.ID = &fileID
	}
	if payload.Name == "" {
		payload.Name = sheetMasterAboutUserName
	}
	client := sheetMasterHTTPClient()
	u := baseURL + "/api/v1/files"
	var out sheetMasterSaveFileResponse
	if err := sheetMasterDoJSONWithBody(ctx, client, "POST", u, apiKey, payload, &out); err != nil {
		return sheetMasterFileMeta{}, err
	}
	return sheetMasterFileMeta{
		ID:         out.ID,
		Name:       out.Name,
		AccessRole: out.AccessRole,
	}, nil
}

func sheetMasterGetFileMeta(ctx context.Context, baseURL, apiKey string, fileID uint) (sheetMasterFileMeta, error) {
	if fileID == 0 {
		return sheetMasterFileMeta{}, fmt.Errorf("file_id is required")
	}
	client := sheetMasterHTTPClient()
	u := fmt.Sprintf("%s/api/v1/files/%d", baseURL, fileID)
	var out sheetMasterFileDetailResponse
	if err := sheetMasterDoJSON(ctx, client, "GET", u, apiKey, &out); err != nil {
		return sheetMasterFileMeta{}, err
	}
	return sheetMasterFileMeta{
		ID:         out.ID,
		Name:       out.Name,
		UpdatedAt:  out.UpdatedAt,
		AccessRole: out.AccessRole,
	}, nil
}

func sheetMasterListFilesWithLimit(ctx context.Context, cfg sheetMasterConfig, limit int) ([]sheetMasterFileMeta, error) {
	if limit <= 0 {
		limit = 20
	}
	client := sheetMasterHTTPClient()
	var out sheetMasterFilesResponse
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	u := cfg.BaseURL + "/api/v1/files?" + q.Encode()
	if err := sheetMasterDoJSON(ctx, client, "GET", u, cfg.APIKey, &out); err != nil {
		return nil, err
	}
	return out.Files, nil
}

func sheetMasterGetDBFileMeta(ctx context.Context, dsn string, fileID uint) (sheetMasterFileMeta, error) {
	if fileID == 0 {
		return sheetMasterFileMeta{}, fmt.Errorf("file_id is required")
	}
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return sheetMasterFileMeta{}, err
	}
	defer db.Close()

	var meta sheetMasterFileMeta
	row := db.QueryRowContext(ctx, `
SELECT id, name, updated_at
FROM sheet_files
WHERE id = $1
LIMIT 1`, fileID)
	if err := row.Scan(&meta.ID, &meta.Name, &meta.UpdatedAt); err != nil {
		return sheetMasterFileMeta{}, err
	}
	return meta, nil
}

func sheetMasterFindAboutUserFileInDB(ctx context.Context, dsn string) (sheetMasterFileMeta, bool, error) {
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return sheetMasterFileMeta{}, false, err
	}
	defer db.Close()

	var meta sheetMasterFileMeta
	row := db.QueryRowContext(ctx, `
SELECT id, name, updated_at
FROM sheet_files
WHERE lower(name) = lower($1)
ORDER BY updated_at DESC, id DESC
LIMIT 1`, sheetMasterAboutUserName)
	if err := row.Scan(&meta.ID, &meta.Name, &meta.UpdatedAt); err != nil {
		if errors.Is(err, errSheetMasterDBNotConfigured) {
			return sheetMasterFileMeta{}, false, err
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return sheetMasterFileMeta{}, false, err
		}
		if errors.Is(err, sql.ErrNoRows) {
			return sheetMasterFileMeta{}, false, nil
		}
		return sheetMasterFileMeta{}, false, err
	}
	if meta.ID == 0 {
		return sheetMasterFileMeta{}, false, nil
	}
	return meta, true, nil
}

func sheetMasterResolveOwnerID(ctx context.Context, dsn string) (uint, error) {
	selectedID, _ := sheetMasterSelectedFileID()
	if selectedID != 0 {
		if ownerID, err := sheetMasterGetFileOwnerID(ctx, dsn, selectedID); err == nil && ownerID != 0 {
			return ownerID, nil
		}
	}
	if ownerID, err := sheetMasterGetLatestOwnerID(ctx, dsn); err == nil && ownerID != 0 {
		return ownerID, nil
	}
	if ownerID, err := sheetMasterGetAnyUserID(ctx, dsn); err == nil && ownerID != 0 {
		return ownerID, nil
	}
	return 0, fmt.Errorf("owner user_id not found")
}

func sheetMasterGetFileOwnerID(ctx context.Context, dsn string, fileID uint) (uint, error) {
	if fileID == 0 {
		return 0, fmt.Errorf("file_id is required")
	}
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var ownerID uint
	row := db.QueryRowContext(ctx, `
SELECT user_id
FROM sheet_files
WHERE id = $1
LIMIT 1`, fileID)
	if err := row.Scan(&ownerID); err != nil {
		return 0, err
	}
	return ownerID, nil
}

func sheetMasterGetLatestOwnerID(ctx context.Context, dsn string) (uint, error) {
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var ownerID uint
	row := db.QueryRowContext(ctx, `
SELECT user_id
FROM sheet_files
ORDER BY updated_at DESC, id DESC
LIMIT 1`)
	if err := row.Scan(&ownerID); err != nil {
		return 0, err
	}
	return ownerID, nil
}

func sheetMasterGetAnyUserID(ctx context.Context, dsn string) (uint, error) {
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var ownerID uint
	row := db.QueryRowContext(ctx, `
SELECT id
FROM users
ORDER BY id ASC
LIMIT 1`)
	if err := row.Scan(&ownerID); err != nil {
		return 0, err
	}
	return ownerID, nil
}

func sheetMasterCreateFileInDB(ctx context.Context, dsn string, userID uint, name string, state json.RawMessage) (sheetMasterFileMeta, error) {
	if userID == 0 {
		return sheetMasterFileMeta{}, fmt.Errorf("user_id is required")
	}
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return sheetMasterFileMeta{}, err
	}
	defer db.Close()

	var meta sheetMasterFileMeta
	row := db.QueryRowContext(ctx, `
INSERT INTO sheet_files (user_id, name, state, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())
RETURNING id, name, updated_at
`, userID, name, state)
	if err := row.Scan(&meta.ID, &meta.Name, &meta.UpdatedAt); err != nil {
		return sheetMasterFileMeta{}, err
	}
	return meta, nil
}

func sheetMasterUpdateFileStateInDB(ctx context.Context, fileID uint, state json.RawMessage) error {
	if fileID == 0 {
		return fmt.Errorf("file_id is required")
	}
	dsn, err := sheetMasterDBDSNFromEnv()
	if err != nil {
		return err
	}
	db, err := sheetMasterOpenDB(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `
UPDATE sheet_files
SET state = $1, updated_at = NOW()
WHERE id = $2`, state, fileID)
	return err
}

func normalizeSheetName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isSheetMasterNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "status=404") || strings.Contains(msg, "not found")
}

func loadSheetMasterAboutUserSelection() (sheetMasterAboutUserSelection, bool, error) {
	b, err := os.ReadFile(sheetMasterAboutUserSelectionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return sheetMasterAboutUserSelection{}, false, nil
		}
		return sheetMasterAboutUserSelection{}, false, err
	}
	var sel sheetMasterAboutUserSelection
	if err := json.Unmarshal(b, &sel); err != nil {
		return sheetMasterAboutUserSelection{}, false, err
	}
	if sel.FileID == 0 {
		return sel, false, nil
	}
	return sel, true, nil
}

func saveSheetMasterAboutUserSelection(sel sheetMasterAboutUserSelection) error {
	if sel.FileID == 0 {
		return fmt.Errorf("file_id is required")
	}
	dir := filepath.Dir(sheetMasterAboutUserSelectionFile)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(sel, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sheetMasterAboutUserSelectionFile, b, 0o600)
}

func clearSheetMasterAboutUserSelection() error {
	if err := os.Remove(sheetMasterAboutUserSelectionFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
