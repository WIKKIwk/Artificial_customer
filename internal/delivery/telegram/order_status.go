package telegram

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

// Order status helpers
func (h *BotHandler) saveOrderStatus(orderID string, info orderStatusInfo) {
	if info.OrderID == "" {
		info.OrderID = orderID
	}
	h.orderStatusMu.Lock()
	h.orderStatuses[orderID] = info
	h.orderStatusMu.Unlock()
	_ = h.orderStore.Save(context.Background(), info)
	h.scheduleAboutUserSheetSync("order")
}

func (h *BotHandler) getOrderStatus(orderID string) (orderStatusInfo, bool) {
	h.orderStatusMu.RLock()
	info, ok := h.orderStatuses[orderID]
	h.orderStatusMu.RUnlock()
	if ok {
		return info, true
	}
	if h.orderStore != nil {
		if stored, found, _ := h.orderStore.Get(context.Background(), orderID); found {
			return stored, true
		}
	}
	return info, false
}

func (h *BotHandler) setOrderStatus(orderID, status string) {
	h.orderStatusMu.Lock()
	info := h.orderStatuses[orderID]
	info.Status = status
	h.orderStatuses[orderID] = info
	h.orderStatusMu.Unlock()
	_ = h.orderStore.UpdateStatus(context.Background(), orderID, status)
}

func (h *BotHandler) findOrderByID(orderID string) *orderStatusInfo {
	info, ok := h.getOrderStatus(orderID)
	if ok {
		return &info
	}
	return nil
}

func (h *BotHandler) updateOrderStatus(orderID, status string) {
	h.setOrderStatus(orderID, status)
}

func (h *BotHandler) clearOrdersForUser(userID int64) error {
	if h.orderStore != nil {
		return h.orderStore.DeleteByUser(context.Background(), userID)
	}
	return nil
}

func (h *BotHandler) listOrdersByUser(userID int64) []orderStatusInfo {
	if h.orderStore != nil {
		if res, err := h.orderStore.ListByUser(context.Background(), userID); err == nil {
			return res
		}
	}
	h.orderStatusMu.RLock()
	defer h.orderStatusMu.RUnlock()
	var res []orderStatusInfo
	for _, info := range h.orderStatuses {
		if info.UserID == userID {
			res = append(res, info)
		}
	}
	return res
}

func (h *BotHandler) listRecentOrders(limit int) []orderStatusInfo {
	if h.orderStore != nil {
		if res, err := h.orderStore.ListRecent(context.Background(), limit); err == nil {
			return res
		}
	}
	h.orderStatusMu.RLock()
	defer h.orderStatusMu.RUnlock()
	var all []orderStatusInfo
	for _, ord := range h.orderStatuses {
		all = append(all, ord)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all
}

func statusLabel(status, lang string) string {
	switch status {
	case "processing":
		return t(lang, "Qabul qilindi", "Принят")
	case "ready_delivery":
		return t(lang, "Tayyor (dostavka)", "Готов (доставка)")
	case "ready_pickup":
		return t(lang, "Tayyor (pickup)", "Готов (самовывоз)")
	case "onway":
		return t(lang, "Yo'lga chiqdi", "В пути")
	case "delivered":
		return t(lang, "Yakunlandi", "Доставлен")
	case "canceled":
		return t(lang, "Bekor qilindi", "Отменён")
	default:
		return t(lang, "Jarayonda", "В обработке")
	}
}

func formatOrderStatusSummary(summary string) string {
	s := strings.TrimSpace(summary)
	if s == "" {
		return ""
	}
	// Oddiy tozalash: qator boshidagi bullet/prefixlarni olib tashlaymiz
	cleanLines := []string{}
	for _, ln := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		trim = strings.TrimLeft(trim, "-•* ")
		cleanLines = append(cleanLines, trim)
	}
	return strings.Join(cleanLines, "\n")
}

func orderTitle(ord orderStatusInfo) string {
	if name := configNameFromText(ord.Summary); name != "" {
		return name
	}
	if strings.TrimSpace(ord.OrderID) != "" {
		return ord.OrderID
	}
	return "Konfiguratsiya"
}

func configNameFromText(text string) string {
	for _, ln := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(ln)
		if trim == "" {
			continue
		}
		lower := strings.ToLower(trim)
		if strings.HasPrefix(lower, "konfiguratsiya nomi") || strings.HasPrefix(lower, "имя конфигурации") {
			parts := strings.SplitN(trim, ":", 2)
			if len(parts) == 2 {
				if name := strings.TrimSpace(parts[1]); name != "" {
					return name
				}
			}
		}
		if strings.Contains(lower, "nomli") {
			if m := regexp.MustCompile(`"([^"]{2,50})"`).FindStringSubmatch(trim); len(m) == 2 {
				return strings.TrimSpace(m[1])
			}
		}
	}
	return ""
}
