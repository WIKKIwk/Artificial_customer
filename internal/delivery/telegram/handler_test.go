package telegram

import (
	"sync"
	"testing"
	"time"
)

// TestConfigSessionConcurrency - parallel sessiyalar uchun race condition tekshirish
func TestConfigSessionConcurrency(t *testing.T) {
	handler := &BotHandler{
		configSessions: make(map[int64]*configSession),
	}

	// 100 ta goroutine parallel ishga tushirish
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(userID int64) {
			defer wg.Done()

			// Session yaratish
			handler.startConfigSession(userID)

			// Session mavjudligini tekshirish
			if !handler.hasConfigSession(userID) {
				t.Errorf("Session topilmadi: userID=%d", userID)
			}

			// Bir oz kutish (real scenariyani simulyatsiya qilish)
			time.Sleep(10 * time.Millisecond)

		}(int64(i))
	}

	wg.Wait()

	// Barcha sessiyalar yaratilganligini tekshirish
	handler.configMu.RLock()
	count := len(handler.configSessions)
	handler.configMu.RUnlock()

	if count != numGoroutines {
		t.Errorf("Kutilgan %d sessiya, lekin %d topildi", numGoroutines, count)
	}
}

// TestSessionTimeout - timeout mexanizmini tekshirish
func TestSessionTimeout(t *testing.T) {
	handler := &BotHandler{
		configSessions: make(map[int64]*configSession),
	}

	// Eski sessiya yaratish
	oldSession := &configSession{
		Stage:      configStageNeedType,
		StartedAt:  time.Now().Add(-2 * time.Hour), // 2 soat oldin
		LastUpdate: time.Now().Add(-2 * time.Hour),
	}

	// Yangi sessiya yaratish
	newSession := &configSession{
		Stage:      configStageNeedType,
		StartedAt:  time.Now(),
		LastUpdate: time.Now(),
	}

	handler.configMu.Lock()
	handler.configSessions[1] = oldSession
	handler.configSessions[2] = newSession
	handler.configMu.Unlock()

	// Cleanup simulyatsiya qilish
	timeout := 30 * time.Minute
	now := time.Now()

	handler.configMu.Lock()
	for userID, session := range handler.configSessions {
		if now.Sub(session.LastUpdate) > timeout {
			delete(handler.configSessions, userID)
		}
	}
	handler.configMu.Unlock()

	// Tekshirish
	handler.configMu.RLock()
	_, oldExists := handler.configSessions[1]
	_, newExists := handler.configSessions[2]
	handler.configMu.RUnlock()

	if oldExists {
		t.Error("Eski sessiya o'chirilmagan!")
	}

	if !newExists {
		t.Error("Yangi sessiya xato o'chirilgan!")
	}
}

// TestValidationFunctions - validatsiya funksiyalarini test qilish
func TestPhoneValidation(t *testing.T) {
	tests := []struct {
		phone    string
		expected bool
	}{
		{"+998901234567", true},
		{"998901234567", true},
		{"901234567", true},
		{"1234567", true},
		{"+99890 123 45 67", true}, // Bo'shliq bilan
		{"abc123", false},
		{"123", false},
		{"", false},
		{"+998", false},
	}

	for _, test := range tests {
		result := validatePhoneNumber(test.phone)
		if result != test.expected {
			t.Errorf("Phone=%s: kutilgan=%v, natija=%v", test.phone, test.expected, result)
		}
	}
}

func TestNameValidation(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"John Doe", true},
		{"Алишер Навоий", true},
		{"O'zbek", true},
		{"John.", false},
		{"John, Doe", false},
		{"Jasur123", false},
		{"A", false},
		{"123", false},
		{"", false},
		{"Valid Name", true},
	}

	for _, test := range tests {
		result := validateName(test.name)
		if result != test.expected {
			t.Errorf("Name=%s: kutilgan=%v, natija=%v", test.name, test.expected, result)
		}
	}
}

// TestIsConfigRequest - false positive tekshirish
func TestIsConfigRequest(t *testing.T) {
	tests := []struct {
		text     string
		expected bool
	}{
		{"gaming pc kerak", true},
		{"1000$ lik kompyuter yig'ib ber", true},
		{"pc sborka", true},
		{"monitor bormi?", false},
		{"pc qancha?", false},
		{"100$ monitor", false},
		{"salom", false},
	}

	for _, test := range tests {
		result := isConfigRequest(test.text)
		if result != test.expected {
			t.Errorf("Text=%s: kutilgan=%v, natija=%v", test.text, test.expected, result)
		}
	}
}

// BenchmarkConfigSessionCreation - session yaratish tezligini o'lchash
func BenchmarkConfigSessionCreation(b *testing.B) {
	handler := &BotHandler{
		configSessions: make(map[int64]*configSession),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.startConfigSession(int64(i))
	}
}

// TestMemoryLeak - xotira leak tekshirish (oddiy test)
func TestMemoryLeak(t *testing.T) {
	handler := &BotHandler{
		configSessions: make(map[int64]*configSession),
		feedbacks:      make(map[int64]feedbackInfo),
	}

	// 1000 ta sessiya yaratish
	for i := 0; i < 1000; i++ {
		handler.startConfigSession(int64(i))
	}

	// Cleanup qilish
	timeout := 0 * time.Minute // Hamma sessiyalarni o'chirish
	now := time.Now()

	handler.configMu.Lock()
	for userID, session := range handler.configSessions {
		if now.Sub(session.LastUpdate) > timeout {
			delete(handler.configSessions, userID)
		}
	}
	handler.configMu.Unlock()

	// Map bo'sh bo'lishi kerak
	handler.configMu.RLock()
	count := len(handler.configSessions)
	handler.configMu.RUnlock()

	if count != 0 {
		t.Errorf("Memory leak: %d sessiya o'chirilmagan", count)
	}
}

// TestRaceCondition - race detector bilan test (go test -race ile ishga tushiring)
func TestRaceCondition(t *testing.T) {
	handler := &BotHandler{
		configSessions: make(map[int64]*configSession),
	}

	var wg sync.WaitGroup

	// Bir vaqtning o'zida o'qish va yozish
	for i := 0; i < 10; i++ {
		wg.Add(2)

		// Yozish
		go func(id int64) {
			defer wg.Done()
			handler.startConfigSession(id)
		}(int64(i))

		// O'qish
		go func(id int64) {
			defer wg.Done()
			handler.hasConfigSession(id)
		}(int64(i))
	}

	wg.Wait()
}

func TestIsLikelyProductSuggestionRTX5090(t *testing.T) {
	text := "Ajoyib tanlov! Palit - 32GB GeForce RTX5090 GAMEROCK GDDR7 512bit 3-DP HDMI (NE7590019RS-GB2020G) - 2900$."
	if !isLikelyProductSuggestion(text) {
		t.Errorf("RTX5090 taklifini aniqlamadi")
	}
}

func TestIsLikelyProductSuggestionCoreUltra(t *testing.T) {
	text := "Ha, bizda Intel-Core Ultra 7-265KF, 5.5 GHz, 30MB, oem, LGA 1851, Arrow Lake bor. Narxi: 277$"
	if !isLikelyProductSuggestion(text) {
		t.Errorf("Core Ultra 7-265KF taklifini aniqlamadi")
	}
}

func TestIsLikelyProductSuggestionMonitor(t *testing.T) {
	text := "Ha, bizda \"Dell - 27\" S2725H Monitor, IPS, 100Hz, 8mc, FHD(1920x1080), HDMI, White\" monitori bor. Narxi: 170$."
	if !isLikelyProductSuggestion(text) {
		t.Errorf("Monitor S2725H taklifini aniqlamadi")
	}
}
