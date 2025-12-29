package telegram

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestStressMultipleConcurrentUsers - Multiple users parallel testlar
func TestStressMultipleConcurrentUsers(t *testing.T) {
	handler := &BotHandler{
		configSessions:   make(map[int64]*configSession),
		feedbacks:        make(map[int64]feedbackInfo),
		configReminded:   make(map[int64]bool),
		configReminder:   make(map[int64]*time.Timer),
		reminderInput:    make(map[int64]*reminderInputState),
		pendingChange:    make(map[int64]changeRequest),
		orderSessions:    make(map[int64]*orderSession),
		orderCleanup:     make(map[int64]orderFormCleanup),
		awaitingPassword: make(map[int64]bool),
		chatStore:        newMemoryChatStore(),
		cache:            newResponseCache(defaultCacheTTL, defaultMaxCacheSize),
	}

	var wg sync.WaitGroup
	numUsers := 50  // 50 ta concurrent user
	iterations := 5 // har bir user 5 marta so'rov yuboradi

	var totalRequests int64
	var successCount int64
	var errorCount int64

	startTime := time.Now()

	t.Logf("🚀 Stress test boshlandi: %d users, har biri %d ta so'rov", numUsers, iterations)

	for userID := 0; userID < numUsers; userID++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()

			for i := 0; i < iterations; i++ {
				atomic.AddInt64(&totalRequests, 1)

				// Har xil turdagi so'rovlar
				switch i % 5 {
				case 0:
					// Config session yaratish
					handler.startConfigSession(id)
					if handler.hasConfigSession(id) {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
						t.Errorf("User %d: session yaratilmadi", id)
					}

				case 1:
					// Session mavjudligini tekshirish
					if handler.hasConfigSession(id) {
						atomic.AddInt64(&successCount, 1)
					}

				case 2:
					// Config reminded tekshirish va belgilash
					handler.markConfigReminded(id)
					if handler.wasConfigReminded(id) {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}

				case 3:
					// Cache operatsiyalari
					key := handler.getCacheKey(id, fmt.Sprintf("test message %d", i))
					handler.cacheResponse(key, fmt.Sprintf("response for user %d iteration %d", id, i))
					if _, ok := handler.getCachedResponse(key); ok {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}

				case 4:
					// Session o'chirish
					handler.configMu.Lock()
					delete(handler.configSessions, id)
					handler.configMu.Unlock()
					atomic.AddInt64(&successCount, 1)
				}

				// Real scenario'ni simulatsiya qilish uchun biroz kutish
				time.Sleep(time.Millisecond * time.Duration(10+i*2))
			}
		}(int64(userID))
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	// Natijalarni ko'rsatish
	t.Logf("\n"+`
╔══════════════════════════════════════════════════════╗
║          STRESS TEST NATIJALARI                     ║
╠══════════════════════════════════════════════════════╣
║ 👥 Foydalanuvchilar:     %5d                       ║
║ 📊 Jami so'rovlar:        %5d                       ║
║ ✅ Muvaffaqiyatli:        %5d                       ║
║ ❌ Xatolar:               %5d                       ║
║ ⏱️  Jami vaqt:            %v                    ║
║ 🚀 Throughput:            %5.2f req/s              ║
║ 📈 O'rtacha javob:        %v                    ║
╚══════════════════════════════════════════════════════╝
	`,
		numUsers,
		totalRequests,
		successCount,
		errorCount,
		elapsed,
		float64(totalRequests)/elapsed.Seconds(),
		elapsed/time.Duration(totalRequests),
	)

	// Cache statistikasini ko'rsatish
	hits, misses, cacheSize := handler.cache.stats()
	t.Logf("\n📦 Cache Statistics:")
	t.Logf("   Hits:   %d", hits)
	t.Logf("   Misses: %d", misses)
	t.Logf("   Size:   %d entries", cacheSize)
	if hits+misses > 0 {
		hitRate := float64(hits) / float64(hits+misses) * 100
		t.Logf("   Hit rate: %.2f%%", hitRate)
	}

	// Xatolar bo'lsa test fail qilish
	if errorCount > 0 {
		t.Errorf("❌ Stress test failed: %d xato topildi", errorCount)
	} else {
		t.Log("✅ Barcha testlar muvaffaqiyatli o'tdi!")
	}
}

// TestStressRateLimiting - Rate limiting ishlashini tekshirish
func TestStressRateLimiting(t *testing.T) {
	handler := &BotHandler{
		cache: newResponseCache(defaultCacheTTL, defaultMaxCacheSize),
	}

	handler.workerPool = newWorkerPool(handler, defaultWorkerCount)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler.workerPool.start(ctx)
	defer handler.workerPool.shutdown()

	userID := int64(12345)

	// Bir vaqtda 10 ta so'rov yuborish (rate limit 3/s)
	var accepted int64
	var rejected int64

	for i := 0; i < 10; i++ {
		if handler.workerPool.checkRateLimit(userID) {
			atomic.AddInt64(&accepted, 1)
		} else {
			atomic.AddInt64(&rejected, 1)
		}
	}

	t.Logf("\n📊 Rate Limiting Test:")
	t.Logf("   Qabul qilingan: %d", accepted)
	t.Logf("   Rad etilgan:    %d", rejected)

	// 3 dan ortiq qabul qilinmasligi kerak
	if accepted > maxRequestsPerSecond {
		t.Errorf("Rate limiting ishlamayapti: %d > %d", accepted, maxRequestsPerSecond)
	}

	// 1 soniya kutgandan keyin yana so'rov yuborish mumkin
	time.Sleep(1100 * time.Millisecond)

	if handler.workerPool.checkRateLimit(userID) {
		t.Log("✅ Rate limit reset ishlayapti")
	} else {
		t.Error("❌ Rate limit reset ishlamayapti")
	}
}

// TestStressCacheConcurrency - Cache concurrent access testi
func TestStressCacheConcurrency(t *testing.T) {
	cache := newResponseCache(defaultCacheTTL, 100)

	var wg sync.WaitGroup
	numGoroutines := 100

	t.Logf("🔄 Cache concurrency test: %d parallel goroutine", numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			key := fmt.Sprintf("key_%d", id)
			value := fmt.Sprintf("value_%d", id)

			// Yozish
			cache.set(key, value)

			// O'qish
			if got, ok := cache.get(key); !ok || got != value {
				t.Errorf("Cache consistency xatosi: kutilgan=%s, olindi=%s, ok=%v", value, got, ok)
			}
		}(i)
	}

	wg.Wait()

	hits, misses, size := cache.stats()
	t.Logf("✅ Cache test tugadi:")
	t.Logf("   Hits:   %d", hits)
	t.Logf("   Misses: %d", misses)
	t.Logf("   Size:   %d", size)
}

// TestStressWorkerPoolQueue - Worker pool navbat testi
func TestStressWorkerPoolQueue(t *testing.T) {
	handler := &BotHandler{
		cache:          newResponseCache(defaultCacheTTL, defaultMaxCacheSize),
		configSessions: make(map[int64]*configSession),
		feedbacks:      make(map[int64]feedbackInfo),
	}

	handler.workerPool = newWorkerPool(handler, 5) // 5 ta worker
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handler.workerPool.start(ctx)
	defer handler.workerPool.shutdown()

	// Queue hajmidan ortiq so'rov yuborish
	numRequests := requestQueueSize + 50

	var submitted int64
	var rejected int64

	t.Logf("📬 Queue test: %d so'rov yuborish", numRequests)

	for i := 0; i < numRequests; i++ {
		req := &messageRequest{
			ctx:      ctx,
			userID:   int64(i),
			username: fmt.Sprintf("user_%d", i),
			text:     fmt.Sprintf("test message %d", i),
			chatID:   int64(i),
		}

		// Non-blocking submit
		select {
		case handler.workerPool.requestQueue <- req:
			atomic.AddInt64(&submitted, 1)
		default:
			atomic.AddInt64(&rejected, 1)
		}
	}

	t.Logf("✅ Queue test tugadi:")
	t.Logf("   Qabul qilingan: %d", submitted)
	t.Logf("   Rad etilgan:    %d", rejected)

	// Barcha ishlarni tugashini kutish
	time.Sleep(2 * time.Second)
}

// TestStressMemoryUsage - Xotira ishlatishni monitoring qilish
func TestStressMemoryUsage(t *testing.T) {
	handler := &BotHandler{
		configSessions: make(map[int64]*configSession),
		feedbacks:      make(map[int64]feedbackInfo),
		feedbackByID:   make(map[string]feedbackInfo),
		feedbackLatest: make(map[int64]string),
		configReminded: make(map[int64]bool),
		cache:          newResponseCache(defaultCacheTTL, defaultMaxCacheSize),
	}

	handler.workerPool = newWorkerPool(handler, defaultWorkerCount)

	// 1000 ta session yaratish
	for i := 0; i < 1000; i++ {
		handler.startConfigSession(int64(i))

		// Har xil ma'lumotlar qo'shish
		handler.saveFeedback(int64(i), feedbackInfo{
			Summary:    fmt.Sprintf("Summary %d", i),
			ConfigText: fmt.Sprintf("Config %d", i),
			Username:   fmt.Sprintf("User %d", i),
			ChatID:     int64(i),
		})

		key := handler.getCacheKey(int64(i), fmt.Sprintf("message %d", i))
		handler.cacheResponse(key, fmt.Sprintf("response %d", i))
	}

	// Map hajmlarini tekshirish
	handler.configMu.RLock()
	sessionCount := len(handler.configSessions)
	handler.configMu.RUnlock()

	handler.feedbackMu.RLock()
	feedbackCount := len(handler.feedbacks)
	handler.feedbackMu.RUnlock()

	_, _, cacheSize := handler.cache.stats()

	t.Logf("\n💾 Memory Usage Test:")
	t.Logf("   Config sessions: %d", sessionCount)
	t.Logf("   Feedbacks:       %d", feedbackCount)
	t.Logf("   Cache entries:   %d", cacheSize)

	// Cleanup
	handler.configMu.Lock()
	for k := range handler.configSessions {
		delete(handler.configSessions, k)
	}
	handler.configMu.Unlock()

	handler.feedbackMu.Lock()
	for k := range handler.feedbacks {
		delete(handler.feedbacks, k)
	}
	handler.feedbackMu.Unlock()

	handler.cache.clear()

	handler.configMu.RLock()
	sessionCountAfter := len(handler.configSessions)
	handler.configMu.RUnlock()

	if sessionCountAfter != 0 {
		t.Errorf("Cleanup ishlamadi: %d sessiya qoldi", sessionCountAfter)
	} else {
		t.Log("✅ Cleanup muvaffaqiyatli")
	}
}
