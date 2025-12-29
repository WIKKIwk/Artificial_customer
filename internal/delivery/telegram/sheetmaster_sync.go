package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type sheetMasterConfig struct {
	BaseURL string
	APIKey  string
	FileID  string
}

func sheetMasterConfigFromEnv() (sheetMasterConfig, error) {
	baseURL := strings.TrimSpace(os.Getenv("SHEETMASTER_API_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("SHEETMASTER_API_KEY"))
	fileID := strings.TrimSpace(os.Getenv("SHEETMASTER_CATALOG_FILE_ID"))

	if baseURL == "" {
		return sheetMasterConfig{}, fmt.Errorf("SHEETMASTER_API_BASE_URL yo'q (misol: http://backend-go:8080 yoki http://localhost:8080)")
	}
	if apiKey == "" {
		return sheetMasterConfig{}, fmt.Errorf("SHEETMASTER_API_KEY yo'q (SheetMaster'dan API key yarating)")
	}
	if fileID == "" {
		return sheetMasterConfig{}, fmt.Errorf("SHEETMASTER_CATALOG_FILE_ID yo'q (SheetMaster fayl ID sini kiriting)")
	}

	return sheetMasterConfig{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		FileID:  fileID,
	}, nil
}

type sheetMasterHealthResponse struct {
	Status string `json:"status"`
}

type sheetMasterUsedRange struct {
	A1 string `json:"a1"`
}

type sheetMasterSchemaResponse struct {
	FileID     uint                  `json:"file_id"`
	Name       string                `json:"name"`
	UsedRange  *sheetMasterUsedRange `json:"used_range"`
	AccessRole string                `json:"access_role"`
}

type sheetMasterFileStateResponse struct {
	ID        uint            `json:"id"`
	Name      string          `json:"name"`
	State     json.RawMessage `json:"state"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type sheetMasterCellsPoint struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type sheetMasterCellsResponse struct {
	FileID     uint                  `json:"file_id"`
	Range      string                `json:"range"`
	Start      sheetMasterCellsPoint `json:"start"`
	End        sheetMasterCellsPoint `json:"end"`
	Format     string                `json:"format"`
	ValueMode  string                `json:"value_mode"`
	Values     [][]string            `json:"values"`
	AccessRole string                `json:"access_role"`
}

func sheetMasterHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func sheetMasterDoJSON(ctx context.Context, client *http.Client, method, urlStr, apiKey string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("status=%d: %s", resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 25<<20))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("json decode error: %w", err)
	}
	return nil
}

func sheetMasterDoJSONWithBody(ctx context.Context, client *http.Client, method, urlStr, apiKey string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("status=%d: %s", resp.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 25<<20))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("json decode error: %w", err)
	}
	return nil
}

func sheetMasterHealth(ctx context.Context, cfg sheetMasterConfig) (string, error) {
	client := sheetMasterHTTPClient()
	var out sheetMasterHealthResponse
	u := cfg.BaseURL + "/health"
	if err := sheetMasterDoJSON(ctx, client, http.MethodGet, u, "", &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Status) == "" {
		return "unknown", nil
	}
	return out.Status, nil
}

func sheetMasterGetSchema(ctx context.Context, cfg sheetMasterConfig) (sheetMasterSchemaResponse, error) {
	client := sheetMasterHTTPClient()
	var out sheetMasterSchemaResponse
	u := fmt.Sprintf("%s/api/v1/files/%s/schema", cfg.BaseURL, url.PathEscape(cfg.FileID))
	if err := sheetMasterDoJSON(ctx, client, http.MethodGet, u, cfg.APIKey, &out); err != nil {
		return sheetMasterSchemaResponse{}, err
	}
	return out, nil
}

func sheetMasterGetFileState(ctx context.Context, cfg sheetMasterConfig) (sheetMasterDBFile, error) {
	client := sheetMasterHTTPClient()
	var out sheetMasterFileStateResponse
	u := fmt.Sprintf("%s/api/v1/files/%s", cfg.BaseURL, url.PathEscape(cfg.FileID))
	if err := sheetMasterDoJSON(ctx, client, http.MethodGet, u, cfg.APIKey, &out); err != nil {
		legacy := fmt.Sprintf("%s/api/files/%s", cfg.BaseURL, url.PathEscape(cfg.FileID))
		if legacyErr := sheetMasterDoJSON(ctx, client, http.MethodGet, legacy, cfg.APIKey, &out); legacyErr != nil {
			return sheetMasterDBFile{}, err
		}
	}
	if out.ID == 0 {
		return sheetMasterDBFile{}, fmt.Errorf("file not found")
	}
	return sheetMasterDBFile{
		ID:        out.ID,
		Name:      out.Name,
		StateJSON: out.State,
		UpdatedAt: out.UpdatedAt,
	}, nil
}

func sheetMasterGetCells(ctx context.Context, cfg sheetMasterConfig, rangeA1 string) (sheetMasterCellsResponse, error) {
	client := sheetMasterHTTPClient()
	var out sheetMasterCellsResponse

	q := url.Values{}
	q.Set("range", strings.TrimSpace(rangeA1))
	q.Set("format", "grid")
	q.Set("value", "raw")
	u := fmt.Sprintf("%s/api/v1/files/%s/cells?%s", cfg.BaseURL, url.PathEscape(cfg.FileID), q.Encode())

	if err := sheetMasterDoJSON(ctx, client, http.MethodGet, u, cfg.APIKey, &out); err != nil {
		return sheetMasterCellsResponse{}, err
	}
	return out, nil
}

func sheetMasterCellsToXLSX(resp sheetMasterCellsResponse) ([]byte, error) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	for r, row := range resp.Values {
		for c, v := range row {
			if strings.TrimSpace(v) == "" {
				continue
			}
			cell, err := excelize.CoordinatesToCellName(resp.Start.Col+c+1, resp.Start.Row+r+1)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type sheetMasterPatchCellsInput struct {
	Edits []sheetMasterPatchCellEdit `json:"edits"`
}

type sheetMasterPatchCellEdit struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Value string `json:"value"`
}

type sheetMasterPatchCellsResponse struct {
	Updated int `json:"updated"`
}

func sheetMasterPatchCells(ctx context.Context, cfg sheetMasterConfig, edits []sheetMasterCellEdit) (int, error) {
	if len(edits) == 0 {
		return 0, nil
	}
	if strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.FileID) == "" {
		return 0, fmt.Errorf("sheetmaster api not configured")
	}

	payload := sheetMasterPatchCellsInput{
		Edits: make([]sheetMasterPatchCellEdit, 0, len(edits)),
	}
	for _, edit := range edits {
		if edit.Row < 0 || edit.Col < 0 {
			continue
		}
		payload.Edits = append(payload.Edits, sheetMasterPatchCellEdit{
			Row:   edit.Row,
			Col:   edit.Col,
			Value: edit.Value,
		})
	}
	if len(payload.Edits) == 0 {
		return 0, nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	client := sheetMasterHTTPClient()
	u := fmt.Sprintf("%s/api/v1/files/%s/cells", cfg.BaseURL, url.PathEscape(cfg.FileID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, u, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(cfg.APIKey) != "" {
		req.Header.Set("X-API-Key", cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return 0, fmt.Errorf("status=%d: %s", resp.StatusCode, msg)
	}

	var out sheetMasterPatchCellsResponse
	dec := json.NewDecoder(io.LimitReader(resp.Body, 5<<20))
	if err := dec.Decode(&out); err != nil {
		return 0, fmt.Errorf("json decode error: %w", err)
	}
	return out.Updated, nil
}
