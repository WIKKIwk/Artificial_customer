package storage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

type memoryProductRepository struct {
	mu       sync.RWMutex
	products map[string]entity.Product // key: product ID
	catalog  *entity.ProductCatalog
}

// NewMemoryProductRepository in-memory product repository yaratish
func NewMemoryProductRepository() repository.ProductRepository {
	return &memoryProductRepository{
		products: make(map[string]entity.Product),
		catalog:  nil,
	}
}

// SaveProduct mahsulotni saqlash
func (m *memoryProductRepository) SaveProduct(ctx context.Context, product entity.Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.products[product.ID] = product
	return nil
}

// SaveMany ko'p mahsulotlarni saqlash
func (m *memoryProductRepository) SaveMany(ctx context.Context, products []entity.Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, product := range products {
		m.products[product.ID] = product
	}
	return nil
}

// GetByID ID bo'yicha mahsulotni olish
func (m *memoryProductRepository) GetByID(ctx context.Context, id string) (*entity.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	product, exists := m.products[id]
	if !exists {
		return nil, fmt.Errorf("product not found: %s", id)
	}
	return &product, nil
}

// Search mahsulot qidirish
func (m *memoryProductRepository) Search(ctx context.Context, query string) ([]entity.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, nil
	}
	queryNorm := normalizeProductString(query)
	tokens := filterTokens(queryTokens(query))
	tokensNorm := normalizeTokens(tokens)
	compactQuery := normalizeAlphaNum(query)
	queryDigits := extractDigits(query)
	gpuQuery := isGPUQuery(query)
	var results []entity.Product
	var scored []scoredProduct

	for _, product := range m.products {
		nameLower := strings.ToLower(product.Name)
		catLower := strings.ToLower(product.Category)
		descLower := strings.ToLower(product.Description)
		nameNorm := normalizeProductString(product.Name)
		nameCompact := normalizeAlphaNum(product.Name)
		catNorm := normalizeAlphaNum(product.Category)
		descNorm := normalizeAlphaNum(product.Description)

		// Name, category, description da qidirish
		if strings.Contains(nameLower, query) ||
			strings.Contains(catLower, query) ||
			strings.Contains(descLower, query) ||
			(queryNorm != "" && strings.Contains(nameNorm, queryNorm)) ||
			(compactQuery != "" && strings.Contains(nameCompact, compactQuery)) ||
			matchTokens(tokens, nameLower, catLower, descLower, nameCompact, nameNorm) ||
			matchTokens(tokensNorm, nameNorm, nameCompact, catNorm, descNorm) {
			results = append(results, product)
			continue
		}

		// Raqamga qat'iy moslik
		if queryDigits != "" {
			nameDigits := extractDigits(nameNorm)
			if nameDigits != "" && strings.Contains(nameDigits, queryDigits) {
				results = append(results, product)
				continue
			}
		}

		// Specs da qidirish
		for _, value := range product.Specs {
			if strings.Contains(strings.ToLower(value), query) {
				results = append(results, product)
				break
			}
		}

		// Ballar berib o'xshashlikni aniqlaymiz (faqat minimal ball bo'lsa qo'shamiz)
		score := similarityScore(tokensNorm, compactQuery, nameNorm, nameCompact, catNorm, descNorm)
		if score >= 5 { // Minimal ball talabi - faqat chindan ham o'xshashlarini olish
			scored = append(scored, scoredProduct{Product: product, Score: score})
		}
	}

	// Agar to'g'ridan-to'g'ri topilmasa, yaxshi ball olganlarni qaytaramiz
	if len(results) == 0 && len(scored) > 0 {
		sort.Slice(scored, func(i, j int) bool {
			if scored[i].Score == scored[j].Score {
				return scored[i].Product.Price < scored[j].Product.Price
			}
			return scored[i].Score > scored[j].Score
		})
		// Faqat eng yaxshi 6 ta va ball >= 8 bo'lganlarni qo'shamiz
		for _, sp := range scored {
			if sp.Score >= 8 && len(results) < 6 {
				results = append(results, sp.Product)
			}
		}
	}

	// GPU so'rovi bo'lsa, natijani GPU ga filtrlaymiz
	if gpuQuery && len(results) > 0 {
		filtered := make([]entity.Product, 0, len(results))
		for _, p := range results {
			if isGPUProduct(p) {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) > 0 {
			results = filtered
		}
	}

	// MUHIM: Agar hech narsa topilmasa, bo'sh massiv qaytaramiz
	// Tasodifiy mahsulotlar ko'rsatmaydi!
	return results, nil
}

// GetByCategory kategoriya bo'yicha mahsulotlarni olish
func (m *memoryProductRepository) GetByCategory(ctx context.Context, category string) ([]entity.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	category = strings.ToLower(strings.TrimSpace(category))
	var results []entity.Product

	for _, product := range m.products {
		if strings.ToLower(product.Category) == category {
			results = append(results, product)
		}
	}

	return results, nil
}

// GetAll barcha mahsulotlarni olish
func (m *memoryProductRepository) GetAll(ctx context.Context) ([]entity.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	products := make([]entity.Product, 0, len(m.products))
	for _, product := range m.products {
		products = append(products, product)
	}

	return products, nil
}

// UpdateCatalog butun katalogni yangilash
func (m *memoryProductRepository) UpdateCatalog(ctx context.Context, catalog entity.ProductCatalog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Eski mahsulotlarni o'chirish
	m.products = make(map[string]entity.Product)

	// Yangi mahsulotlarni qo'shish
	for _, product := range catalog.Products {
		m.products[product.ID] = product
	}

	m.catalog = &catalog
	return nil
}

// GetCatalog katalogni olish
func (m *memoryProductRepository) GetCatalog(ctx context.Context) (*entity.ProductCatalog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.catalog == nil {
		return nil, fmt.Errorf("catalog not found")
	}

	return m.catalog, nil
}

// Clear barcha mahsulotlarni o'chirish
func (m *memoryProductRepository) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.products = make(map[string]entity.Product)
	m.catalog = nil
	return nil
}

// Qidiruv yordamchi funksiyalar
func normalizeProductString(s string) string {
	s = strings.ToLower(s)
	replacements := []string{" ", "-", "_", ".", ",", "'", "\"", "/", "\\", "?", "!"}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r, "")
	}
	return s
}

func queryTokens(q string) []string {
	q = strings.ToLower(q)
	separators := []string{",", ".", "?", "!", ";", ":", "/", "\\", "-", "_"}
	for _, sep := range separators {
		q = strings.ReplaceAll(q, sep, " ")
	}
	fields := strings.Fields(q)

	var tokens []string
	for _, f := range fields {
		if len(f) >= 2 {
			tokens = append(tokens, f)
		}
	}
	return tokens
}

func matchTokens(tokens []string, parts ...string) bool {
	if len(tokens) == 0 {
		return false
	}
	for _, t := range tokens {
		for _, p := range parts {
			if strings.Contains(p, t) {
				return true
			}
		}
	}
	return false
}

func normalizeAlphaNum(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

func filterTokens(tokens []string) []string {
	stop := map[string]struct{}{
		"bormi": {}, "bor": {}, "bormidi": {}, "bormi?": {}, "bormidi?": {},
		"kerak": {}, "kerakmi": {}, "kerak?": {}, "qachon": {}, "qanday": {},
	}
	var out []string
	for _, t := range tokens {
		if _, skip := stop[t]; skip {
			continue
		}
		out = append(out, t)
	}
	return out
}

func normalizeTokens(tokens []string) []string {
	var out []string
	for _, t := range tokens {
		n := normalizeAlphaNum(t)
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

type scoredProduct struct {
	Product entity.Product
	Score   int
}

func similarityScore(qTokens []string, compactQuery, nameNorm, nameCompact, catNorm, descNorm string) int {
	score := 0

	// Tokenlarga asoslangan
	for _, qt := range qTokens {
		if qt == "" {
			continue
		}
		if strings.Contains(nameNorm, qt) || strings.Contains(nameCompact, qt) {
			score += 4
			continue
		}
		if strings.Contains(catNorm, qt) || strings.Contains(descNorm, qt) {
			score += 2
			continue
		}

		// Raqamlar yaqinligi (masalan 5090 -> 4090)
		qNum := extractNumber(qt)
		pNum := extractNumber(nameNorm)
		if qNum > 0 && pNum > 0 {
			diff := qNum - pNum
			if diff < 0 {
				diff = -diff
			}
			if diff <= 200 { // GPU modellari yaqin
				score += 2
			}
		}
	}

	// Umumiy harf-raqam chiziqli o'xshashlik
	if compactQuery != "" {
		lcs := longestCommonSubstringLength(compactQuery, nameCompact)
		if lcs >= 3 {
			score += lcs
		}
	}

	return score
}

func extractNumber(s string) int {
	numStr := ""
	for _, r := range s {
		if r >= '0' && r <= '9' {
			numStr += string(r)
		}
	}
	if numStr == "" {
		return 0
	}
	val, _ := strconv.Atoi(numStr)
	return val
}

func extractDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isGPUQuery(q string) bool {
	lower := strings.ToLower(q)
	digits := extractDigits(lower)
	return strings.Contains(lower, "rtx") ||
		strings.Contains(lower, "gtx") ||
		strings.Contains(lower, "rx ") ||
		strings.Contains(lower, "rx-") ||
		strings.Contains(lower, "rx") && digits != "" ||
		len(digits) >= 3 ||
		strings.Contains(lower, "gpu") ||
		strings.Contains(lower, "video") ||
		strings.Contains(lower, "videokarta") ||
		strings.Contains(lower, "karta")
}

func isGPUProduct(p entity.Product) bool {
	name := strings.ToLower(p.Name)
	cat := strings.ToLower(p.Category)
	if strings.Contains(cat, "gpu") || strings.Contains(cat, "video") || strings.Contains(cat, "karta") || strings.Contains(cat, "videokarta") {
		return true
	}
	if strings.Contains(name, "rtx") || strings.Contains(name, "gtx") || strings.Contains(name, "rx ") || strings.Contains(name, "rx-") || strings.Contains(name, "gpu") || strings.Contains(name, "video") {
		return true
	}
	digits := extractDigits(name)
	return len(digits) >= 3 && (strings.Contains(name, "nvidia") || strings.Contains(name, "amd"))
}

func longestCommonSubstringLength(a, b string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	maxLen := 0
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
				if dp[i][j] > maxLen {
					maxLen = dp[i][j]
				}
			}
		}
	}
	return maxLen
}
