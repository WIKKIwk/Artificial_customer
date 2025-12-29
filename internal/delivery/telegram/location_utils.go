package telegram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var latLonPattern = regexp.MustCompile(`(?i)lat:\s*([-0-9]+(?:\.[0-9]+)?)\s*,?\s*lon:\s*([-0-9]+(?:\.[0-9]+)?)`)

func normalizeLocationText(location string) string {
	loc := strings.TrimSpace(location)
	if loc == "" {
		return ""
	}

	lower := strings.ToLower(loc)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return loc
	}

	if m := latLonPattern.FindStringSubmatch(loc); len(m) == 3 {
		lat, err1 := strconv.ParseFloat(m[1], 64)
		lon, err2 := strconv.ParseFloat(m[2], 64)
		if err1 == nil && err2 == nil {
			return fmt.Sprintf("https://www.google.com/maps?q=%.5f,%.5f", lat, lon)
		}
	}

	return loc
}
