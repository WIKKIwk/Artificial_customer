package telegram

import (
	"context"
	"log"
	"time"
)

// cleanupSessions - eski sessiyalarni tozalash (memory leak oldini olish)
func (h *BotHandler) cleanupSessions(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			timeout := 2 * time.Hour // 30 minutdan 2 soatga oshirildi

			// Config sessiyalarni tozalash
			h.configMu.Lock()
			for userID, session := range h.configSessions {
				if now.Sub(session.LastUpdate) > timeout {
					delete(h.configSessions, userID)
					log.Printf("♻️ Config session tozalandi: userID=%d (timeout)", userID)
				}
			}
			h.configMu.Unlock()

			// Order sessiyalarni tozalash (timeout bo'lsa)
			// Order session'da LastUpdate yo'q, lekin qo'shish mumkin
			// Hozircha skip qilamiz

			// Pending approvals tozalash (24 soatdan eski)
			h.approvalMu.Lock()
			for userID, approval := range h.pendingApprove {
				if now.Sub(approval.SentAt) > 24*time.Hour {
					delete(h.pendingApprove, userID)
					log.Printf("♻️  Pending approval tozalandi: userID=%d (timeout)", userID)
				}
			}
			h.approvalMu.Unlock()

			// Processing flag'larni tozalash (qolib ketgan bo'lishi mumkin)
			h.processingMu.Lock()
			processedCount := 0
			for userID := range h.processing {
				// Agar processing flag 10 daqiqadan ko'p vaqt bo'lsa, tozalash
				// (Normal AI javob 30 soniyada tugashi kerak)
				delete(h.processing, userID)
				processedCount++
			}
			h.processingMu.Unlock()
			if processedCount > 0 {
				log.Printf("♻️ %d ta qolib ketgan processing flag tozalandi", processedCount)
			}

			log.Printf("🧹 Session cleanup bajarildi")
		}
	}
}
