package telegram

import (
	"context"
	"fmt"
	"strings"
)

func (h *BotHandler) handleVariantSelection(ctx context.Context, userID int64, chatID int64, idx int) bool {
	items, ok := h.getLastNumberedOptions(ctx, userID)
	if !ok {
		return false
	}
	if idx < 1 || idx > len(items) {
		h.sendMessage(chatID, fmt.Sprintf("Faqat 1-%d variant mavjud. Raqamini yozing.", len(items)))
		return true
	}
	item := items[idx-1]
	reply := formatSelectedProductReply(item, h.getUserLang(userID))
	if strings.TrimSpace(reply) == "" {
		return false
	}
	h.sendMessage(chatID, reply)
	h.setLastSuggestion(userID, reply)
	h.sendPurchaseConfirmationButtons(chatID, userID, reply, "")
	return true
}

func (h *BotHandler) promptVariantSelection(ctx context.Context, userID int64, chatID int64) bool {
	items, ok := h.getLastNumberedOptions(ctx, userID)
	if !ok {
		return false
	}
	h.sendMessage(chatID, fmt.Sprintf("Qaysi birini tanlaysiz? Raqamini yozing (1-%d).", len(items)))
	return true
}

func (h *BotHandler) getLastNumberedOptions(ctx context.Context, userID int64) ([]string, bool) {
	history, err := h.chatUseCase.GetHistory(ctx, userID)
	if err != nil {
		return nil, false
	}
	for i := len(history) - 1; i >= 0; i-- {
		items := extractNumberedItems(history[i].Response)
		if len(items) > 0 {
			return items, true
		}
	}
	return nil, false
}

func formatSelectedProductReply(item, lang string) string {
	item = strings.TrimSpace(item)
	if item == "" {
		return ""
	}
	price := bestPriceFromLine(item)
	if price != "" {
		return t(lang,
			fmt.Sprintf("✅ Bizda bor: %s\nJami: %s", item, price),
			fmt.Sprintf("✅ В наличии: %s\nИтого: %s", item, price),
		)
	}
	return t(lang,
		fmt.Sprintf("✅ Bizda bor: %s", item),
		fmt.Sprintf("✅ В наличии: %s", item),
	)
}
