package telegram

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// messageRequest represents a message to be processed
type messageRequest struct {
	ctx      context.Context
	userID   int64
	username string
	text     string
	chatID   int64
	message  *tgbotapi.Message
	lang     string
}

// workerPool manages parallel processing of messages
type workerPool struct {
	requestQueue chan *messageRequest
	workerCount  int
	handler      *BotHandler
	wg           sync.WaitGroup

	// Rate limiting per user
	rateLimiter   map[int64]*userRateLimit
	rateLimiterMu sync.RWMutex
}

// userRateLimit tracks rate limiting per user
type userRateLimit struct {
	lastRequest  time.Time
	requestCount int
	mu           sync.Mutex
}

const (
	maxRequestsPerSecond   = 3
	requestQueueSize       = 100
	defaultWorkerCount     = 30
	aiRequestTimeout       = 45 * time.Second // Reduced from 100s for better UX
	defaultOrderLock       = 24 * time.Hour
	rateLimiterCleanupTime = 5 * time.Minute  // How often to clean up rate limiters
	rateLimiterMaxIdleTime = 10 * time.Minute // Max idle time before removing rate limiter
	maxRateLimitersInCache = 10000            // Max number of rate limiters to keep in memory
)

// newWorkerPool creates a new worker pool
func newWorkerPool(handler *BotHandler, workerCount int) *workerPool {
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
	}

	wp := &workerPool{
		requestQueue: make(chan *messageRequest, requestQueueSize),
		workerCount:  workerCount,
		handler:      handler,
		rateLimiter:  make(map[int64]*userRateLimit),
	}

	return wp
}

// start starts all workers
func (wp *workerPool) start(ctx context.Context) {
	log.Printf("Starting %d workers for parallel message processing", wp.workerCount)

	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}

	// Cleanup old rate limit entries periodically
	go wp.cleanupRateLimits(ctx)
}

// worker processes messages from the queue
func (wp *workerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d shutting down", id)
			return
		case req, ok := <-wp.requestQueue:
			if !ok {
				log.Printf("Worker %d shutting down (queue closed)", id)
				return
			}
			if req == nil {
				continue
			}

			// Check rate limit
			if !wp.checkRateLimit(req.userID) {
				wp.handler.sendMessage(req.chatID, "⚠️ Juda ko'p so'rov. Iltimos, biroz kutib turing.")
				wp.handler.clearWaitingMessage(req.userID)
				wp.handler.endProcessing(req.userID)
				continue
			}

			// Process with timeout
			wp.processMessageWithTimeout(req)
		}
	}
}

// processMessageWithTimeout processes a message with context timeout
func (wp *workerPool) processMessageWithTimeout(req *messageRequest) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(req.ctx, aiRequestTimeout)
	defer cancel()

	if wp.handler == nil {
		log.Printf("worker pool: handler is nil, skipping request user=%d", req.userID)
		return
	}

	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in message processing for user %d: %v", req.userID, r)
			wp.handler.sendMessage(req.chatID, "⚠️ Ichki xatolik yuz berdi. Iltimos, qayta urinib ko'ring.")
		}
	}()

	defer wp.handler.clearWaitingMessage(req.userID)
	defer wp.handler.endProcessing(req.userID)
	defer wp.handler.resetProcessingWarn(req.userID)

	// Defensive: tests may create handler without dependencies.
	if wp.handler.chatUseCase == nil {
		log.Printf("worker pool: chatUseCase is nil, skipping request user=%d", req.userID)
		return
	}

	// Process message with AI - minimal prompt, main logic in chat_usecase
	prompt := req.text
	if req.lang == "ru" {
		prompt = "Отвечай только на русском языке.\n" + prompt
	}

	// Show typing indicator before AI request
	if wp.handler.bot != nil {
		typingAction := tgbotapi.NewChatAction(req.chatID, tgbotapi.ChatTyping)
		wp.handler.bot.Send(typingAction)
	}

	response, err := wp.handler.chatUseCase.ProcessMessage(ctx, req.userID, req.username, prompt)
	if err != nil {
		// Check context errors first
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("AI request timeout for user %d after %v", req.userID, aiRequestTimeout)
			wp.handler.sendMessage(req.chatID, "⏱️ So'rovni qayta ishlash vaqti tugadi. Iltimos, qaytadan urinib ko'ring yoki soddaroq so'rov yuboring.")
		} else if ctx.Err() == context.Canceled {
			log.Printf("AI request canceled for user %d", req.userID)
			wp.handler.sendMessage(req.chatID, "⚠️ So'rov bekor qilindi.")
		} else {
			log.Printf("AI request error for user %d: %v", req.userID, err)
			wp.handler.sendMessage(req.chatID, "Kechirasiz, xatolik yuz berdi. Iltimos, qayta urinib ko'ring.")
		}
		return
	}

	// AI o'zi to'g'ri formatda javob beradi - hech narsa o'zgartirmaymiz
	response = wp.handler.applyCurrencyPreference(response)
	wp.handler.sendMessage(req.chatID, response)

	// Order tugmalarini ko'rsatish SHARTLARI:
	// - bitta mahsulotga o'xshash javob (narx bor, ko'p variant yo'q)
	// - "Jami:" bo'lmasa ham ruxsat (bitta mahsulot bo'lsa)
	variantCount := countNumberedVariants(response)
	isSingleProduct := isSingleProductSuggestion(response)

	// DEBUG LOG - nima bo'layotganini ko'rish uchun
	log.Printf("🔍 [Order Button Check] UserID=%d, hasTotal=%v, variantCount=%d, isMultipleVariants=%v, isSingleProduct=%v",
		req.userID, hasTotalLine(response), variantCount, variantCount >= 2, isSingleProduct)

	if isSingleProduct {
		log.Printf("✅ [Showing Order Buttons] UserID=%d", req.userID)
		wp.handler.setLastSuggestion(req.userID, response)
		wp.handler.sendPurchaseConfirmationButtons(req.chatID, req.userID, response, "")
	} else if hasTotalLine(response) {
		log.Printf("❌ [NOT Showing Buttons] UserID=%d, Reason: isMultipleVariants=%v (variantCount=%d)", req.userID, variantCount >= 2, variantCount)
	}
}

// hasPriceInfoButTooManyOptions - agar bitta javobda ko'p variant bulletlari bo'lsa, sotib olish tugmalarini ko'rsatmaymiz
func hasPriceInfoButTooManyOptions(text string) bool {
	lower := strings.ToLower(text)
	if !hasPriceInfo(text) {
		return false
	}
	// Bulletlarni sanaymiz: agar 2 tadan ko'p bo'lsa, bu ro'yxat deb qabul qilamiz
	bullets := strings.Count(lower, "\n-") + strings.Count(lower, "\n*") + strings.Count(lower, "\n•")
	// 1-2 bullet (masalan, tavsif + 1 ta model) uchun tugma ko'rsatamiz, ko'proq bo'lsa ko'rsatmaymiz
	return bullets >= 3
}

// hasPriceInfo matnda narxga oid belgi bormi degan soddalashtirilgan tekshiruv
func hasPriceInfo(text string) bool {
	lower := strings.ToLower(text)
	priceIndicators := []string{"$", "so'm", "soum", "sum", "usd", "narx", "jami"}
	for _, p := range priceIndicators {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// checkRateLimit checks if user is within rate limit
func (wp *workerPool) checkRateLimit(userID int64) bool {
	wp.rateLimiterMu.Lock()
	defer wp.rateLimiterMu.Unlock()

	limiter, exists := wp.rateLimiter[userID]
	if !exists {
		limiter = &userRateLimit{
			lastRequest:  time.Now(),
			requestCount: 1,
		}
		wp.rateLimiter[userID] = limiter
		return true
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(limiter.lastRequest)

	// Reset counter if more than 1 second has passed
	if elapsed >= time.Second {
		limiter.requestCount = 1
		limiter.lastRequest = now
		return true
	}

	// Check if within limit
	if limiter.requestCount >= maxRequestsPerSecond {
		log.Printf("Rate limit exceeded for user %d", userID)
		return false
	}

	limiter.requestCount++
	return true
}

// cleanupRateLimits removes old rate limit entries
func (wp *workerPool) cleanupRateLimits(ctx context.Context) {
	ticker := time.NewTicker(rateLimiterCleanupTime)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			toDelete := []int64{}

			// First pass: collect users to delete (without holding both locks)
			wp.rateLimiterMu.RLock()
			cacheSize := len(wp.rateLimiter)
			for userID, limiter := range wp.rateLimiter {
				limiter.mu.Lock()
				if now.Sub(limiter.lastRequest) > rateLimiterMaxIdleTime {
					toDelete = append(toDelete, userID)
				}
				limiter.mu.Unlock()
			}
			wp.rateLimiterMu.RUnlock()

			// Second pass: delete collected users
			if len(toDelete) > 0 {
				wp.rateLimiterMu.Lock()
				for _, userID := range toDelete {
					delete(wp.rateLimiter, userID)
				}
				wp.rateLimiterMu.Unlock()
				log.Printf("Cleaned up %d inactive rate limiters (total: %d -> %d)", len(toDelete), cacheSize, cacheSize-len(toDelete))
			}

			// If cache is still too large, remove oldest entries
			if cacheSize > maxRateLimitersInCache {
				wp.evictOldestRateLimiters(cacheSize - maxRateLimitersInCache)
			}
		}
	}
}

// evictOldestRateLimiters removes oldest rate limiters when cache is full
func (wp *workerPool) evictOldestRateLimiters(count int) {
	type userTime struct {
		userID      int64
		lastRequest time.Time
	}

	wp.rateLimiterMu.RLock()
	users := make([]userTime, 0, len(wp.rateLimiter))
	for userID, limiter := range wp.rateLimiter {
		limiter.mu.Lock()
		users = append(users, userTime{userID: userID, lastRequest: limiter.lastRequest})
		limiter.mu.Unlock()
	}
	wp.rateLimiterMu.RUnlock()

	// Sort by last request time (oldest first)
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			if users[i].lastRequest.After(users[j].lastRequest) {
				users[i], users[j] = users[j], users[i]
			}
		}
	}

	// Delete oldest entries
	wp.rateLimiterMu.Lock()
	deleted := 0
	for i := 0; i < len(users) && deleted < count; i++ {
		delete(wp.rateLimiter, users[i].userID)
		deleted++
	}
	wp.rateLimiterMu.Unlock()

	if deleted > 0 {
		log.Printf("Evicted %d oldest rate limiters to prevent memory leak", deleted)
	}
}

// submit submits a message to the worker pool
func (wp *workerPool) submit(req *messageRequest) bool {
	select {
	case wp.requestQueue <- req:
		return true
	default:
		// Queue is full
		log.Printf("Worker pool queue is full (%d/%d), rejecting request from user %d", len(wp.requestQueue), requestQueueSize, req.userID)
		wp.handler.sendMessage(req.chatID, "⚠️ Bot juda band. Iltimos, bir oz kutib turing.")
		wp.handler.endProcessing(req.userID)
		return false
	}
}

// shutdown gracefully shuts down the worker pool
func (wp *workerPool) shutdown() {
	log.Printf("Shutting down worker pool, %d messages in queue", len(wp.requestQueue))
	close(wp.requestQueue)
	wp.wg.Wait()
	log.Println("Worker pool shut down successfully")
}
