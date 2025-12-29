package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"converter-backend/internal/logger"
	"converter-backend/internal/services"
)

type realtimeInternalCellEdit struct {
	Row   int    `json:"row"`
	Col   int    `json:"col"`
	Value string `json:"value"`
}

type realtimeInternalBatchEditRequest struct {
	Edits []realtimeInternalCellEdit `json:"edits"`
}

func notifyRealtimeBatchEdits(sheetID uint, edits []services.CellEdit) {
	baseURL := strings.TrimRight(os.Getenv("REALTIME_INTERNAL_URL"), "/")
	if baseURL == "" {
		return
	}

	secret := os.Getenv("REALTIME_INTERNAL_SECRET")
	if secret == "" {
		secret = os.Getenv("INTERNAL_API_SECRET")
	}
	if secret == "" {
		return
	}

	payload := realtimeInternalBatchEditRequest{Edits: make([]realtimeInternalCellEdit, 0, len(edits))}
	for _, edit := range edits {
		if edit.Row < 0 || edit.Col < 0 {
			continue
		}
		payload.Edits = append(payload.Edits, realtimeInternalCellEdit{Row: edit.Row, Col: edit.Col, Value: edit.Value})
	}

	if len(payload.Edits) == 0 {
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logger.Warn("realtime bridge: failed to marshal edits:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/spreadsheets/%d/batch_edit", baseURL, sheetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		logger.Warn("realtime bridge: failed to create request:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", secret)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("realtime bridge: request failed:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("realtime bridge: non-2xx response:", resp.Status)
	}
}
