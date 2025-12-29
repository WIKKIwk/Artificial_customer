package telegram

import (
	"fmt"
	"math"
	"strings"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// FormatPCAnalytics PC tahlil natijalarini chiroyli format qiladi
func FormatPCAnalytics(build *entity.PCBuild, analytics *entity.PCAnalytics, lang string) string {
	var sb strings.Builder

	// Header
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(t(lang, "📊 PROFESSIONAL PC TAHLIL\n", "📊 ПРОФЕССИОНАЛЬНЫЙ АНАЛИЗ ПК\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Build Info
	sb.WriteString(fmt.Sprintf(t(lang, "💻 KONFIGURATSIYA: %s\n", "💻 КОНФИГУРАЦИЯ: %s\n"), build.Purpose))
	if build.ColorScheme != "" {
		sb.WriteString(fmt.Sprintf(t(lang, "🎨 Rang: %s\n", "🎨 Цвет: %s\n"), build.ColorScheme))
	}
	sb.WriteString(fmt.Sprintf(t(lang, "💰 Narx: $%.2f\n\n", "💰 Цена: $%.2f\n\n"), resolveBuildPrice(build)))

	// Overall Score
	sb.WriteString(fmt.Sprintf(t(lang, "⭐ UMUMIY REYTING: %.1f/10.0\n", "⭐ ОБЩИЙ РЕЙТИНГ: %.1f/10.0\n"), analytics.OverallScore))
	sb.WriteString(formatScoreBar(analytics.OverallScore, lang))
	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	requestedUseCase := resolveRequestedUseCase(build, analytics)
	if shouldShowFPS(requestedUseCase, analytics) {
		// FPS Section
		sb.WriteString(t(lang, "🎮 O'YINLARDA (FPS)\n", "🎮 В ИГРАХ (FPS)\n"))
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		games := []string{"CS2", "Cyberpunk 2077", "Red Dead Redemption 2", "GTA 5", "PUBG", "Fortnite", "Forza Horizon 5"}
		for _, gameName := range games {
			if fps, ok := analytics.FPS[gameName]; ok {
				sb.WriteString(formatFPSLine(gameName, fps))
			}
		}
		sb.WriteString("\n")
	} else {
		if workload := formatWorkloadSection(requestedUseCase, analytics, lang); workload != "" {
			sb.WriteString(workload)
		}
	}

	// Temperature Section
	sb.WriteString(t(lang, "🌡️ TEMPERATURA\n", "🌡️ ТЕМПЕРАТУРА\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(formatTemperature("CPU", analytics.CPUTemp, lang))
	sb.WriteString(formatTemperature("GPU", analytics.GPUTemp, lang))
	sb.WriteString("\n")

	// Bottleneck Section
	sb.WriteString(t(lang, "⚖️ MUVOZANAT (BOTTLENECK)\n", "⚖️ УЗКИЕ МЕСТА (BOTTLENECK)\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(formatBottleneck(analytics.Bottleneck, lang))
	sb.WriteString("\n")

	// Power Section
	sb.WriteString(t(lang, "⚡ QUVVAT SARFI & PSU\n", "⚡ ПОТРЕБЛЕНИЕ & БП\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(formatPower(analytics.PowerConsumption, lang))
	sb.WriteString("\n")

	// Storage Speed
	sb.WriteString(t(lang, "💾 STORAGE TEZLIGI\n", "💾 СКОРОСТЬ НАКОПИТЕЛЯ\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(formatStorageSpeed(analytics.StorageSpeed, lang))
	sb.WriteString("\n")

	// Use Case Match
	sb.WriteString(t(lang, "🎯 MAQSADGA MOS KELISH\n", "🎯 СООТВЕТСТВИЕ ЦЕЛИ\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(formatUseCaseMatch(analytics.UseCaseMatch, lang))
	sb.WriteString("\n")

	// Upgrade Path
	if len(analytics.UpgradePath) > 0 {
		sb.WriteString(t(lang, "📈 UPGRADE TAVSIYALARI\n", "📈 РЕКОМЕНДАЦИИ ПО АПГРЕЙДУ\n"))
		sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		sb.WriteString(formatUpgrades(analytics.UpgradePath, lang))
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(t(lang, "✅ Tahlil yakunlandi!\n", "✅ Анализ завершён!\n"))
	sb.WriteString(t(lang, "📞 Savollar bo'lsa, adminga yozing: @Ingame_support\n", "📞 Если есть вопросы, пишите админу: @Ingame_support\n"))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	return sb.String()
}

// formatScoreBar reyting uchun progress bar
func formatScoreBar(score float64, lang string) string {
	filled := int(score)
	empty := 10 - filled

	bar := "["
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}
	bar += "]"

	// Rating description
	desc := ""
	if score >= 9.0 {
		desc = t(lang, "🏆 A'lo darajada!", "🏆 Отлично!")
	} else if score >= 7.0 {
		desc = t(lang, "✨ Juda yaxshi!", "✨ Очень хорошо!")
	} else if score >= 5.0 {
		desc = t(lang, "👍 Yaxshi", "👍 Хорошо")
	} else {
		desc = t(lang, "⚠️ O'rtacha", "⚠️ Средне")
	}

	return fmt.Sprintf("%s %s\n", bar, desc)
}

// formatFPSLine bitta o'yin uchun FPS ma'lumoti
func formatFPSLine(gameName string, fps entity.FPSData) string {
	icon := "🎮"
	if !fps.IsPlayable {
		icon = "⚠️" // Unplayable
	} else if fps.Smoothness == "Smooth" {
		icon = "✅"
	} else if fps.Smoothness == "Stuttering" {
		icon = "📉"
	}

	// Format: 🎮 CS2: 1080p 250 | 1440p 180
	return fmt.Sprintf("%s %s:\n   1080p (Comp/High): %d │ 1440p (High/Ultra): %d\n",
		icon, gameName, fps.FPS1080p, fps.FPS1440p)
}

// formatTemperature CPU/GPU temperatura
func formatTemperature(component string, temp entity.TemperatureData, lang string) string {
	statusIcon := ""
	switch temp.Status {
	case "Excellent":
		statusIcon = "✅"
	case "Good":
		statusIcon = "👍"
	case "Warm":
		statusIcon = "⚠️"
	case "Hot":
		statusIcon = "🔥"
	}

	result := fmt.Sprintf(t(lang, "🌡️ %s: Idle %d°C │ Load %d°C │ %s %s\n", "🌡️ %s: Простой %d°C │ Нагрузка %d°C │ %s %s\n"),
		component, temp.Idle, temp.Load, temp.Status, statusIcon)

	result += fmt.Sprintf(t(lang, "   Sovutish: %s\n", "   Охлаждение: %s\n"), temp.CoolerType)

	if temp.Warning != "" {
		result += fmt.Sprintf("   %s\n", temp.Warning)
	}

	return result + "\n"
}

// formatBottleneck bottleneck tahlili
func formatBottleneck(b entity.BottleneckAnalysis, lang string) string {
	if !b.HasBottleneck {
		return t(lang, "✅ Bottleneck YO'Q\n", "✅ Узких мест нет\n") +
			"   " + b.Description + "\n"
	}

	return fmt.Sprintf(t(lang, "⚠️ %s BOTTLENECK (%.0f%%)\n", "⚠️ %s BOTTLENECK (%.0f%%)\n"), b.BottleneckType, b.Percentage) +
		"   " + b.Description + "\n" +
		t(lang, "   💡 Tavsiya: ", "   💡 Рекомендация: ") + b.Recommendation + "\n"
}

// formatPower quvvat sarfi
func formatPower(p entity.PowerData, lang string) string {
	if !p.IsAdequate {
		return fmt.Sprintf(t(lang, `🔴 DIQQAT: PSU KUCHSIZ!
⚡ Tizim talabi: ~%dW
🔌 Sizning PSU: %dW
❌ YETMAYDI! Kamida %dW tavsiya etiladi.
💡 %s
`, `🔴 ВНИМАНИЕ: БП СЛАБЫЙ!
⚡ Требование системы: ~%dW
🔌 Ваш БП: %dW
❌ НЕ ХВАТАЕТ! Рекомендуется минимум %dW.
💡 %s
`), p.TotalWattage, p.PSUWattage, int(float64(p.TotalWattage)*1.25), p.Recommendation)
	}

	return fmt.Sprintf(t(lang, `✅ QUVVAT YETARLI
⚡ Tizim talabi: ~%dW
🔌 Sizning PSU: %dW (Zaxira: %.0fW)
💡 %s
`, `✅ МОЩНОСТИ ДОСТАТОЧНО
⚡ Требование системы: ~%dW
🔌 Ваш БП: %dW (Запас: %.0fW)
💡 %s
`), p.TotalWattage, p.PSUWattage, p.HeadRoom, p.Recommendation)
}

// formatStorageSpeed storage tezligi
func formatStorageSpeed(s entity.StorageSpeedData, lang string) string {
	ratingIcon := ""
	switch s.Rating {
	case "Excellent":
		ratingIcon = "🏆"
	case "Excellent (Gen4)":
		ratingIcon = "⚡"
	case "Good":
		ratingIcon = "👍"
	case "Average":
		ratingIcon = "👌"
	default:
		ratingIcon = "⚠️"
	}

	return fmt.Sprintf(t(lang, "💾 Turi: %s %s\n", "💾 Тип: %s %s\n"), s.Type, ratingIcon) +
		fmt.Sprintf(t(lang, "   Read: %d MB/s │ Write: %d MB/s\n", "   Чтение: %d MB/s │ Запись: %d MB/s\n"), s.ReadSpeed, s.WriteSpeed)
}

// formatUseCaseMatch maqsadga mos kelish
func formatUseCaseMatch(u entity.UseCaseMatchData, lang string) string {
	result := ""
	if strings.TrimSpace(u.RequestedUseCase) != "" {
		result += fmt.Sprintf(t(lang, "🎯 So'ralgan: %s\n", "🎯 Запрошено: %s\n"), localizeUseCase(lang, u.RequestedUseCase))
	}
	result += fmt.Sprintf(t(lang, "🎯 Eng mos: %s\n\n", "🎯 Лучше всего: %s\n\n"), localizeUseCase(lang, u.BestFor))

	order := []string{"Gaming", "Developer", "Design", "Server", "Office"}
	for _, useCase := range order {
		score, ok := u.Matches[useCase]
		if !ok {
			continue
		}
		icon := ""
		if score.Score >= 8 {
			icon = "🏆"
		} else if score.Score >= 6 {
			icon = "👍"
		} else {
			icon = "👌"
		}

		result += fmt.Sprintf("%s %s: %.1f/10 (%s)\n", icon, localizeUseCase(lang, useCase), score.Score, score.Description)
	}

	if len(u.Limitations) > 0 {
		result += "\n" + t(lang, "⚠️ Cheklovlar:\n", "⚠️ Ограничения:\n")
		for _, limit := range u.Limitations {
			result += fmt.Sprintf("• %s\n", limit)
		}
	}

	return result
}

// formatUpgrades upgrade tavsiyalari
func formatUpgrades(upgrades []entity.UpgradeSuggestion, lang string) string {
	result := ""

	for i, u := range upgrades {
		priorityIcon := ""
		switch u.Priority {
		case "High":
			priorityIcon = "🔴"
		case "Medium":
			priorityIcon = "🟡"
		case "Low":
			priorityIcon = "🟢"
		}

		result += fmt.Sprintf("%d. %s %s → %s\n", i+1, priorityIcon, u.Component, u.SuggestedSpec)
		result += fmt.Sprintf(t(lang, "   Hozir: %s\n", "   Сейчас: %s\n"), u.CurrentSpec)
		result += fmt.Sprintf(t(lang, "   Foyda: %s\n", "   Польза: %s\n"), u.Benefit)
		result += fmt.Sprintf(t(lang, "   Narx: ~$%.0f\n\n", "   Цена: ~$%.0f\n\n"), u.EstimatedCost)
	}

	return result
}

func resolveRequestedUseCase(build *entity.PCBuild, analytics *entity.PCAnalytics) string {
	if analytics != nil {
		if normalized := normalizeUseCaseKey(analytics.UseCaseMatch.RequestedUseCase); normalized != "" {
			return normalized
		}
	}
	if build != nil {
		if normalized := normalizeUseCaseKey(build.Purpose); normalized != "" {
			return normalized
		}
	}
	return "Gaming"
}

func resolveBuildPrice(build *entity.PCBuild) float64 {
	if build == nil {
		return 0
	}
	componentTotal := build.GetTotalPrice()
	if build.Budget > 0 && (componentTotal == 0 || math.Abs(build.Budget-componentTotal) >= 0.5) {
		return build.Budget
	}
	return componentTotal
}

func shouldShowFPS(useCase string, analytics *entity.PCAnalytics) bool {
	if analytics == nil {
		return false
	}
	return isGamingUseCase(useCase) && len(analytics.FPS) > 0
}

func formatWorkloadSection(useCase string, analytics *entity.PCAnalytics, lang string) string {
	if analytics == nil {
		return ""
	}
	useCase = normalizeUseCaseKey(useCase)
	if useCase == "" {
		return ""
	}
	score, ok := analytics.UseCaseMatch.Matches[useCase]
	if !ok {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(workloadHeader(lang, useCase))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	sb.WriteString(fmt.Sprintf(t(lang, "⭐ Baho: %.1f/10 (%s)\n", "⭐ Оценка: %.1f/10 (%s)\n"), score.Score, score.Description))

	for _, line := range score.Strengths {
		sb.WriteString(fmt.Sprintf("✅ %s\n", line))
	}
	for _, line := range score.Weaknesses {
		sb.WriteString(fmt.Sprintf("⚠️ %s\n", line))
	}
	sb.WriteString("\n")
	return sb.String()
}

func workloadHeader(lang, useCase string) string {
	switch normalizeUseCaseKey(useCase) {
	case "Developer":
		return t(lang, "🧑‍💻 DEVELOPER ISH YUKLAMASI\n", "🧑‍💻 НАГРУЗКА ДЛЯ РАЗРАБОТКИ\n")
	case "Design":
		return t(lang, "🎨 DIZAYN/MONTAJ YUKLAMASI\n", "🎨 НАГРУЗКА ДЛЯ ДИЗАЙНА/МОНТАЖА\n")
	case "Server":
		return t(lang, "🖥️ SERVER YUKLAMASI\n", "🖥️ СЕРВЕРНАЯ НАГРУЗКА\n")
	case "Office":
		return t(lang, "💼 OFFICE YUKLAMASI\n", "💼 ОФИСНАЯ НАГРУЗКА\n")
	default:
		return t(lang, "🧩 ISH YUKLAMASI\n", "🧩 НАГРУЗКА\n")
	}
}

func normalizeUseCaseKey(useCase string) string {
	lower := strings.ToLower(strings.TrimSpace(useCase))
	switch {
	case strings.Contains(lower, "gaming") || strings.Contains(lower, "o'yin") || strings.Contains(lower, "oʻyin") ||
		strings.Contains(lower, "o‘yin") || strings.Contains(lower, "игр") || strings.Contains(lower, "game"):
		return "Gaming"
	case strings.Contains(lower, "developer") || strings.Contains(lower, "dev") || strings.Contains(lower, "dasturchi") ||
		strings.Contains(lower, "program") || strings.Contains(lower, "coding"):
		return "Developer"
	case strings.Contains(lower, "design") || strings.Contains(lower, "designer") || strings.Contains(lower, "dizayn") ||
		strings.Contains(lower, "montaj") || strings.Contains(lower, "editing") || strings.Contains(lower, "render"):
		return "Design"
	case strings.Contains(lower, "server") || strings.Contains(lower, "сервер") || strings.Contains(lower, "hosting") ||
		strings.Contains(lower, "vps"):
		return "Server"
	case strings.Contains(lower, "office") || strings.Contains(lower, "ofis") || strings.Contains(lower, "офис") ||
		strings.Contains(lower, "work") || strings.Contains(lower, "study"):
		return "Office"
	case strings.Contains(lower, "stream"):
		return "Gaming"
	default:
		return strings.TrimSpace(useCase)
	}
}

func localizeUseCase(_ string, useCase string) string {
	normalized := normalizeUseCaseKey(useCase)
	if normalized != "" {
		return normalized
	}
	return useCase
}

func isGamingUseCase(useCase string) bool {
	return normalizeUseCaseKey(useCase) == "Gaming"
}
