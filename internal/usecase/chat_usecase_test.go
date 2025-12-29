package usecase

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

type stubAIRepo struct {
	resp        string
	called      bool
	lastMessage string
}

func (s *stubAIRepo) GenerateResponse(ctx context.Context, message entity.Message, history []entity.Message) (string, error) {
	return s.resp, nil
}

func (s *stubAIRepo) GenerateResponseWithHistory(ctx context.Context, userID int64, message string, history []entity.Message) (string, error) {
	s.called = true
	s.lastMessage = message
	return s.resp, nil
}

func (s *stubAIRepo) GenerateConfigResponse(ctx context.Context, userID int64, message string, history []entity.Message) (string, error) {
	return s.resp, nil
}

func (s *stubAIRepo) GetRawClient() *genai.Client {
	return nil
}

type stubChatRepo struct {
	history []entity.Message
	saved   []entity.Message
}

func (s *stubChatRepo) SaveMessage(ctx context.Context, message entity.Message) error {
	s.saved = append(s.saved, message)
	return nil
}

func (s *stubChatRepo) GetHistory(ctx context.Context, userID int64, limit int) ([]entity.Message, error) {
	if limit <= 0 || len(s.history) <= limit {
		return s.history, nil
	}
	return s.history[len(s.history)-limit:], nil
}

func (s *stubChatRepo) ClearHistory(ctx context.Context, userID int64) error {
	s.history = nil
	return nil
}

func (s *stubChatRepo) ClearAll(ctx context.Context) error {
	s.history = nil
	s.saved = nil
	return nil
}

func (s *stubChatRepo) GetContext(ctx context.Context, userID int64) (*entity.ChatContext, error) {
	return nil, nil
}

type stubProductRepo struct {
	csvData     string
	csvFilename string
}

func (s *stubProductRepo) SaveProduct(ctx context.Context, product entity.Product) error { return nil }
func (s *stubProductRepo) SaveMany(ctx context.Context, products []entity.Product) error { return nil }
func (s *stubProductRepo) GetByID(ctx context.Context, id string) (*entity.Product, error) {
	return nil, fmt.Errorf("not found")
}
func (s *stubProductRepo) Search(ctx context.Context, query string) ([]entity.Product, error) {
	return nil, nil
}
func (s *stubProductRepo) GetByCategory(ctx context.Context, category string) ([]entity.Product, error) {
	return nil, nil
}
func (s *stubProductRepo) GetAll(ctx context.Context) ([]entity.Product, error) { return nil, nil }
func (s *stubProductRepo) UpdateCatalog(ctx context.Context, catalog entity.ProductCatalog) error {
	return nil
}
func (s *stubProductRepo) GetCatalog(ctx context.Context) (*entity.ProductCatalog, error) {
	return nil, nil
}
func (s *stubProductRepo) Clear(ctx context.Context) error {
	s.csvData = ""
	s.csvFilename = ""
	return nil
}
func (s *stubProductRepo) SaveCSV(ctx context.Context, csvData string, filename string) error {
	s.csvData = csvData
	s.csvFilename = filename
	return nil
}
func (s *stubProductRepo) GetCSV(ctx context.Context) (string, string, error) {
	if strings.TrimSpace(s.csvData) == "" {
		return "", "", fmt.Errorf("CSV data not found")
	}
	return s.csvData, s.csvFilename, nil
}

func TestProcessMessage_AsksCategoryWhenMissing(t *testing.T) {
	ai := &stubAIRepo{resp: "AI response"}
	chat := &stubChatRepo{}
	prod := &stubProductRepo{
		csvFilename: "test.csv",
		csvData:     "Monitor\nA,10.00\nCPU\nX,100.00\n",
	}

	u := NewChatUseCase(ai, chat, prod)
	resp, err := u.ProcessMessage(context.Background(), 1, "u", "tavsiya kerak")
	if err != nil {
		t.Fatalf("ProcessMessage returned error: %v", err)
	}
	if !ai.called {
		t.Fatalf("AI should be called when category is missing")
	}
	if resp != "AI response" {
		t.Fatalf("unexpected response: %q", resp)
	}
}

func TestProcessMessage_NoBudget_DoesNotBlockAndProvides5Options(t *testing.T) {
	sampleCSV := strings.Join([]string{
		"Monitor",
		"A,10.00",
		"B,20.00",
		"C,30.00",
		"D,40.00",
		"E,50.00",
		"CPU",
		"X,100.00",
	}, "\n")

	ai := &stubAIRepo{resp: "Budjet qancha?"} // triggers no-budget fallback
	chat := &stubChatRepo{}
	prod := &stubProductRepo{
		csvFilename: "test.csv",
		csvData:     sampleCSV,
	}

	u := NewChatUseCase(ai, chat, prod)
	resp, err := u.ProcessMessage(context.Background(), 1, "u", "monitor tavsiya")
	if err != nil {
		t.Fatalf("ProcessMessage returned error: %v", err)
	}
	if !ai.called {
		t.Fatalf("AI should be called when category is present")
	}
	if strings.Contains(resp, "Variantlarni aniq topish uchun") {
		t.Fatalf("should not block asking for 3 required fields, got: %q", resp)
	}
	if got := countPricedVariants(resp); got != 5 {
		t.Fatalf("expected 5 priced variants, got %d. resp=%q", got, resp)
	}
	if strings.Contains(ai.lastMessage, "\nCPU\n") {
		t.Fatalf("expected category-filtered CSV in AI prompt, got message containing CPU section")
	}
}

func TestProcessMessage_ModelToken_DoesNotAskCategory(t *testing.T) {
	sampleCSV := strings.Join([]string{
		"CPU",
		"INTEL CORE I5 13400F T,154.00",
		"I5 10400F,100.00",
		"GPU",
		"RTX 4060,300.00",
	}, "\n")

	ai := &stubAIRepo{resp: "SHOULD_NOT_BE_USED"}
	chat := &stubChatRepo{}
	prod := &stubProductRepo{
		csvFilename: "test.csv",
		csvData:     sampleCSV,
	}

	u := NewChatUseCase(ai, chat, prod)
	resp, err := u.ProcessMessage(context.Background(), 1, "u", "13400f kerak")
	if err != nil {
		t.Fatalf("ProcessMessage returned error: %v", err)
	}
	if !ai.called {
		t.Fatalf("AI should be called when fast-match is disabled")
	}
	if resp != "SHOULD_NOT_BE_USED" {
		t.Fatalf("unexpected response: %q", resp)
	}
}
