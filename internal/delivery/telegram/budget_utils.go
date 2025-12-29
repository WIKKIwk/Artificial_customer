package telegram

import (
	"regexp"
	"strconv"
	"strings"
)

var budgetKRegex = regexp.MustCompile(`(?i)(\d+(?:[.,]\d+)?)\s*k`)

func parseBudgetUSD(input string) float64 {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return 0
	}

	if match := budgetKRegex.FindStringSubmatch(lower); len(match) > 1 {
		num := strings.ReplaceAll(match[1], ",", ".")
		if v, err := strconv.ParseFloat(num, 64); err == nil {
			return v * 1000
		}
	}

	if v, ok := parseNumberWithSeparators(lower); ok {
		if strings.Contains(lower, "mln") || strings.Contains(lower, "million") {
			return v * 1000000
		}
		return v
	}

	return 0
}
