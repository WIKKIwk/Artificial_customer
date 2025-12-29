package telegram

import (
	"strings"
	"unicode"
)

// validatePhoneNumber telefon raqamni validatsiya qilish
func validatePhoneNumber(phone string) bool {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return false
	}

	var digits strings.Builder
	digits.Grow(len(phone))
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}

	// Allowed: 7..15 digits (e.g. 901234567, 1234567, 998901234567)
	n := digits.Len()
	return n >= 7 && n <= 15
}

// validateName ism validatsiyasi
func validateName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}

	letters := 0
	for _, r := range name {
		switch {
		case unicode.IsLetter(r):
			letters++
		case unicode.IsSpace(r):
			// allow spaces between words
		case r == '\'' || r == '’' || r == '‘' || r == 'ʼ' || r == '-':
			// allow common apostrophes / hyphen in Uzbek names
		default:
			// digits, dots, commas, etc. are not allowed
			return false
		}
	}

	// Kamida 2 ta harf bo'lishi kerak
	return letters >= 2
}
