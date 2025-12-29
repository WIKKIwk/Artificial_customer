package telegram

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultWorkStartHour = 9
	defaultWorkEndHour   = 20
	defaultWorkTZ        = "Asia/Tashkent"
)

func shouldNotifyAfterHours(now time.Time) bool {
	startHour := envHour("ADMIN_WORK_START_HOUR", defaultWorkStartHour)
	endHour := envHour("ADMIN_WORK_END_HOUR", defaultWorkEndHour)
	loc := loadWorkLocation()
	now = now.In(loc)

	if startHour == endHour {
		return false
	}

	hour := now.Hour()
	if startHour < endHour {
		return hour < startHour || hour >= endHour
	}
	// Overnight shift: work hours cross midnight.
	return hour >= endHour && hour < startHour
}

func loadWorkLocation() *time.Location {
	tz := strings.TrimSpace(os.Getenv("ADMIN_WORK_TZ"))
	if tz == "" {
		tz = defaultWorkTZ
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.Local
}

func envHour(key string, def int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if val < 0 {
		return 0
	}
	if val > 23 {
		return 23
	}
	return val
}

func adminApprovalWaitMessage(lang string, now time.Time) string {
	if shouldNotifyAfterHours(now) {
		return t(lang,
			"Konfiguratsiya admin tasdiqlashiga yuborildi. Hozir ish vaqti tugagan — iltimos, ertalabgacha kuting. Adminlar imkon qadar tez javob berishadi.",
			"Конфигурация отправлена на утверждение администратору. Сейчас вне рабочего времени — пожалуйста, дождитесь утра. Админы ответят при первой возможности.",
		)
	}
	return t(lang,
		"Konfiguratsiya admin tasdiqlashiga yuborildi. Admin javobidan keyin rasmiylashtirishni davom ettirasiz.",
		"Конфигурация отправлена на утверждение администратору. После ответа админа продолжите оформление.",
	)
}
