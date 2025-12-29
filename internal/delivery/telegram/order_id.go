package telegram

import (
	"fmt"
	"time"
)

// generateOrderID - sana + kun bo'yicha inkrement (format: DDMMYYYY-XX)
func (h *BotHandler) generateOrderID() string {
	today := time.Now().Format("02012006")
	h.orderCounterMu.Lock()
	defer h.orderCounterMu.Unlock()
	h.orderCounter[today]++
	return fmt.Sprintf("%s-%02d", today, h.orderCounter[today])
}
