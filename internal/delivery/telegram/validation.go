package telegram

import (
	"regexp"
	"strings"
)

// validatePhoneNumber telefon raqamni validatsiya qilish
func validatePhoneNumber(phone string) bool {
	// Bo'shliqlarni olib tashlash
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.TrimSpace(phone)

	// Telefon raqam regex: +998901234567 yoki 901234567 yoki 998901234567
	phoneRegex := regexp.MustCompile(`^\+?[0-9]{9,15}$`)
	return phoneRegex.MatchString(phone)
}

// validateName ism validatsiyasi
func validateName(name string) bool {
	name = strings.TrimSpace(name)
	// Kamida 2 ta harf bo'lishi kerak
	if len(name) < 2 {
		return false
	}
	//Faqat harflar, bo'shliqlar va ba'zi maxsus belgilar
	nameRegex := regexp.MustCompile(`^[a-zA-ZÀ-ÿА-яЁёўӮөҒғҚқҲҳ\s'-]+$`)
	return nameRegex.MatchString(name)
}
