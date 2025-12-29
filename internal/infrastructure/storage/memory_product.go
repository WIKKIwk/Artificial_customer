package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

type memoryProductRepository struct {
	mu          sync.RWMutex
	products    map[string]entity.Product // key: product ID
	catalog     *entity.ProductCatalog
	csvData     string // CSV ma'lumotlari
	csvFilename string // CSV fayl nomi
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

var searchNormalizeRe = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func transliterateToLatin(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range input {
		switch r {
		case '\u0430':
			b.WriteByte('a')
		case '\u0431':
			b.WriteByte('b')
		case '\u0432':
			b.WriteByte('v')
		case '\u0433':
			b.WriteByte('g')
		case '\u0434':
			b.WriteByte('d')
		case '\u0435':
			b.WriteByte('e')
		case '\u0451':
			b.WriteString("yo")
		case '\u0436':
			b.WriteString("zh")
		case '\u0437':
			b.WriteByte('z')
		case '\u0438':
			b.WriteByte('i')
		case '\u0439':
			b.WriteByte('y')
		case '\u043a':
			b.WriteByte('k')
		case '\u043b':
			b.WriteByte('l')
		case '\u043c':
			b.WriteByte('m')
		case '\u043d':
			b.WriteByte('n')
		case '\u043e':
			b.WriteByte('o')
		case '\u043f':
			b.WriteByte('p')
		case '\u0440':
			b.WriteByte('r')
		case '\u0441':
			b.WriteByte('s')
		case '\u0442':
			b.WriteByte('t')
		case '\u0443':
			b.WriteByte('u')
		case '\u0444':
			b.WriteByte('f')
		case '\u0445':
			b.WriteByte('h')
		case '\u0446':
			b.WriteString("ts")
		case '\u0447':
			b.WriteString("ch")
		case '\u0448':
			b.WriteString("sh")
		case '\u0449':
			b.WriteString("shch")
		case '\u044a', '\u044c':
			continue
		case '\u044b':
			b.WriteByte('y')
		case '\u044d':
			b.WriteByte('e')
		case '\u044e':
			b.WriteString("yu")
		case '\u044f':
			b.WriteString("ya")
		case '\u0454':
			b.WriteString("ye")
		case '\u0456':
			b.WriteByte('i')
		case '\u0457':
			b.WriteString("yi")
		case '\u049b':
			b.WriteByte('q')
		case '\u0491':
			b.WriteByte('g')
		case '\u0493':
			b.WriteByte('g')
		case '\u045e':
			b.WriteByte('o')
		case '\u04d9':
			b.WriteByte('a')
		case '\u04e9':
			b.WriteByte('o')
		case '\u04af':
			b.WriteByte('u')
		case '\u04b1':
			b.WriteByte('u')
		case '\u04b3':
			b.WriteByte('h')
		case '\u04bb':
			b.WriteByte('h')
		case '\u04a3':
			b.WriteString("ng")
		case '\u02bc', '\u2019', '\'':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeSearchText(input string) string {
	input = strings.ToLower(input)
	input = transliterateToLatin(input)
	input = searchNormalizeRe.ReplaceAllString(input, " ")
	return strings.TrimSpace(input)
}

func compactSearchText(input string) string {
	input = strings.ToLower(input)
	input = transliterateToLatin(input)
	return searchNormalizeRe.ReplaceAllString(input, "")
}

func buildProductSearchText(product entity.Product) string {
	var b strings.Builder
	b.WriteString(product.Name)
	b.WriteString(" ")
	b.WriteString(product.Category)
	b.WriteString(" ")
	b.WriteString(product.Description)
	for key, value := range product.Specs {
		if key != "" {
			b.WriteString(" ")
			b.WriteString(key)
		}
		if value != "" {
			b.WriteString(" ")
			b.WriteString(value)
		}
	}
	return b.String()
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return true
	default:
		return false
	}
}

func consonantSignature(input string) string {
	var b strings.Builder
	prev := rune(0)
	for i, r := range input {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if unicode.IsLetter(r) && isVowel(r) && i > 0 {
			continue
		}
		if r == prev {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

func ngramSet(input string, n int) map[string]struct{} {
	if n <= 0 {
		return nil
	}
	runes := []rune(input)
	if len(runes) == 0 {
		return nil
	}
	if len(runes) < n {
		return map[string]struct{}{string(runes): {}}
	}
	set := make(map[string]struct{}, len(runes)-n+1)
	for i := 0; i <= len(runes)-n; i++ {
		set[string(runes[i:i+n])] = struct{}{}
	}
	return set
}

func ngramSimilarity(a, b string, n int) float64 {
	setA := ngramSet(a, n)
	setB := ngramSet(b, n)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for gram := range setA {
		if _, ok := setB[gram]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func hasLetter(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func maxEditDistance(token string) int {
	l := len([]rune(token))
	switch {
	case l <= 3:
		return 0
	case l <= 5:
		return 1
	case l <= 8:
		return 2
	case l <= 12:
		return 3
	default:
		return 4
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func min3(a, b, c int) int {
	return minInt(minInt(a, b), c)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func editDistanceWithin(a, b string, max int) (int, bool) {
	if max < 0 {
		return 0, false
	}
	if a == b {
		return 0, true
	}
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 {
		if len(rb) <= max {
			return len(rb), true
		}
		return 0, false
	}
	if len(rb) == 0 {
		if len(ra) <= max {
			return len(ra), true
		}
		return 0, false
	}
	if absInt(len(ra)-len(rb)) > max {
		return 0, false
	}
	if len(rb) > len(ra) {
		ra, rb = rb, ra
	}

	prev := make([]int, len(rb)+1)
	curr := make([]int, len(rb)+1)
	for j := 0; j <= len(rb); j++ {
		prev[j] = j
	}

	for i, raChar := range ra {
		curr[0] = i + 1
		minRow := curr[0]
		for j, rbChar := range rb {
			cost := 0
			if raChar != rbChar {
				cost = 1
			}
			del := prev[j+1] + 1
			ins := curr[j] + 1
			sub := prev[j] + cost
			v := min3(del, ins, sub)
			curr[j+1] = v
			if v < minRow {
				minRow = v
			}
		}
		if minRow > max {
			return 0, false
		}
		prev, curr = curr, prev
	}

	dist := prev[len(rb)]
	if dist <= max {
		return dist, true
	}
	return 0, false
}

// Search mahsulot qidirish
func (m *memoryProductRepository) Search(ctx context.Context, query string) ([]entity.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normalizedQuery := normalizeSearchText(query)
	compactQuery := compactSearchText(query)
	if normalizedQuery == "" && compactQuery == "" {
		return nil, nil
	}
	queryTokens := strings.Fields(normalizedQuery)
	compactQueryLen := len([]rune(compactQuery))
	querySignature := consonantSignature(normalizedQuery)

	type scoredProduct struct {
		product entity.Product
		score   int
	}
	var matches []scoredProduct

	for _, product := range m.products {
		searchText := buildProductSearchText(product)
		textNorm := normalizeSearchText(searchText)
		textCompact := compactSearchText(searchText)
		nameNorm := normalizeSearchText(product.Name)
		nameCompact := compactSearchText(product.Name)
		textTokens := strings.Fields(textNorm)
		textSignature := consonantSignature(textNorm)
		nameSignature := consonantSignature(nameNorm)

		score := 0
		if normalizedQuery != "" {
			if strings.Contains(nameNorm, normalizedQuery) {
				score += 120
			} else if strings.Contains(textNorm, normalizedQuery) {
				score += 100
			}
		}
		if compactQuery != "" {
			if strings.Contains(nameCompact, compactQuery) {
				score += 110
			} else if strings.Contains(textCompact, compactQuery) {
				score += 90
			}
		}
		if compactQuery != "" && compactQueryLen >= 4 {
			sim := ngramSimilarity(compactQuery, nameCompact, 2)
			if sim >= 0.35 {
				score += int(sim * 80)
			} else if sim >= 0.25 {
				score += int(sim * 50)
			}
		}
		if len(querySignature) >= 3 {
			if strings.Contains(nameSignature, querySignature) {
				score += 60
			} else if strings.Contains(textSignature, querySignature) {
				score += 40
			}
		}

		if len(queryTokens) > 0 && len(textTokens) > 0 {
			tokenSet := make(map[string]struct{}, len(textTokens))
			tokenList := make([]string, 0, len(textTokens))
			tokenSignatureSet := make(map[string]struct{}, len(textTokens))
			for _, t := range textTokens {
				if t == "" {
					continue
				}
				if _, ok := tokenSet[t]; ok {
					continue
				}
				tokenSet[t] = struct{}{}
				tokenList = append(tokenList, t)
				if sig := consonantSignature(t); len(sig) >= 3 {
					tokenSignatureSet[sig] = struct{}{}
				}
			}
			for _, qt := range queryTokens {
				if len(qt) < 2 {
					continue
				}
				if _, ok := tokenSet[qt]; ok {
					score += 12
					continue
				}
				matched := false
				if len(qt) >= 2 {
					for _, tt := range tokenList {
						if strings.HasPrefix(tt, qt) {
							score += 8
							matched = true
							break
						}
					}
				}
				if matched {
					continue
				}
				if len(qt) >= 3 {
					for _, tt := range tokenList {
						if strings.Contains(tt, qt) {
							score += 4
							matched = true
							break
						}
					}
				}
				if matched {
					continue
				}
				if !hasLetter(qt) {
					continue
				}
				maxEdits := maxEditDistance(qt)
				if maxEdits == 0 {
					continue
				}
				bestDist := 0
				found := false
				for _, tt := range tokenList {
					dist, ok := editDistanceWithin(qt, tt, maxEdits)
					if !ok {
						continue
					}
					if !found || dist < bestDist {
						found = true
						bestDist = dist
						if bestDist == 0 {
							break
						}
					}
				}
				if found {
					score += 6 + (maxEdits - bestDist)
					continue
				}
				if sig := consonantSignature(qt); len(sig) >= 3 {
					if _, ok := tokenSignatureSet[sig]; ok {
						score += 5
					}
				}
			}
		}

		if score > 0 {
			matches = append(matches, scoredProduct{product: product, score: score})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score == matches[j].score {
			return matches[i].product.Name < matches[j].product.Name
		}
		return matches[i].score > matches[j].score
	})

	results := make([]entity.Product, len(matches))
	for i, match := range matches {
		results[i] = match.product
	}
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
	m.csvData = ""
	m.csvFilename = ""
	return nil
}

// SaveCSV CSV ma'lumotlarini faylga saqlash
func (m *memoryProductRepository) SaveCSV(ctx context.Context, csvData string, filename string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Xotirada ham saqlash
	m.csvData = csvData
	m.csvFilename = filename

	// data/catalogs papkani yaratish
	catalogDir := "data/catalogs"
	if err := os.MkdirAll(catalogDir, 0755); err != nil {
		return fmt.Errorf("failed to create catalog directory: %w", err)
	}

	// CSV fayl nomini yaratish (timestamp bilan)
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	// Original fayl nomidan kengaytmani olib tashlash va .csv qo'shish
	baseFilename := strings.TrimSuffix(filename, filepath.Ext(filename))
	csvFilename := fmt.Sprintf("%s_%s.csv", baseFilename, timestamp)
	csvFilePath := filepath.Join(catalogDir, csvFilename)

	// CSV ni faylga yozish
	if err := os.WriteFile(csvFilePath, []byte(csvData), 0644); err != nil {
		return fmt.Errorf("failed to write CSV file: %w", err)
	}

	return nil
}

// GetCSV saqlangan CSV ma'lumotlarini olish
func (m *memoryProductRepository) GetCSV(ctx context.Context) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.csvData == "" {
		return "", "", fmt.Errorf("CSV data not found")
	}

	return m.csvData, m.csvFilename, nil
}
