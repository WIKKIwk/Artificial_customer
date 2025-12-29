package handlers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"converter-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AIHandler struct {
	apiKey       string
	cache        *AICache
	geminiClient *http.Client
	DB           *gorm.DB
}

type AICache struct {
	mu    sync.RWMutex
	items map[string]*CacheItem
}

type CacheItem struct {
	Response  string
	ExpiresAt time.Time
}

type AIRequest struct {
	Prompt      string  `json:"prompt" binding:"required"`
	SheetData   string  `json:"sheet_data"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
}

type AIResponse struct {
	Text      string `json:"text"`
	Cached    bool   `json:"cached"`
	Timestamp int64  `json:"timestamp"`
}

type GeminiRequest struct {
	Contents []GeminiContent `json:"contents"`
}

type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text string `json:"text"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func NewAIHandler() *AIHandler {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("VITE_GEMINI_API_KEY")
	}

	return &AIHandler{
		apiKey: apiKey,
		cache:  NewAICache(),
		geminiClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func NewAIHandlerWithDB(db *gorm.DB) *AIHandler {
	h := NewAIHandler()
	h.DB = db
	return h
}

// getEffectiveGeminiKey returns user-specific Gemini API key if set, otherwise falls back to server-level key.
// Returns empty string if none configured.
func (h *AIHandler) getEffectiveGeminiKey(c *gin.Context) string {
	userKey := ""
	if h.DB != nil {
		if userID, ok := c.Get("user_id"); ok {
			var user models.User
			if err := h.DB.Select("gemini_api_key").First(&user, userID.(uint)).Error; err == nil && user.GeminiAPIKey != nil {
				userKey = strings.TrimSpace(*user.GeminiAPIKey)
			}
		}
	}

	if userKey != "" {
		return userKey
	}
	return h.apiKey
}

type GeminiKeyRequest struct {
	GeminiAPIKey *string `json:"gemini_api_key"`
}

// SetGeminiAPIKey saves or clears the user's Gemini API key.
func (h *AIHandler) SetGeminiAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req GeminiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var toStore *string
	if req.GeminiAPIKey != nil {
		key := strings.TrimSpace(*req.GeminiAPIKey)
		if key != "" {
			toStore = &key
		}
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", userID.(uint)).Update("gemini_api_key", toStore).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Gemini API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"gemini_api_key": toStore})
}

// GetGeminiAPIKey returns the stored Gemini API key for the authenticated user (if any).
func (h *AIHandler) GetGeminiAPIKey(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var user models.User
	if err := h.DB.Select("gemini_api_key").First(&user, userID.(uint)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load Gemini API key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"gemini_api_key": user.GeminiAPIKey})
}

func NewAICache() *AICache {
	cache := &AICache{
		items: make(map[string]*CacheItem),
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

func (c *AICache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.ExpiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

func (c *AICache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return "", false
	}

	if time.Now().After(item.ExpiresAt) {
		return "", false
	}

	return item.Response, true
}

func (c *AICache) Set(key string, response string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &CacheItem{
		Response:  response,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (h *AIHandler) getCacheKey(prompt, sheetData string) string {
	combined := prompt + "|" + sheetData
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

func (h *AIHandler) GenerateAI(c *gin.Context) {
	effectiveKey := h.getEffectiveGeminiKey(c)
	if effectiveKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI service not configured. Please add your Gemini API key.",
		})
		return
	}

	var req AIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check cache first
	cacheKey := h.getCacheKey(req.Prompt, req.SheetData)
	if cached, found := h.cache.Get(cacheKey); found {
		c.JSON(http.StatusOK, AIResponse{
			Text:      cached,
			Cached:    true,
			Timestamp: time.Now().Unix(),
		})
		return
	}

	// Call Gemini API
	fullPrompt := req.Prompt
	if req.SheetData != "" {
		fullPrompt = fmt.Sprintf("Context (Current Sheet Data):\n%s\n\nUser Request:\n%s", req.SheetData, req.Prompt)
	}

	geminiReq := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: fullPrompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(geminiReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare request"})
		return
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-exp:generateContent?key=%s", effectiveKey)

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.geminiClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("AI API error: %s", string(body))})
		return
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse AI response"})
		return
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No response from AI"})
		return
	}

	responseText := geminiResp.Candidates[0].Content.Parts[0].Text

	// Cache the response for 15 minutes
	h.cache.Set(cacheKey, responseText, 15*time.Minute)

	c.JSON(http.StatusOK, AIResponse{
		Text:      responseText,
		Cached:    false,
		Timestamp: time.Now().Unix(),
	})
}

func (h *AIHandler) StreamAI(c *gin.Context) {
	effectiveKey := h.getEffectiveGeminiKey(c)
	if effectiveKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "AI service not configured",
		})
		return
	}

	var req AIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check cache first
	cacheKey := h.getCacheKey(req.Prompt, req.SheetData)
	if cached, found := h.cache.Get(cacheKey); found {
		// Return cached result immediately
		c.JSON(http.StatusOK, AIResponse{
			Text:      cached,
			Cached:    true,
			Timestamp: time.Now().Unix(),
		})
		return
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	// Call Gemini API
	fullPrompt := req.Prompt
	if req.SheetData != "" {
		fullPrompt = fmt.Sprintf("Context:\n%s\n\nRequest:\n%s", req.SheetData, req.Prompt)
	}

	geminiReq := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: fullPrompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(geminiReq)
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\": \"Failed to prepare request\"}\n\n")
		flusher.Flush()
		return
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash-exp:streamGenerateContent?key=%s&alt=sse", effectiveKey)

	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\": \"Failed to create request\"}\n\n")
		flusher.Flush()
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.geminiClient.Do(httpReq)
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\": \"AI request failed\"}\n\n")
		flusher.Flush()
		return
	}
	defer resp.Body.Close()

	// Stream the response
	fullResponse := ""
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		// SSE format: "data: {...}"
		if bytes.HasPrefix([]byte(line), []byte("data: ")) {
			dataJSON := line[6:] // Remove "data: " prefix

			var geminiChunk GeminiResponse
			if err := json.Unmarshal([]byte(dataJSON), &geminiChunk); err == nil {
				if len(geminiChunk.Candidates) > 0 && len(geminiChunk.Candidates[0].Content.Parts) > 0 {
					chunk := geminiChunk.Candidates[0].Content.Parts[0].Text
					fullResponse += chunk

					// Forward to client
					fmt.Fprintf(c.Writer, "data: %s\n\n", dataJSON)
					flusher.Flush()
				}
			}
		}
	}

	// Cache the full response
	if fullResponse != "" {
		h.cache.Set(cacheKey, fullResponse, 15*time.Minute)
	}

	// Send completion signal
	fmt.Fprintf(c.Writer, "data: {\"done\": true}\n\n")
	flusher.Flush()
}
