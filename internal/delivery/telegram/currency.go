package telegram

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var dollarPricePattern = regexp.MustCompile(`(?i)(\$\s*[0-9][0-9\s.,]*|[0-9][0-9\s.,]*\s*\$|[0-9][0-9\s.,]*\s*usd)`)

func (h *BotHandler) setCurrencyMode(mode string, rate float64) {
	h.currencyMu.Lock()
	h.currencyMode = strings.ToLower(strings.TrimSpace(mode))
	if rate > 0 {
		h.currencyRate = rate
	}
	h.currencyMu.Unlock()
}

func (h *BotHandler) setCurrencyRate(rate float64) {
	h.currencyMu.Lock()
	h.currencyRate = rate
	h.currencyMu.Unlock()
}

func (h *BotHandler) getCurrencySettings() (string, float64) {
	h.currencyMu.RLock()
	defer h.currencyMu.RUnlock()
	return h.currencyMode, h.currencyRate
}

func (h *BotHandler) setAwaitingCurrencyRate(userID int64, awaiting bool) {
	h.currencyMu.Lock()
	if awaiting {
		h.currencyAwait[userID] = true
	} else {
		delete(h.currencyAwait, userID)
	}
	h.currencyMu.Unlock()
}

func (h *BotHandler) isAwaitingCurrencyRate(userID int64) bool {
	h.currencyMu.RLock()
	defer h.currencyMu.RUnlock()
	return h.currencyAwait[userID]
}

func parseDollarAmount(text string) (float64, bool) {
	clean := strings.ToLower(text)
	clean = strings.ReplaceAll(clean, "usd", "")
	clean = strings.ReplaceAll(clean, "$", "")
	clean = strings.TrimSpace(clean)
	// Normalizatsiya: vergul/bo'shliqlarni olib tashlash
	clean = strings.ReplaceAll(clean, " ", "")
	if strings.Count(clean, ",") == 1 && strings.Count(clean, ".") == 0 {
		clean = strings.ReplaceAll(clean, ",", ".")
	} else {
		clean = strings.ReplaceAll(clean, ",", "")
	}
	val, err := strconv.ParseFloat(clean, 64)
	if err != nil || val <= 0 {
		return 0, false
	}
	return val, true
}

func formatUZS(v float64) string {
	neg := v < 0
	n := int64(math.Round(math.Abs(v)/1000) * 1000)
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, " ") + " so'm"
	if neg && n != 0 {
		return "-" + out
	}
	return out
}

// applyCurrencyPreference - agar admin SUM rejimini tanlagan bo'lsa, $ narxlarni so'mga o'girish
func (h *BotHandler) applyCurrencyPreference(text string) string {
	mode, rate := h.getCurrencySettings()
	if strings.ToLower(mode) != "sum" || rate <= 0 {
		return text
	}

	convert := func(match string) string {
		amt, ok := parseDollarAmount(match)
		if !ok {
			return match
		}
		converted := amt * rate
		return fmt.Sprintf("%s (~%s)", strings.TrimSpace(match), formatUZS(converted))
	}

	return dollarPricePattern.ReplaceAllStringFunc(text, convert)
}

// formatTotalForDisplay ensures we don't double-convert totals that already contain so'm equivalents.
func (h *BotHandler) formatTotalForDisplay(total string) string {
	lower := strings.ToLower(total)
	if strings.Contains(lower, "~") || strings.Contains(lower, "so'm") || strings.Contains(lower, "сум") {
		return total
	}
	return h.applyCurrencyPreference(total)
}
