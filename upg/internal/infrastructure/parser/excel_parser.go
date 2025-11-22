package parser

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

type excelParser struct{}

// NewExcelParser yangi Excel parser yaratish
func NewExcelParser() repository.ExcelParser {
	return &excelParser{}
}

// ParseProducts Excel fayldan mahsulotlarni o'qish
func (e *excelParser) ParseProducts(ctx context.Context, filePath string) ([]entity.Product, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	return e.parseExcelFile(f)
}

// ParseProductsFromBytes byte array dan parse qilish
func (e *excelParser) ParseProductsFromBytes(ctx context.Context, data []byte, filename string) ([]entity.Product, error) {
	reader := bytes.NewReader(data)
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel from bytes: %w", err)
	}
	defer f.Close()

	return e.parseExcelFile(f)
}

// parseExcelFile Excel faylni parse qilish
func (e *excelParser) parseExcelFile(f *excelize.File) ([]entity.Product, error) {
	// Birinchi sheet ni olish
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("excel file has no sheets")
	}

	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("excel file is empty")
	}

	// Debug: Birinchi qatorni chop etish
	if len(rows) > 0 {
		log.Printf("📋 Excel first row: %v", rows[0])
		log.Printf("📊 Total rows: %d", len(rows))
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("excel file has no data")
	}

	// Header qatori borligini tekshirish
	// Agar birinchi qatorning 2-ustuni raqam bo'lsa, header yo'q
	hasHeader := true
	startRow := 1

	if len(rows) > 0 && len(rows[0]) > 1 {
		// 2-ustunni tekshirish (narx bo'lishi kerak)
		secondCol := strings.TrimSpace(rows[0][1])
		if _, err := strconv.ParseFloat(strings.ReplaceAll(secondCol, ",", ""), 64); err == nil {
			// Raqam! Demak bu header emas, data
			hasHeader = false
			startRow = 0
			log.Printf("🔍 No header detected - data starts from row 0")
		}
	}

	var columnMap map[string]int

	var header []string

	if hasHeader {
		// Header row dan column mapping yaratish
		header = rows[0]
		columnMap = e.mapColumns(header)
		log.Printf("🗺️ Column mapping from header: %v", columnMap)
	} else {
		// Header yo'q - default mapping
		columnMap = map[string]int{
			"name":  0, // 1-ustun: nom
			"price": 1, // 2-ustun: narx
		}
		if len(rows[0]) > 2 {
			columnMap["category"] = 2 // 3-ustun: kategoriya (agar bor bo'lsa)
		}
		log.Printf("🗺️ Default column mapping (no header): %v", columnMap)
	}

	// Kolonkalar indekslari
	nameCol := 0
	if idx, ok := columnMap["name"]; ok {
		nameCol = idx
	}

	priceCol := -1
	if idx, ok := columnMap["price"]; ok {
		priceCol = idx
	}
	if priceCol == -1 {
		if guessed := e.detectPriceColumn(rows, startRow); guessed >= 0 {
			priceCol = guessed
			columnMap["price"] = guessed
			log.Printf("🧠 Guessed price column: %d", guessed)
		} else if len(rows[0]) > 1 {
			priceCol = 1
			log.Printf("⚠️ Price column not found, falling back to column 1")
		} else {
			priceCol = 0
			log.Printf("⚠️ Price column not found, using column 0")
		}
	}

	categoryCol, hasCategory := columnMap["category"]
	descriptionCol, hasDescription := columnMap["description"]
	stockCol, hasStock := columnMap["stock"]

	var products []entity.Product
	now := time.Now()

	// Format aniqlash: Standart jadval (nom, narx, ...) yoki Side-by-side (nom1, narx1, nom2, narx2)
	isTableFormat := true
	// Header bo'lsa bu jadval formatida deb qabul qilamiz.
	if !hasHeader {
		isTableFormat = e.detectTableFormat(rows, startRow, priceCol)
	}

	if isTableFormat {
		log.Printf("📊 Detected: STANDARD TABLE format (Name | Price | ...)")
	} else {
		log.Printf("📊 Detected: SIDE-BY-SIDE format (Name1 | Price1 | Name2 | Price2 | ...)")
	}

	// Standart jadval formatini parse qilish
	if isTableFormat {
		for i := startRow; i < len(rows); i++ {
			row := rows[i]

			// Bo'sh qatorlarni skip qilish
			if len(row) == 0 || isEmptyRow(row) {
				continue
			}

			// Kamida 2 ta ustun bo'lishi kerak (nom va narx)
			if len(row) <= nameCol || len(row) <= priceCol {
				continue
			}

			nameStr := strings.TrimSpace(row[nameCol])
			priceStr := strings.TrimSpace(row[priceCol])

			// Bo'sh bo'lsa skip
			if nameStr == "" || priceStr == "" {
				continue
			}

			// Narxni parse qilish
			price, err := e.parsePrice(priceStr)
			if err != nil || price == 0 {
				log.Printf("⚠️ Row %d: Invalid price '%s' - skipping", i, priceStr)
				continue
			}

			// Nom juda qisqa bo'lsa skip
			if len(nameStr) < 3 {
				continue
			}

			// Mahsulot yaratish
			product := entity.Product{
				ID:        uuid.New().String(),
				Name:      nameStr,
				Price:     price,
				Category:  "Boshqa",
				CreatedAt: now,
				UpdatedAt: now,
				Specs:     make(map[string]string),
			}

			// Kategoriya - Excel dan yoki nomga qarab aniqlaymiz
			if hasCategory && categoryCol < len(row) {
				if category := strings.TrimSpace(row[categoryCol]); category != "" {
					product.Category = category
				} else {
					product.Category = e.detectCategory(nameStr)
				}
			} else {
				product.Category = e.detectCategory(nameStr)
			}

			// Tavsif
			if hasDescription && descriptionCol < len(row) {
				product.Description = strings.TrimSpace(row[descriptionCol])
			}

			// Ombordagi son
			if hasStock && stockCol < len(row) {
				if stockStr := strings.TrimSpace(row[stockCol]); stockStr != "" {
					if stock, err := e.parsePrice(stockStr); err == nil {
						product.Stock = int(stock)
					}
				}
			}

			// Qo'shimcha ustunlarni specs ga qo'shish (header bo'lsa nomlarini ishlatamiz)
			usedCols := map[int]struct{}{nameCol: {}, priceCol: {}}
			if hasCategory {
				usedCols[categoryCol] = struct{}{}
			}
			if hasDescription {
				usedCols[descriptionCol] = struct{}{}
			}
			if hasStock {
				usedCols[stockCol] = struct{}{}
			}

			if hasHeader {
				for idx, raw := range row {
					if _, used := usedCols[idx]; used {
						continue
					}
					value := strings.TrimSpace(raw)
					if value == "" {
						continue
					}

					key := fmt.Sprintf("Extra_%d", idx)
					if idx < len(header) && strings.TrimSpace(header[idx]) != "" {
						key = strings.TrimSpace(header[idx])
					}
					product.Specs[key] = value
				}
			} else {
				if len(row) > 2 {
					for j := 2; j < len(row); j++ {
						value := strings.TrimSpace(row[j])
						if value != "" {
							key := fmt.Sprintf("Extra_%d", j)
							product.Specs[key] = value
						}
					}
				}
			}

			log.Printf("✅ Found: %s - $%.2f (category: %s)", product.Name, product.Price, product.Category)
			products = append(products, product)
		}
	} else {
		// Side-by-side formatni parse qilish
		// Pattern: [Nom1, Narx1, Nom2, Narx2, ...]
		for i := startRow; i < len(rows); i++ {
			row := rows[i]

			// Bo'sh qatorlarni skip qilish
			if len(row) == 0 || isEmptyRow(row) {
				continue
			}

			log.Printf("📝 Row %d: %d columns", i, len(row))

			// Qatordagi barcha mahsulotlarni topish
			for colIdx := 0; colIdx < len(row); colIdx += 2 {
				// Nom va narx borligini tekshirish
				if colIdx+1 >= len(row) {
					break
				}

				nameStr := strings.TrimSpace(row[colIdx])
				priceStr := strings.TrimSpace(row[colIdx+1])

				// Bo'sh bo'lsa skip
				if nameStr == "" && priceStr == "" {
					continue
				}

				// Agar narx raqam bo'lmasa, skip
				price, err := e.parsePrice(priceStr)
				if err != nil || price == 0 {
					continue
				}

				// Agar nom juda qisqa bo'lsa skip
				if len(nameStr) < 3 {
					continue
				}

				// Mahsulot yaratish
				product := entity.Product{
					ID:        uuid.New().String(),
					Name:      nameStr,
					Price:     price,
					Category:  "Boshqa",
					CreatedAt: now,
					UpdatedAt: now,
					Specs:     make(map[string]string),
				}

				// Kategoriyani aniqlash
				product.Category = e.detectCategory(nameStr)

				log.Printf("✅ Found: %s - $%.2f (category: %s)", product.Name, product.Price, product.Category)
				products = append(products, product)
			}
		}
	}

	log.Printf("📦 Total products parsed: %d", len(products))

	if len(products) == 0 {
		return nil, fmt.Errorf("no valid products found in excel file (parsed %d rows, but all were invalid)", len(rows)-1)
	}

	return products, nil
}

// isEmptyRow qator bo'sh yoki yo'qligini tekshirish
func isEmptyRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

// detectTableFormat Excel formatini aniqlash
// true = Standart jadval (Nom | Narx | ...)
// false = Side-by-side (Nom1 | Narx1 | Nom2 | Narx2)
func (e *excelParser) detectTableFormat(rows [][]string, startRow int, priceCol int) bool {
	if len(rows) <= startRow {
		return true // Default: table format
	}

	// priceCol noto'g'ri bo'lsa, default 1-ustunni tekshiramiz
	if priceCol < 0 {
		priceCol = 1
	}

	// Bir nechta qatorni tekshirish (birinchi 5 ta data qatori)
	validPriceCount := 0
	totalChecked := 0

	for i := startRow; i < len(rows) && totalChecked < 5; i++ {
		row := rows[i]
		if len(row) <= priceCol || isEmptyRow(row) {
			continue
		}

		totalChecked++

		// priceCol ustunida narx bo'lsa - bu table format
		priceCandidate := strings.TrimSpace(row[priceCol])
		if _, err := e.parsePrice(priceCandidate); err == nil {
			validPriceCount++
		}
	}

	// Agar ko'pchilik qatorlarda priceCol ustuni narx bo'lsa - table format
	if totalChecked == 0 {
		return true
	}

	// Agar 70% dan ko'p qatorlarda priceCol narx bo'lsa - table format
	return float64(validPriceCount)/float64(totalChecked) > 0.7
}

// detectPriceColumn narx ustunini topish (agar headerda topilmasa)
func (e *excelParser) detectPriceColumn(rows [][]string, startRow int) int {
	maxCols := 0
	limitRows := startRow + 15
	if limitRows > len(rows) {
		limitRows = len(rows)
	}

	for i := startRow; i < limitRows; i++ {
		if len(rows[i]) > maxCols {
			maxCols = len(rows[i])
		}
	}

	bestCol := -1
	bestCount := 0

	for col := 0; col < maxCols; col++ {
		count := 0
		for i := startRow; i < limitRows; i++ {
			row := rows[i]
			if col >= len(row) {
				continue
			}
			val := strings.TrimSpace(row[col])
			if val == "" {
				continue
			}
			if _, err := e.parsePrice(val); err == nil {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			bestCol = col
		}
	}

	// Kamida 2 ta qator narx sifatida o'qilsa, shu ustunni narx deb olamiz
	if bestCount >= 2 {
		return bestCol
	}

	return -1
}

// mapColumns header qatoridan column mapping yaratish
func (e *excelParser) mapColumns(header []string) map[string]int {
	columnMap := make(map[string]int)

	for i, col := range header {
		colName := strings.ToLower(strings.TrimSpace(col))

		// Debug
		log.Printf("🔍 Checking column %d: '%s'", i, colName)

		// Standart maydonlar uchun mapping - JUDA KO'P VARIANTLAR
		switch {
		// NAME variants
		case contains(colName, "name", "nom", "nomi", "название", "product", "mahsulot", "tovar"):
			columnMap["name"] = i
			log.Printf("✅ Mapped 'name' to column %d", i)

		// CATEGORY variants
		case contains(colName, "category", "kategoriya", "tur", "тип", "категория", "type"):
			columnMap["category"] = i
			log.Printf("✅ Mapped 'category' to column %d", i)

		// PRICE variants
		case contains(colName, "price", "narx", "summa", "цена", "сум", "som", "cost", "$", "usd", "uzs"):
			columnMap["price"] = i
			log.Printf("✅ Mapped 'price' to column %d", i)

		// DESCRIPTION variants
		case contains(colName, "description", "tavsif", "malumot", "описание", "info", "details"):
			columnMap["description"] = i
			log.Printf("✅ Mapped 'description' to column %d", i)

		// STOCK variants
		case contains(colName, "stock", "soni", "miqdor", "количество", "qty", "quantity"):
			columnMap["stock"] = i
			log.Printf("✅ Mapped 'stock' to column %d", i)

		default:
			// Boshqa barcha columnlarni specs sifatida saqlash
			if colName != "" {
				columnMap[colName] = i
				log.Printf("📝 Mapped '%s' to specs (column %d)", colName, i)
			}
		}
	}

	// Agar asosiy maydonlar topilmasa, birinchi ustunlarni default mapping qilish
	if _, ok := columnMap["name"]; !ok && len(header) > 0 {
		columnMap["name"] = 0
		log.Printf("⚠️ No name column found, using column 0")
	}
	if _, ok := columnMap["price"]; !ok && len(header) > 1 {
		columnMap["price"] = 1
		log.Printf("⚠️ No price column found, using column 1")
	}

	return columnMap
}

// contains tekshirish uchun helper
func contains(str string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(str, keyword) {
			return true
		}
	}
	return false
}

// parsePrice narxni parse qilish
func (e *excelParser) parsePrice(priceStr string) (float64, error) {
	// Turli formatlarni qo'llab-quvvatlash
	priceStr = strings.ToLower(strings.TrimSpace(priceStr))

	// Bo'sh bo'lsa
	if priceStr == "" {
		return 0, fmt.Errorf("empty price")
	}

	// Tozalash
	priceStr = strings.ReplaceAll(priceStr, ",", "")
	priceStr = strings.ReplaceAll(priceStr, " ", "")
	priceStr = strings.ReplaceAll(priceStr, "$", "")
	priceStr = strings.ReplaceAll(priceStr, "€", "")
	priceStr = strings.ReplaceAll(priceStr, "£", "")
	priceStr = strings.ReplaceAll(priceStr, "₽", "")
	priceStr = strings.ReplaceAll(priceStr, "¥", "")
	priceStr = strings.ReplaceAll(priceStr, "so'm", "")
	priceStr = strings.ReplaceAll(priceStr, "soʻm", "")
	priceStr = strings.ReplaceAll(priceStr, "soum", "")
	priceStr = strings.ReplaceAll(priceStr, "som", "")
	priceStr = strings.ReplaceAll(priceStr, "сум", "")
	priceStr = strings.ReplaceAll(priceStr, "сом", "")
	priceStr = strings.ReplaceAll(priceStr, "sum", "")
	priceStr = strings.ReplaceAll(priceStr, "uzs", "")
	priceStr = strings.ReplaceAll(priceStr, "usd", "")
	priceStr = strings.ReplaceAll(priceStr, "eur", "")
	priceStr = strings.ReplaceAll(priceStr, "руб", "")
	priceStr = strings.ReplaceAll(priceStr, "р", "")

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %s", priceStr)
	}

	return price, nil
}

// detectCategory mahsulot nomidan kategoriyani aniqlash
// MUHIM: Eng aniq belgilarni birinchi tekshiramiz!
func (e *excelParser) detectCategory(name string) string {
	nameLower := strings.ToLower(name)

	// Monitor - BIRINCHI (aniq kategoriya)
	if strings.Contains(nameLower, "monitor") || strings.Contains(nameLower, "монитор") ||
		strings.Contains(nameLower, "display") || strings.Contains(nameLower, "screen") ||
		strings.Contains(nameLower, "144hz") || strings.Contains(nameLower, "165hz") ||
		strings.Contains(nameLower, "240hz") || strings.Contains(nameLower, "300hz") ||
		strings.Contains(nameLower, "ips") || strings.Contains(nameLower, "va panel") ||
		strings.Contains(nameLower, "curved") || strings.Contains(nameLower, "ultrawide") {
		return "Monitor"
	}

	// SSD/Storage
	if strings.Contains(nameLower, "ssd") || strings.Contains(nameLower, "nvme") ||
		strings.Contains(nameLower, "hdd") || strings.Contains(nameLower, "hard drive") {
		return "Storage"
	}

	// RAM - DDR aniq belgi
	if strings.Contains(nameLower, "ddr4") || strings.Contains(nameLower, "ddr5") ||
		strings.Contains(nameLower, "ddr3") || strings.Contains(nameLower, "ddr ") {
		return "RAM"
	}

	// CPU - Processor modellari
	if strings.Contains(nameLower, "intel") || strings.Contains(nameLower, "amd") ||
		strings.Contains(nameLower, "ryzen") || strings.Contains(nameLower, "core i") ||
		strings.Contains(nameLower, "processor") || strings.Contains(nameLower, "xeon") {
		return "CPU"
	}

	// GPU - Video karta
	if strings.Contains(nameLower, "rtx") || strings.Contains(nameLower, "gtx") ||
		strings.Contains(nameLower, "radeon") || strings.Contains(nameLower, "rx ") ||
		strings.Contains(nameLower, "geforce") || strings.Contains(nameLower, "nvidia") ||
		strings.Contains(nameLower, "arc") || strings.Contains(nameLower, "inno3d") {
		return "GPU"
	}

	// Motherboard - Ona plata
	if strings.Contains(nameLower, "motherboard") ||
		strings.Contains(nameLower, "b450") || strings.Contains(nameLower, "b550") ||
		strings.Contains(nameLower, "b650") || strings.Contains(nameLower, "b760") ||
		strings.Contains(nameLower, "x570") || strings.Contains(nameLower, "x670") ||
		strings.Contains(nameLower, "x870") || strings.Contains(nameLower, "z690") ||
		strings.Contains(nameLower, "z790") || strings.Contains(nameLower, "lga1700") ||
		strings.Contains(nameLower, "lga1851") || strings.Contains(nameLower, "am4") ||
		strings.Contains(nameLower, "am5") {
		return "Motherboard"
	}

	// PSU - Quvvat bloki
	if strings.Contains(nameLower, "psu") || strings.Contains(nameLower, "power supply") ||
		strings.Contains(nameLower, "блок питания") || strings.Contains(nameLower, "watt") ||
		strings.Contains(nameLower, "w ") && (strings.Contains(nameLower, "80+") || strings.Contains(nameLower, "bronze") || strings.Contains(nameLower, "gold")) {
		return "PSU"
	}

	// Case - Korpus
	if strings.Contains(nameLower, "case") || strings.Contains(nameLower, "корпус") ||
		strings.Contains(nameLower, "chassis") || strings.Contains(nameLower, "tower") {
		return "Case"
	}

	// Cooling - Sovutgich
	if strings.Contains(nameLower, "cooler") || strings.Contains(nameLower, "cooling") ||
		strings.Contains(nameLower, "fan") || strings.Contains(nameLower, "aio") ||
		strings.Contains(nameLower, "liquid") || strings.Contains(nameLower, "air cooler") {
		return "Cooling"
	}

	// Chair - Stul
	if strings.Contains(nameLower, "chair") || strings.Contains(nameLower, "стул") ||
		strings.Contains(nameLower, "gaming chair") || strings.Contains(nameLower, "office chair") {
		return "Chair"
	}

	// Desk - Stol
	if strings.Contains(nameLower, "desk") || strings.Contains(nameLower, "стол") ||
		strings.Contains(nameLower, "table") || strings.Contains(nameLower, "gaming desk") {
		return "Desk"
	}

	// Keyboard
	if strings.Contains(nameLower, "keyboard") || strings.Contains(nameLower, "клавиатура") {
		return "Keyboard"
	}

	// Mouse
	if strings.Contains(nameLower, "mouse") || strings.Contains(nameLower, "мышь") {
		return "Mouse"
	}

	// Headset
	if strings.Contains(nameLower, "headset") || strings.Contains(nameLower, "headphone") ||
		strings.Contains(nameLower, "наушники") {
		return "Headset"
	}

	// RAM - Umumiy kalit so'zlar (DDR dan keyin)
	if strings.Contains(nameLower, "ram") || strings.Contains(nameLower, "memory") ||
		strings.Contains(nameLower, "corsair vengeance") || strings.Contains(nameLower, "kingston fury") {
		return "RAM"
	}

	return "Boshqa"
}
