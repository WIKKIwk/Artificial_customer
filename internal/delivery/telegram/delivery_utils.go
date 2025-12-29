package telegram

import "strings"

func deliveryDisplay(delivery, lang string) string {
	raw := strings.TrimSpace(delivery)
	if raw == "" {
		return ""
	}
	normalized := strings.ToLower(raw)
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.Join(strings.Fields(normalized), " ")

	switch normalized {
	case "pickup":
		return t(lang, "Olib ketish", "Самовывоз")
	case "courier", "yandex go", "yandexgo", "yandex":
		return t(lang, "Dostavka (Yandex Go)", "Доставка (Yandex Go)")
	default:
		if strings.Contains(normalized, "yandex") {
			return t(lang, "Dostavka (Yandex Go)", "Доставка (Yandex Go)")
		}
		if strings.Contains(normalized, "courier") {
			return t(lang, "Dostavka (Yandex Go)", "Доставка (Yandex Go)")
		}
		return raw
	}
}
