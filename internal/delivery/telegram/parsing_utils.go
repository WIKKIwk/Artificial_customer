package telegram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var jamiRegex = regexp.MustCompile(`(?i)^(?:jami|итого|summa|total|overall\s*(?:price|total)?|umumiy\s*(?:narx|summa)?|общая\s*(?:цена|стоимость)?|всего)(?:\s|:|-|$)`)
var requirementsHeader = regexp.MustCompile(`(?i)talablaringizni yozib oldim|ваши требования|requirements`)
var priceLabelRegex = regexp.MustCompile(`(?i)\b(narxi|narx|price|jami|itogo|total)\b[: ]*`)
var numberedItemRegex = regexp.MustCompile(`^\s*(\d{1,2})[.)]\s*(.+)$`)

func stripBulletPrefix(s string) string {
	return strings.TrimSpace(strings.TrimLeft(s, "*-•— "))
}

func hasTotalLine(text string) bool {
	for _, ln := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		trim = stripBulletPrefix(trim)
		if jamiRegex.MatchString(trim) {
			return true
		}
	}
	return false
}

func parseSelectionIndex(text string) (int, bool) {
	trim := strings.TrimSpace(text)
	if trim == "" {
		return 0, false
	}
	if len(trim) > 2 {
		return 0, false
	}
	n, err := strconv.Atoi(trim)
	if err != nil || n < 1 || n > 9 {
		return 0, false
	}
	return n, true
}

func extractNumberedItems(text string) []string {
	items := []string{}
	for _, line := range strings.Split(text, "\n") {
		match := numberedItemRegex.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil || len(match) < 3 {
			continue
		}
		item := strings.TrimSpace(match[2])
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	return items
}

func countNumberedVariants(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' {
			if trimmed[1] == '.' || trimmed[1] == ')' {
				count++
			}
		}
	}
	return count
}

func isSingleProductSuggestion(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if hasTotalLine(text) {
		return true
	}
	if countNumberedVariants(text) >= 2 {
		return false
	}
	if hasPriceInfoButTooManyOptions(text) {
		return false
	}
	prices := priceWithCurrencyRegex.FindAllString(text, -1)
	if len(prices) == 0 {
		return false
	}
	if len(prices) == 1 {
		return true
	}
	nonEmpty := 0
	for _, ln := range strings.Split(text, "\n") {
		if strings.TrimSpace(ln) != "" {
			nonEmpty++
		}
	}
	return nonEmpty <= 2
}

func extractTotalPrice(text string) string {
	// 1) Explicit total line (Jami/Итого/Overall/Total/Umumiy/Всего...) has priority.
	for _, ln := range strings.Split(text, "\n") {
		trim := stripBulletPrefix(strings.TrimSpace(ln))
		if trim == "" {
			continue
		}
		if !jamiRegex.MatchString(trim) {
			continue
		}
		if price := bestPriceFromLine(trim); price != "" {
			return price
		}
	}

	// 2) Fallback: last price in text (skip budget lines); prefer USD if present.
	var lastUSD string
	var lastAny string
	for _, ln := range strings.Split(text, "\n") {
		trim := stripBulletPrefix(strings.TrimSpace(ln))
		if trim == "" {
			continue
		}
		lower := strings.ToLower(trim)
		if strings.Contains(lower, "budjet") || strings.Contains(lower, "budget") {
			continue
		}
		for _, m := range priceWithCurrencyRegex.FindAllString(trim, -1) {
			m = strings.TrimSpace(m)
			lastAny = m
			if canonicalCurrencyFromMatch(m) == "$" {
				lastUSD = m
			}
		}
	}
	if lastUSD != "" {
		return lastUSD
	}
	return lastAny
}

func canonicalCurrencyFromMatch(match string) string {
	lower := strings.ToLower(match)
	switch {
	case strings.Contains(lower, "$") || strings.Contains(lower, "usd"):
		return "$"
	case strings.Contains(lower, "€") || strings.Contains(lower, "eur"):
		return "€"
	case strings.Contains(lower, "₽") || strings.Contains(lower, "rub"):
		return "₽"
	case strings.Contains(lower, "so'm") || strings.Contains(lower, "so’m") ||
		strings.Contains(lower, "сум") || strings.Contains(lower, " sum") || strings.HasSuffix(lower, "sum"):
		return "so'm"
	default:
		return ""
	}
}

func bestPriceFromLine(line string) string {
	matches := priceWithCurrencyRegex.FindAllString(line, -1)
	if len(matches) == 0 {
		return ""
	}
	for _, m := range matches {
		if canonicalCurrencyFromMatch(m) == "$" {
			return strings.TrimSpace(m)
		}
	}
	return strings.TrimSpace(matches[0])
}

func parseNumberWithSeparators(raw string) (float64, bool) {
	// Keep only digits and separators.
	var b strings.Builder
	for _, r := range raw {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' || r == ' ' {
			b.WriteRune(r)
		}
	}
	clean := strings.TrimSpace(b.String())
	if clean == "" {
		return 0, false
	}
	clean = strings.ReplaceAll(clean, " ", "")

	dot := strings.LastIndex(clean, ".")
	comma := strings.LastIndex(clean, ",")

	switch {
	case dot >= 0 && comma >= 0:
		// Both present: rightmost is decimal separator.
		if dot > comma {
			clean = strings.ReplaceAll(clean, ",", "")
		} else {
			clean = strings.ReplaceAll(clean, ".", "")
			clean = strings.ReplaceAll(clean, ",", ".")
		}
	case dot >= 0:
		if strings.Count(clean, ".") > 1 {
			clean = strings.ReplaceAll(clean, ".", "")
		} else if after := clean[dot+1:]; len(after) == 3 {
			// Likely thousand separator (e.g., 1.400)
			clean = strings.ReplaceAll(clean, ".", "")
		}
	case comma >= 0:
		if strings.Count(clean, ",") > 1 {
			clean = strings.ReplaceAll(clean, ",", "")
		} else if after := clean[comma+1:]; len(after) == 3 {
			// Likely thousand separator (e.g., 1,400)
			clean = strings.ReplaceAll(clean, ",", "")
		} else {
			// Decimal separator
			clean = strings.ReplaceAll(clean, ",", ".")
		}
	}

	val, err := strconv.ParseFloat(clean, 64)
	if err != nil || val <= 0 {
		return 0, false
	}
	return val, true
}

func parseAmountFromPriceMatch(match string) (float64, bool) {
	lower := strings.ToLower(match)
	for _, tok := range []string{"usd", "$", "eur", "€", "rub", "₽", "so'm", "so’m", "сум", "sum"} {
		lower = strings.ReplaceAll(lower, tok, "")
	}
	return parseNumberWithSeparators(lower)
}

// sumPriceLines - matndagi barcha narxlarni yig'adi va topilgan valyutada qaytaradi.
func sumPriceLines(text string) string {
	type entry struct {
		amount   float64
		currency string
	}
	var entries []entry

	for _, ln := range strings.Split(text, "\n") {
		trim := stripBulletPrefix(strings.TrimSpace(ln))
		if trim == "" {
			continue
		}
		// Total lines should not be included in summation (prevents double counting).
		if jamiRegex.MatchString(trim) {
			continue
		}
		for _, m := range priceWithCurrencyRegex.FindAllString(trim, -1) {
			cur := canonicalCurrencyFromMatch(m)
			amt, ok := parseAmountFromPriceMatch(m)
			if !ok || amt <= 0 {
				continue
			}
			entries = append(entries, entry{amount: amt, currency: cur})
		}
	}

	if len(entries) == 0 {
		return ""
	}

	target := ""
	for _, e := range entries {
		if e.currency == "$" {
			target = "$"
			break
		}
	}
	if target == "" {
		target = entries[0].currency
	}
	if target == "" {
		target = "$"
	}

	sum := 0.0
	for _, e := range entries {
		if e.currency != target {
			continue
		}
		sum += e.amount
	}

	if sum <= 0 {
		return ""
	}

	// Butun son bo'lsa kasrsiz, aks holda ikki xona
	if sum == float64(int64(sum)) {
		return fmt.Sprintf("%.0f%s", sum, target)
	}
	return fmt.Sprintf("%.2f%s", sum, target)
}

func removeJamiLines(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	for _, ln := range lines {
		if jamiRegex.MatchString(stripBulletPrefix(strings.TrimSpace(ln))) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// removeRequirementBlocks - "Talablaringizni yozib oldim" blokini kesib tashlaydi
func removeRequirementBlocks(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	skip := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			if !skip {
				out = append(out, ln)
			}
			continue
		}
		lower := strings.ToLower(t)
		if requirementsHeader.MatchString(lower) {
			skip = true
			continue
		}
		// Agar skip holatida bo'lsak, ehtimoliy talablar qatorlarini tashlab ketamiz (budjet/cpu/gpu/xotira)
		if skip {
			if strings.Contains(lower, "budjet") || strings.Contains(lower, "budget") ||
				strings.Contains(lower, "cpu") || strings.Contains(lower, "gpu") ||
				strings.Contains(lower, "xotira") || strings.Contains(lower, "storage") ||
				strings.Contains(lower, "nomi") || strings.Contains(lower, "maqsad") {
				continue
			}
			// Bo'sh qator kelganda skip tugaydi
			if t == "" {
				skip = false
			}
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func isLikelyProductSuggestion(text string) bool {
	if priceWithCurrencyRegex.MatchString(text) {
		return true
	}
	lines := strings.Split(text, "\n")
	cnt := 0
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "•") || strings.HasPrefix(trim, "*") || strings.HasPrefix(trim, "—") {
			cnt++
		}
		if priceWithCurrencyRegex.MatchString(trim) {
			cnt++
		}
	}
	// Yakuniy holat: bitta "--" va narx mavjud bo'lsa ham yakka product deb olamiz
	if cnt >= 2 {
		return true
	}
	if strings.Count(text, "--") == 1 && priceWithCurrencyRegex.MatchString(text) {
		return true
	}
	return false
}

func addDashPrefixToProducts(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		lower := strings.ToLower(trim)
		if jamiRegex.MatchString(lower) {
			out = append(out, trim)
			continue
		}
		if priceWithCurrencyRegex.MatchString(trim) {
			if strings.HasPrefix(trim, "--") {
				out = append(out, trim)
			} else {
				out = append(out, "-- "+trim)
			}
			continue
		}
		out = append(out, trim)
	}
	return strings.Join(out, "\n")
}

func hasConfigIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	for _, kw := range []string{"config", "konfig", "yig'", "pc", "sbor", "сбор", "компьютер"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func isLikelyConfigResponse(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{"cpu", "gpu", "ram", "ssd", "hdd", "motherboard", "protsessor", "videokarta", "grafik"}
	hit := 0
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			hit++
		}
	}
	return hasCPUAndRAM(lower) && (hit >= 2 || priceWithCurrencyRegex.MatchString(text))
}

// hasCPUAndRAM returns true if CPU, RAM, and "Jami" (total price) all exist
// This ensures we only detect FULL PC configurations, not just product mentions
// AI uses "Jami:" ONLY when giving full PC configs, not in simple product suggestions
func hasCPUAndRAM(lower string) bool {
	cpu := strings.Contains(lower, "cpu") || strings.Contains(lower, "protsessor") || strings.Contains(lower, "processor")
	ram := strings.Contains(lower, "ram") || strings.Contains(lower, "ozu") || strings.Contains(lower, " оператив")
	hasTotal := strings.Contains(lower, "jami:") || strings.Contains(lower, "jami :") ||
		strings.Contains(lower, "итого:") || strings.Contains(lower, "total:")
	return cpu && ram && hasTotal
}

func formatProductSuggestion(text string) string {
	title := cartTitleFromText(text)
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(text)
	}
	price := extractTotalPrice(text)
	msg := fmt.Sprintf("Ha, bizda bor: -- %s", title)
	if price != "" {
		msg += fmt.Sprintf("\nNarxi: %s", price)
		msg += "\nJami: " + price
	}
	return msg
}

func formatDisplaySummary(summary string) string {
	summaryClean := removeJamiLines(summary)
	summaryClean = removeRequirementBlocks(summaryClean)
	summaryClean = trimBeforeFirstSpecBlock(summaryClean)
	if names := extractProductNames(summaryClean); len(names) > 0 {
		var bullets []string
		for _, n := range names {
			bullets = append(bullets, "• "+n)
		}
		return strings.Join(bullets, "\n")
	}
	return summaryClean
}

// sanitizeOrderSummaryForAdmin - foydalanuvchiga ko'rsatilgan promptlarni va takrorlarni olib tashlaydi
func sanitizeOrderSummaryForAdmin(text string) string {
	clean := removeRequirementBlocks(text)
	clean = removeJamiLines(clean)
	lines := strings.Split(clean, "\n")
	var out []string
	seen := make(map[string]struct{})
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		// Keraksiz promptlar
		if strings.Contains(lower, "tugmasini bos") ||
			strings.Contains(lower, "button bos") ||
			strings.Contains(lower, "buttonni bos") ||
			strings.Contains(lower, "savatcha") ||
			strings.Contains(lower, "savatchadagi") ||
			strings.Contains(lower, "savatdagi") ||
			strings.Contains(lower, "yana mahsulot") ||
			strings.Contains(lower, "boshqa narsa kerakmi") ||
			strings.Contains(lower, "qo'shasizmi") ||
			strings.Contains(lower, "qo'shasiz") ||
			strings.Contains(lower, "tanlang") ||
			strings.Contains(lower, "bosib") && strings.Contains(lower, "sotib") ||
			strings.Contains(lower, "sotib oling tugmasini") {
			continue
		}
		// Admin xabarda tavsiyalar/upgrade blokini umuman ko'rsatmaymiz
		if strings.Contains(lower, "tavsiya") ||
			strings.Contains(lower, "upgrade") ||
			strings.Contains(lower, "bottleneck") ||
			strings.Contains(lower, "performance expectation") ||
			strings.Contains(lower, "cooling adequacy") ||
			strings.Contains(lower, "psu sufficiency") ||
			strings.Contains(lower, "cpu/gpu balance") {
			continue
		}
		if _, ok := seen[strings.ToLower(t)]; ok {
			continue
		}
		seen[strings.ToLower(t)] = struct{}{}
		out = append(out, t)
	}
	return strings.Join(out, "\n")
}

// formatLinesWithPrices - har bir qatordagi narxni ajratib, "Title - Narx" ko'rinishiga keltiradi
func formatLinesWithPrices(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	seen := make(map[string]struct{})
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		// Bullet/prefixlarni olib tashlaymiz
		tNoBullet := strings.TrimLeft(t, "*-• ")
		price := ""
		if m := priceWithCurrencyRegex.FindString(tNoBullet); m != "" {
			price = strings.TrimSpace(m)
		}
		title := tNoBullet
		if price != "" {
			// Narx va "Narxi" so'zini olib tashlaymiz
			title = priceWithCurrencyRegex.ReplaceAllString(tNoBullet, "")
			title = priceLabelRegex.ReplaceAllString(title, "")
			title = strings.Trim(title, "-:.,; ")
		}
		if strings.TrimSpace(title) == "" {
			title = strings.TrimSpace(tNoBullet)
		}
		formatted := strings.TrimSpace(title)
		if price != "" && formatted != "" {
			formatted = fmt.Sprintf("%s - %s", formatted, price)
		}
		if formatted == "" {
			continue
		}
		key := strings.ToLower(formatted)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, "• "+formatted)
	}
	return strings.Join(out, "\n")
}

// sanitizeConfigResponse - konfiguratsiya javobidan PSU hisobi va narx breakdown bloklarini olib tashlaydi
func sanitizeConfigResponse(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	skipPSU := false
	skipRecs := false

	looksLikeConfigLine := func(lower string) bool {
		base := strings.TrimSpace(strings.TrimLeft(lower, "*-• "))
		if base == "" {
			return false
		}
		if strings.HasPrefix(base, "cpu") ||
			strings.HasPrefix(base, "ram") ||
			strings.HasPrefix(base, "gpu") ||
			strings.HasPrefix(base, "ssd") ||
			strings.HasPrefix(base, "hdd") ||
			strings.HasPrefix(base, "motherboard") ||
			strings.HasPrefix(base, "psu") ||
			strings.HasPrefix(base, "case") ||
			strings.HasPrefix(base, "cooler") ||
			strings.HasPrefix(base, "monitor") ||
			strings.HasPrefix(base, "overall price") ||
			strings.HasPrefix(base, "jami") ||
			strings.HasPrefix(base, "price:") {
			return true
		}
		return false
	}

	isRecommendationsHeading := func(lower string) bool {
		return strings.Contains(lower, "tavsiya") ||
			strings.Contains(lower, "upgrade") ||
			strings.Contains(lower, "рекомендац") ||
			strings.Contains(lower, "апгрейд")
	}

	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		lower := strings.ToLower(t)

		// PSU/Quvvat bloki bloklarini tashlab yuboramiz, narx breakdown qolsin
		if strings.Contains(lower, "quvvat bloki") ||
			strings.Contains(lower, "psu hisobi") ||
			strings.Contains(lower, "quvvat sarfi") ||
			strings.Contains(lower, "zaxira quvvat") {
			skipPSU = true
			continue
		}

		// Tavsiyalar/upgrade blokini butunlay olib tashlaymiz (user so'ramagan bo'lsa kerak emas).
		if !skipRecs && isRecommendationsHeading(lower) {
			skipRecs = true
			continue
		}

		// Bo'sh qator kelsa skip tugaydi
		if t == "" {
			skipPSU = false
			if skipRecs {
				continue
			}
			out = append(out, ln)
			continue
		}
		if skipPSU {
			continue
		}

		if skipRecs {
			// Agar AI tasodifan tavsiyalar blokidan keyin yana konfiguratsiya yozib qolsa, uni saqlab qolamiz.
			if looksLikeConfigLine(lower) {
				skipRecs = false
			} else {
				continue
			}
		}

		out = append(out, ln)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// trimBeforeFirstSpecBlock - CPU/GPU/RAM bloklaridan oldingi kirish matnlarini kesib tashlaydi
func trimBeforeFirstSpecBlock(text string) string {
	lines := strings.Split(text, "\n")
	cpuIndex := -1
	firstSpec := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		base := strings.TrimLeft(lower, "*-• ")
		if strings.HasPrefix(base, "cpu") {
			cpuIndex = i
			break
		}
		if strings.HasPrefix(base, "gpu") || strings.HasPrefix(base, "ram") ||
			strings.HasPrefix(base, "ssd") || strings.HasPrefix(base, "hdd") || strings.HasPrefix(base, "motherboard") ||
			strings.HasPrefix(base, "psu") || strings.HasPrefix(base, "case") || strings.HasPrefix(base, "cooling") {
			if firstSpec == -1 {
				firstSpec = i
			}
		}
	}
	if cpuIndex >= 0 {
		return strings.Join(lines[cpuIndex:], "\n")
	}
	if firstSpec >= 0 {
		return strings.Join(lines[firstSpec:], "\n")
	}
	return text
}

// normalizeSpecBlock - CPU dan boshlab keladigan blokni olib, bir xil bullet formatiga keltiradi
func normalizeSpecBlock(text string) string {
	block := trimBeforeFirstSpecBlock(text)
	lines := strings.Split(block, "\n")
	var out []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		lower := strings.ToLower(t)
		base := strings.TrimLeft(lower, "*-• ")
		if strings.Contains(base, "jami") {
			break
		}
		if strings.Contains(base, "ajoyib tanlov") ||
			strings.Contains(base, "budjetga mos") ||
			strings.Contains(base, "talablaringizga mos") ||
			strings.Contains(base, "konfiguratsiyani taklif") ||
			strings.Contains(base, "variantlarni") ||
			strings.Contains(base, "konfiguratsiya") {
			continue
		}
		t = strings.TrimLeft(t, "*•- ")
		out = append(out, "* "+t)
	}
	return strings.Join(out, "\n")
}

// detectComponentKeyword - matnda komponent kalit so'zini aniqlash (cpu/gpu/ram/ssd/other)
func detectComponentKeyword(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "cooler") || strings.Contains(lower, "sovutgich") || strings.Contains(lower, "охлаж"):
		return "cooler"
	case (strings.Contains(lower, "cpu") || strings.Contains(lower, "protsessor") || strings.Contains(lower, "processor")) && !strings.Contains(lower, "cooler"):
		return "cpu"
	case strings.Contains(lower, "gpu") || strings.Contains(lower, "videokarta") || strings.Contains(lower, "grafik") || strings.Contains(lower, "video"):
		return "gpu"
	case strings.Contains(lower, "ram") || strings.Contains(lower, " оператив") || strings.Contains(lower, "xotira") && strings.Contains(lower, "gb"):
		return "ram"
	case strings.Contains(lower, "motherboard") || strings.Contains(lower, "anakart") || strings.Contains(lower, "mat plata") || strings.Contains(lower, "mobo"):
		return "motherboard"
	case strings.Contains(lower, "psu") || strings.Contains(lower, "power supply") || strings.Contains(lower, "quvvat bloki") || strings.Contains(lower, "blok pitaniya"):
		return "psu"
	case strings.Contains(lower, "case") || strings.Contains(lower, "korpus") || strings.Contains(lower, "корпус"):
		return "case"
	case strings.Contains(lower, "monitor"):
		return "monitor"
	case strings.Contains(lower, "ssd") || strings.Contains(lower, "hdd") || strings.Contains(lower, "nvme"):
		return "ssd"
	default:
		return ""
	}
}
