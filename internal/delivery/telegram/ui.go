package telegram

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *BotHandler) sendLanguageSelector(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Tilni tanlang / Выберите язык")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇺🇿 O'zbek", "lang_uz"),
			tgbotapi.NewInlineKeyboardButtonData("🇷🇺 Русский", "lang_ru"),
		),
	)
	if sent, err := h.sendAndLog(msg); err == nil {
		h.trackWelcomeMessage(chatID, sent.MessageID)
	}
}

func (h *BotHandler) sendConfigCTA(chatID int64, lang string) {
	text := t(lang, "⚙️ PC yig'ishda yordam berish uchun pastdagi tugmani bosing va bosqichma-bosqich ma'lumot kiriting.", "⚙️ Для подбора сборки нажмите кнопку ниже и пройдите шаги.")
	btn := tgbotapi.NewInlineKeyboardButtonData(t(lang, "🚀 Konfiguratsiya", "🚀 Конфигурация"), "config_start")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btn),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigCTA(chatID, sent.MessageID)
	}
}

func (h *BotHandler) sendConfigRetryPrompt(chatID int64, lang string) {
	text := t(lang, "😔 Uzr, bu konfiguratsiya yoqmadi. Yana bir bor harakat qilib ko'ramizmi? Pastdagi tugmani bosib qayta boshlang.", "😔 Сожалею, конфигурация не понравилась. Попробуем еще раз? Нажмите кнопку ниже, чтобы начать заново.")
	btn := tgbotapi.NewInlineKeyboardButtonData(t(lang, "🚀 Konfiguratsiya", "🚀 Конфигурация"), "config_start")
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(btn),
	)
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	if sent, err := h.sendAndLog(msg); err == nil {
		h.setConfigCTA(chatID, sent.MessageID)
	}
}

func (h *BotHandler) getWelcomeMessage(lang, name string) string {
	trimmedName := strings.TrimSpace(name)
	greeting := "👋 Salom"
	if lang == "ru" {
		greeting = "👋 Привет"
	}
	if trimmedName != "" {
		greeting = fmt.Sprintf("%s, %s", greeting, trimmedName)
	}
	greeting += "!"
	if lang == "ru" {
		return fmt.Sprintf("%s Я Ingamer — твой AI-помощник по компьютерной технике. Пиши, чем могу помочь.", greeting)
	}

	return fmt.Sprintf("%s Men Ingamer — kompyuter texnikasi bo'yicha AI yordamchingizman. Savollaringiz bo'lsa yozing.", greeting)
}

func (h *BotHandler) getHelpMessage() string {
	return `🤖 *Bot yordam menyusi*

📋 *Mavjud komandalar:*
/start - Botni qayta boshlash
/help - Yordam menyusini ko'rish
/clear - Chat tarixini tozalash
/history - Chat tarixini ko'rish
/configuratsiya - PC yig'ish uchun bosqichma-bosqich sozlash

🔐 Admin:
/admin - Admin panelga kirish
/logout - Admin paneldan chiqish
/catalog - Katalog haqida ma'lumot (admin)
/products - Barcha mahsulotlar
/not - Eslatmalarni sozlash (on/off/interval/matn, admin)

*Qanday foydalanish:*
Menga oddiy xabar yuboring va men sizga javob beraman. Masalan:
• "Gaming uchun kompyuter tavsiya qiling"
• "RTX 4070 haqida ma'lumot bering"
• "16GB RAM yetadimi?"

Men sizning savollaringizni saqlayman, shuning uchun kontekstni eslab qolaman! 💡`
}
