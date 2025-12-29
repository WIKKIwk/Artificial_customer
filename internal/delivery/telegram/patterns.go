package telegram

import (
	"regexp"
	"strings"
)

var (
	// Matches prices with currency either suffix or prefix (e.g. "250$", "$250", "250 USD", "USD 250").
	priceWithCurrencyRegex = regexp.MustCompile(`(?i)(?:(?:\$|usd|so['’]?m|sum|сум|eur|€|rub|₽)\s*[0-9][0-9\s,.]*|[0-9][0-9\s,.]*\s*(?:\$|usd|so['’]?m|sum|сум|eur|€|rub|₽))`)
	cpuModelRegex          = regexp.MustCompile(`(?i)\b(ryzen|intel|i[3579]-?\d{3,5}|xeon|threadripper)`)
	gpuModelRegex          = regexp.MustCompile(`(?i)\b(rtx|gtx|rx)\s*\d{3,4}`)
	genericModelRegex      = regexp.MustCompile(`(?i)\b(laptop|notebook|kompyuter|computer|pc|gpu|cpu)`)
)

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:max]) + "…"
}

func isConfigRequest(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}

	// Agar user "yuqoridagi konfiguratsiya", "bu konfiguratsiya" kabi haqida so'rasa, false qaytarish
	excludePatterns := []string{
		"yuqoridagi", "bu ", "shu ", "ushbu", "o'sha", "этот", "данн", "вышеупомянут",
		"yaxshimi", "yoqdimi", "qanday", "yomon", "yaxshi", "zo'r", "ajoyib", "yoqmadi",
	}
	for _, excl := range excludePatterns {
		if strings.Contains(lower, excl) {
			return false
		}
	}

	// Budjet yoki narx so'rash ("pc qancha?") konfiguratsiya so'rovi emas.
	if strings.Contains(lower, "pc") || strings.Contains(lower, "kompyuter") || strings.Contains(lower, "computer") {
		priceAsking := []string{"qancha", "narx", "price", "цена", "сколько"}
		for _, kw := range priceAsking {
			if strings.Contains(lower, kw) {
				return false
			}
		}
	}

	// Aniq PC yig'ish / sborka so'rovlari
	buildKeywords := []string{
		"pc tuz", "pc yig", "pc yig'", "pc yigib", "pc yig'ib",
		"kompyuter yig", "компьютер собра", "систему собра", "сборк", "sborka", "sbor",
		"konfiguratsiya", "конфиг", "сборка", "assemble", "build pc",
	}
	for _, kw := range buildKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	// "gaming pc kerak" kabi so'rovlar: yig'ish so'zi bo'lmasa ham konfiguratsiya niyatini bildiradi.
	if (strings.Contains(lower, "pc") || strings.Contains(lower, "kompyuter") || strings.Contains(lower, "computer")) &&
		(strings.Contains(lower, "kerak") || strings.Contains(lower, "xohlayman") || strings.Contains(lower, "хочу") || strings.Contains(lower, "нуж")) &&
		(strings.Contains(lower, "gaming") || strings.Contains(lower, "o'yin") || strings.Contains(lower, "oʻyin") || strings.Contains(lower, "o‘yin") ||
			strings.Contains(lower, "игр") || strings.Contains(lower, "игров")) {
		return true
	}

	return false
}
