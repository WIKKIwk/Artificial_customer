package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
)

// PCAnalyzer PC konfiguratsiyasini tahlil qiladi
type PCAnalyzer struct {
	chatUseCase ChatUseCase
}

func pickLang(lang, uz, ru string) string {
	if strings.ToLower(strings.TrimSpace(lang)) == "ru" {
		return ru
	}
	return uz
}

// NewPCAnalyzer yangi PCAnalyzer yaratish
func NewPCAnalyzer(chatUseCase ChatUseCase) *PCAnalyzer {
	return &PCAnalyzer{
		chatUseCase: chatUseCase,
	}
}

// AnalyzePC to'liq PC tahlil (AI yordamida)
func (a *PCAnalyzer) AnalyzePC(ctx context.Context, build *entity.PCBuild, lang string) (*entity.PCAnalytics, error) {
	// AI uchun prompt tayyorlash
	promptUZ := `Quyidagi PC konfiguratsiyasini professional tahlil qilib ber:

CPU: %s
GPU: %s
RAM: %s
SSD: %s
PSU: %s
Case: %s
Cooler: %s
Maqsad: %s

MUHIM: FPS ma'lumotlari REAL WORLD benchmark'lardan olish kerak (Tom's Hardware, TechPowerUp, GamersNexus)

Tahlil quyidagi formatda bo'lishi SHART (har bir bo'lim alohida qatorda):

FPS_CS2_1080P: [raqam, Competitive Low settings]
FPS_CS2_1440P: [raqam, Competitive Low settings]
FPS_CYBERPUNK_1080P: [raqam, High/Ultra settings]
FPS_CYBERPUNK_1440P: [raqam, High settings]
FPS_PUBG_1080P: [raqam, Medium settings]
FPS_PUBG_1440P: [raqam, Medium settings]
FPS_GTA5_1080P: [raqam, High/Very High settings]
FPS_GTA5_1440P: [raqam, High settings]
FPS_FORTNITE_1080P: [raqam, High/Epic settings]
FPS_FORTNITE_1440P: [raqam, Medium/High settings]
FPS_COD_1080P: [raqam, Medium-High settings]
FPS_COD_1440P: [raqam, Medium settings]

CPU_TEMP_IDLE: [30-45 raqam, °C]
CPU_TEMP_LOAD: [70-95 raqam, °C]
GPU_TEMP_IDLE: [30-50 raqam, °C]
GPU_TEMP_LOAD: [75-85 raqam, °C]

BOTTLENECK_PERCENT: [0-30 raqam]
BOTTLENECK_TYPE: [CPU/GPU/None]
BOTTLENECK_DESC: [qisqa tushuntirish]

TDP_CPU_MAX: [raqam W]
TDP_GPU_MAX: [raqam W]

STORAGE_READ: [NVMe uchun 3000-7000 MB/s]
STORAGE_WRITE: [NVMe uchun 2000-6000 MB/s]

OVERALL_SCORE: [0-10 raqam]

REAL WORLD EXAMPLES (I5-13400F + RTX 4070 uchun):
- CS2 (Competitive Low): 1080p ~280+ FPS, 1440p ~170+ FPS
- Cyberpunk 2077 (High/Ultra): 1080p ~100-110 FPS, 1440p ~65-75 FPS
- Fortnite (High/Epic): 1080p ~180+ FPS, 1440p ~120+ FPS
- GTA 5 (High/Very High): 1080p ~150+ FPS, 1440p ~100+ FPS
- PUBG (Medium/High): 1080p ~160+ FPS, 1440p ~110+ FPS
- Call of Duty (Medium/High): 1080p ~140+ FPS, 1440p ~90+ FPS

Faqat shu formatda javob ber. Hech qanday qo'shimcha so'z yozma.`

	promptRU := `Профессионально проанализируй следующую конфигурацию ПК:

CPU: %s
GPU: %s
RAM: %s
SSD: %s
PSU: %s
Case: %s
Cooler: %s
Назначение: %s

ВАЖНО: значения FPS бери из реальных бенчмарков (Tom's Hardware, TechPowerUp, GamersNexus).

Ответ ДОЛЖЕН быть строго в таком формате (каждый параметр на отдельной строке, без лишних слов):

FPS_CS2_1080P: [число, Competitive Low settings]
FPS_CS2_1440P: [число, Competitive Low settings]
FPS_CYBERPUNK_1080P: [число, High/Ultra settings]
FPS_CYBERPUNK_1440P: [число, High settings]
FPS_PUBG_1080P: [число, Medium settings]
FPS_PUBG_1440P: [число, Medium settings]
FPS_GTA5_1080P: [число, High/Very High settings]
FPS_GTA5_1440P: [число, High settings]
FPS_FORTNITE_1080P: [число, High/Epic settings]
FPS_FORTNITE_1440P: [число, Medium/High settings]
FPS_COD_1080P: [число, Medium-High settings]
FPS_COD_1440P: [число, Medium settings]

CPU_TEMP_IDLE: [30-45 число, °C]
CPU_TEMP_LOAD: [70-95 число, °C]
GPU_TEMP_IDLE: [30-50 число, °C]
GPU_TEMP_LOAD: [75-85 число, °C]

BOTTLENECK_PERCENT: [0-30 число]
BOTTLENECK_TYPE: [CPU/GPU/None]
BOTTLENECK_DESC: [короткое объяснение]

TDP_CPU_MAX: [число W]
TDP_GPU_MAX: [число W]

STORAGE_READ: [для NVMe 3000-7000 MB/s]
STORAGE_WRITE: [для NVMe 2000-6000 MB/s]

OVERALL_SCORE: [0-10 число]

REAL WORLD EXAMPLES (для I5-13400F + RTX 4070):
- CS2 (Competitive Low): 1080p ~280+ FPS, 1440p ~170+ FPS
- Cyberpunk 2077 (High/Ultra): 1080p ~100-110 FPS, 1440p ~65-75 FPS
- Fortnite (High/Epic): 1080p ~180+ FPS, 1440p ~120+ FPS
- GTA 5 (High/Very High): 1080p ~150+ FPS, 1440p ~100+ FPS
- PUBG (Medium/High): 1080p ~160+ FPS, 1440p ~110+ FPS
- Call of Duty (Medium/High): 1080p ~140+ FPS, 1440p ~90+ FPS

Отвечай только в этом формате. Никаких дополнительных слов.`

	promptTemplate := promptUZ
	if strings.ToLower(strings.TrimSpace(lang)) == "ru" {
		promptTemplate = promptRU
	}

	prompt := fmt.Sprintf(promptTemplate,
		build.CPU.Name,
		build.GPU.Name,
		build.RAM.Name,
		build.SSD.Name,
		build.PSU.Name,
		safeName(build.Case),
		safeName(build.Cooler),
		build.Purpose,
	)

	// AI dan javob olish
	response, err := a.chatUseCase.ProcessMessage(ctx, build.UserID, "system", prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// Javobni parse qilish
	analytics := a.parseAIResponse(response, build, lang)
	return analytics, nil
}

func safeName(p *entity.Product) string {
	if p == nil {
		return "Not specified"
	}
	return p.Name
}

// parseAIResponse AI javobini structga o'girish
func (a *PCAnalyzer) parseAIResponse(response string, build *entity.PCBuild, lang string) *entity.PCAnalytics {
	analytics := &entity.PCAnalytics{
		FPS: make(map[string]entity.FPSData),
	}

	lines := strings.Split(response, "\n")
	var tdpCPU, tdpGPU int

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// FPS parsing
		if strings.HasPrefix(key, "FPS_") {
			parts := strings.Split(key, "_")
			if len(parts) >= 3 {
				// Handle COD_WARZONE special case
				var gameCode, res string
				if parts[1] == "COD" && len(parts) >= 4 {
					gameCode = "COD_WARZONE"
					res = parts[3]
				} else {
					gameCode = parts[1]
					res = parts[2]
				}

				fps, _ := strconv.Atoi(extractNumber(value))
				gameName := mapGameCodeToName(gameCode)

				data, ok := analytics.FPS[gameName]
				if !ok {
					// Use pointers if needed, but currently using value map
					// Using map assignment replaces the struct value
					data = entity.FPSData{GameName: gameName, Resolution: "Mixed"}
				}

				if res == "1080P" {
					data.FPS1080p = fps
				} else if res == "1440P" {
					data.FPS1440p = fps
				}

				// Playability
				data.IsPlayable = data.FPS1080p >= 60 || data.FPS1440p >= 60

				if data.FPS1080p >= 144 {
					data.Smoothness = "Smooth"
				} else if data.FPS1080p >= 60 {
					data.Smoothness = "Playable"
				} else {
					data.Smoothness = "Stuttering"
				}
				analytics.FPS[gameName] = data
			}
		}

		// Temp parsing
		if key == "CPU_TEMP_IDLE" {
			analytics.CPUTemp.Idle, _ = strconv.Atoi(extractNumber(value))
		} else if key == "CPU_TEMP_LOAD" {
			analytics.CPUTemp.Load, _ = strconv.Atoi(extractNumber(value))
			analytics.CPUTemp.Status = getTempStatus(analytics.CPUTemp.Load)
		} else if key == "GPU_TEMP_IDLE" {
			analytics.GPUTemp.Idle, _ = strconv.Atoi(extractNumber(value))
		} else if key == "GPU_TEMP_LOAD" {
			analytics.GPUTemp.Load, _ = strconv.Atoi(extractNumber(value))
			analytics.GPUTemp.Status = getTempStatus(analytics.GPUTemp.Load)
		}

		// Bottleneck
		if key == "BOTTLENECK_PERCENT" {
			analytics.Bottleneck.Percentage, _ = strconv.ParseFloat(extractNumber(value), 64)
		} else if key == "BOTTLENECK_TYPE" {
			analytics.Bottleneck.BottleneckType = value
			analytics.Bottleneck.HasBottleneck = value != "None"
		} else if key == "BOTTLENECK_DESC" {
			analytics.Bottleneck.Description = value
		}

		// Power Raw Data
		if key == "TDP_CPU_MAX" {
			tdpCPU, _ = strconv.Atoi(extractNumber(value))
		} else if key == "TDP_GPU_MAX" {
			tdpGPU, _ = strconv.Atoi(extractNumber(value))
		}

		// Storage & Boot
		if key == "STORAGE_READ" {
			analytics.StorageSpeed.ReadSpeed, _ = strconv.Atoi(extractNumber(value))
		} else if key == "STORAGE_WRITE" {
			analytics.StorageSpeed.WriteSpeed, _ = strconv.Atoi(extractNumber(value))
		} else if key == "BOOT_TIME" {
			analytics.BootTime.BootTime, _ = strconv.Atoi(extractNumber(value))
		}

		// Overall
		if key == "OVERALL_SCORE" {
			analytics.OverallScore, _ = strconv.ParseFloat(extractNumber(value), 64)
		}
	}

	// Power Logic Calculation (Manual)
	// Base System overhead (Motherboard, RAM, SSD, Fans): ~60-80W
	systemOverhead := 75

	// If AI failed to give TDPs, use defaults to avoid 0
	if tdpCPU == 0 {
		tdpCPU = 65
	} // Safe default for midrange
	if tdpGPU == 0 {
		tdpGPU = 115
	} // Safe default for midrange

	// Calculate Max Load
	// We add 10% safety margin to component TDPs for potential boosts/AIB factory OCs
	estimatedMaxLoad := int(float64(tdpCPU)*1.1+float64(tdpGPU)*1.1) + systemOverhead
	analytics.PowerConsumption.TotalWattage = estimatedMaxLoad

	// 2. Extract PSU wattage
	psuWattage := extractWattageFromName(build.PSU.Name)
	analytics.PowerConsumption.PSUWattage = psuWattage

	// 3. Calculate Recommended
	// Recommended: Load + 200W safety margin (User request)
	recommended := float64(estimatedMaxLoad + 200)
	// Round up to nearest 50
	recInt := int(recommended)
	if recInt%50 != 0 {
		recInt = ((recInt / 50) + 1) * 50
	}

	analytics.PowerConsumption.HeadRoom = float64(psuWattage - estimatedMaxLoad)

	// Is Adequate?
	// As long as PSU Wattage >= Estimated Load + 10%, it's "Technically Safe",
	// but "Recommended" is usually higher.
	// Let's say Strict Limit is Load + 50W.
	safeLimit := estimatedMaxLoad + 50

	analytics.PowerConsumption.IsAdequate = psuWattage >= safeLimit

	if analytics.PowerConsumption.IsAdequate {
		if psuWattage >= recInt {
			analytics.PowerConsumption.Recommendation = fmt.Sprintf(pickLang(lang,
				"✅ Mukammal! PSU quvvati yetarli va zaxirasi bor. (Rec: %dW)",
				"✅ Отлично! Мощности БП хватает с запасом. (Rec: %dW)",
			), recInt)
		} else {
			analytics.PowerConsumption.Recommendation = fmt.Sprintf(pickLang(lang,
				"✅ Yetarli, lekin kelajakdagi upgrade uchun %dW yaxshiroq bo'lardi.",
				"✅ Достаточно, но для будущего апгрейда лучше %dW.",
			), recInt)
		}
	} else {
		analytics.PowerConsumption.Recommendation = fmt.Sprintf(pickLang(lang,
			"⚠️ XAVFLI! PSU kuchsizlik qilishi mumkin. Tavsiya: %dW",
			"⚠️ РИСКОВАННО! БП может не потянуть. Рекомендация: %dW",
		), recInt)
	}

	// Fill missing fields with defaults if needed
	if analytics.BootTime.BootTime < 10 {
		analytics.BootTime.Description = "Fast"
	} else {
		analytics.BootTime.Description = "Normal"
	}

	storageScore, storageType, defaultRead, defaultWrite := scoreStorage(build.SSD.Name)
	if analytics.StorageSpeed.Type == "" {
		analytics.StorageSpeed.Type = storageType
	}
	if analytics.StorageSpeed.ReadSpeed == 0 && defaultRead > 0 {
		analytics.StorageSpeed.ReadSpeed = defaultRead
	}
	if analytics.StorageSpeed.WriteSpeed == 0 && defaultWrite > 0 {
		analytics.StorageSpeed.WriteSpeed = defaultWrite
	}
	if analytics.StorageSpeed.Rating == "" {
		analytics.StorageSpeed.Rating = inferStorageRating(analytics.StorageSpeed.ReadSpeed, storageType)
	}

	analytics.UseCaseMatch = computeUseCaseMatch(build, lang, storageScore, storageType)

	return analytics
}

func extractNumber(s string) string {
	re := regexp.MustCompile(`\d+(\.\d+)?`)
	return re.FindString(s)
}

func mapGameCodeToName(code string) string {
	switch code {
	case "CS2":
		return "CS2"
	case "CYBERPUNK":
		return "Cyberpunk 2077"
	case "RDR2":
		return "Red Dead Redemption 2"
	case "GTA5":
		return "GTA 5"
	case "PUBG":
		return "PUBG"
	case "FORTNITE":
		return "Fortnite"
	case "FORZA":
		return "Forza Horizon 5"
	case "COD_WARZONE":
		return "Call of Duty: Warzone"
	default:
		return code
	}
}

func getTempStatus(temp int) string {
	if temp > 85 {
		return "Hot"
	} else if temp > 75 {
		return "Warm"
	}
	return "Good"
}

func extractWattageFromName(name string) int {
	re := regexp.MustCompile(`(\d{3,4})[Ww]`)
	match := re.FindStringSubmatch(name)
	if len(match) > 1 {
		val, _ := strconv.Atoi(match[1])
		return val
	}
	// Fallback: look for just 3-4 digits if "W" is missing but it looks like a PSU model?
	re2 := regexp.MustCompile(`\d{3,4}`)
	match2 := re2.FindStringSubmatch(name)
	if len(match2) > 0 {
		val, _ := strconv.Atoi(match2[0])
		return val
	}
	return 0
}

type hardwareProfile struct {
	cpuScore     float64
	gpuScore     float64
	ramScore     float64
	storageScore float64
	ramGB        int
	storageType  string
	cpuTier      string
	gpuTier      string
	serverCPU    bool
}

func computeUseCaseMatch(build *entity.PCBuild, lang string, storageScore float64, storageType string) entity.UseCaseMatchData {
	requested := normalizeUseCase(build.Purpose)
	if requested == "" {
		requested = "Gaming"
	}

	profile := buildHardwareProfile(build, storageScore, storageType)

	matches := map[string]entity.UseCaseScore{
		"Gaming":    buildUseCaseScore("Gaming", profile, lang),
		"Developer": buildUseCaseScore("Developer", profile, lang),
		"Design":    buildUseCaseScore("Design", profile, lang),
		"Server":    buildUseCaseScore("Server", profile, lang),
		"Office":    buildUseCaseScore("Office", profile, lang),
	}

	bestFor := requested
	bestScore := -1.0
	for _, useCase := range []string{"Gaming", "Developer", "Design", "Server", "Office"} {
		if score, ok := matches[useCase]; ok {
			if score.Score > bestScore {
				bestScore = score.Score
				bestFor = useCase
			}
		}
	}
	if _, ok := matches[requested]; !ok {
		requested = bestFor
	}

	return entity.UseCaseMatchData{
		RequestedUseCase: requested,
		Matches:          matches,
		BestFor:          bestFor,
	}
}

func buildHardwareProfile(build *entity.PCBuild, storageScore float64, storageType string) hardwareProfile {
	cpuScore, cpuTier, serverCPU := scoreCPU(build.CPU.Name)
	gpuScore, gpuTier := scoreGPU(build.GPU.Name)
	ramGB := extractRAMSizeGB(build.RAM.Name)
	ramScore := scoreRAM(ramGB)

	if storageScore <= 0 {
		storageScore, storageType, _, _ = scoreStorage(build.SSD.Name)
	}

	return hardwareProfile{
		cpuScore:     cpuScore,
		gpuScore:     gpuScore,
		ramScore:     ramScore,
		storageScore: storageScore,
		ramGB:        ramGB,
		storageType:  storageType,
		cpuTier:      cpuTier,
		gpuTier:      gpuTier,
		serverCPU:    serverCPU,
	}
}

func buildUseCaseScore(useCase string, profile hardwareProfile, lang string) entity.UseCaseScore {
	score := 0.0
	strengths := []string{}
	weaknesses := []string{}

	switch useCase {
	case "Developer":
		score = profile.cpuScore*0.4 + profile.ramScore*0.3 + profile.storageScore*0.2 + profile.gpuScore*0.1
		if profile.ramGB < 16 {
			score -= 1.0
		} else if profile.ramGB >= 32 {
			score += 0.5
		}
		if profile.storageScore < 6 {
			score -= 0.7
		}
		if profile.cpuScore < 6 {
			score -= 0.6
		}

		if profile.cpuScore >= 7 {
			strengths = append(strengths, pickLang(lang, "CPU kuchli: build/compile tez", "Сильный CPU: быстрая сборка/компиляция"))
		}
		if profile.ramGB >= 32 {
			strengths = append(strengths, pickLang(lang, "RAM ko'p: Docker/VM uchun qulay", "Много ОЗУ: удобно для Docker/VM"))
		} else if profile.ramGB >= 16 {
			strengths = append(strengths, pickLang(lang, "RAM yetarli: IDE va multitasking", "ОЗУ достаточно: IDE и мультизадачность"))
		}
		if profile.storageScore >= 8 {
			strengths = append(strengths, pickLang(lang, "NVMe tezligi: build cache va git tez", "NVMe скорость: быстрые build cache и git"))
		} else if profile.storageScore >= 6 {
			strengths = append(strengths, pickLang(lang, "SSD bor: umumiy ish tez", "Есть SSD: система отзывчивая"))
		}

		if profile.ramGB < 16 {
			weaknesses = append(weaknesses, pickLang(lang, "RAM kam: katta loyihalar/VM sekin", "Мало ОЗУ: большие проекты/VM будут медленнее"))
		}
		if profile.storageScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "Disk sekin: build va repo operatsiyalar kechikadi", "Медленный диск: build и git операции дольше"))
		}
		if profile.cpuScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "CPU past: compile sekin", "Слабый CPU: компиляция медленнее"))
		}
	case "Design":
		score = profile.gpuScore*0.35 + profile.cpuScore*0.25 + profile.ramScore*0.25 + profile.storageScore*0.15
		if profile.ramGB < 16 {
			score -= 1.0
		} else if profile.ramGB >= 32 {
			score += 0.5
		}
		if profile.gpuScore < 6 {
			score -= 1.2
		}
		if profile.storageScore < 6 {
			score -= 0.6
		}

		if profile.gpuScore >= 7 {
			strengths = append(strengths, pickLang(lang, "GPU kuchli: 3D/render uchun yaxshi", "Сильный GPU: хорошо для 3D/рендера"))
		}
		if profile.cpuScore >= 7 {
			strengths = append(strengths, pickLang(lang, "CPU yaxshi: render/encode tez", "Хороший CPU: быстрый рендер/энкод"))
		}
		if profile.ramGB >= 32 {
			strengths = append(strengths, pickLang(lang, "RAM ko'p: 4K/PSD loyihalar uchun qulay", "Много ОЗУ: удобно для 4K/PSD проектов"))
		} else if profile.ramGB >= 16 {
			strengths = append(strengths, pickLang(lang, "RAM yetarli: dizayn va montaj uchun", "ОЗУ достаточно: для дизайна и монтажа"))
		}
		if profile.storageScore >= 8 {
			strengths = append(strengths, pickLang(lang, "NVMe: media cache tez", "NVMe: быстрый медиакэш"))
		}

		if profile.gpuScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "GPU zaif: 3D/GPU effektlar sekin", "Слабый GPU: 3D/GPU эффекты будут медленнее"))
		}
		if profile.ramGB < 16 {
			weaknesses = append(weaknesses, pickLang(lang, "RAM kam: katta fayllarda lag bo'lishi mumkin", "Мало ОЗУ: возможны лаги на больших проектах"))
		}
		if profile.cpuScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "CPU sekin: render/encode sekinlashadi", "Слабый CPU: рендер/энкод медленнее"))
		}
		if profile.storageScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "Disk sekin: media cache sekin ishlaydi", "Медленный диск: медиакэш будет тормозить"))
		}
	case "Server":
		score = profile.cpuScore*0.4 + profile.ramScore*0.35 + profile.storageScore*0.2 + profile.gpuScore*0.05
		if profile.serverCPU {
			score += 0.4
		}
		if profile.ramGB < 32 {
			score -= 1.0
		}
		if profile.cpuScore < 6 {
			score -= 0.7
		}
		if profile.storageScore < 6 {
			score -= 0.7
		}

		if profile.serverCPU {
			strengths = append(strengths, pickLang(lang, "Server sinfi CPU: barqaror va ko'p yadro", "Серверный CPU: стабильность и много ядер"))
		}
		if profile.cpuScore >= 7 {
			strengths = append(strengths, pickLang(lang, "CPU ko'p yadroli: parallel servislar uchun", "Многопоточный CPU: для параллельных сервисов"))
		}
		if profile.ramGB >= 32 {
			strengths = append(strengths, pickLang(lang, "RAM ko'p: VM/DB uchun qulay", "Много ОЗУ: удобно для VM/БД"))
		}
		if profile.storageScore >= 7 {
			strengths = append(strengths, pickLang(lang, "Disk tez: IO/DB ishlari uchun yaxshi", "Быстрый диск: хорош для IO/БД"))
		}

		if profile.ramGB < 32 {
			weaknesses = append(weaknesses, pickLang(lang, "RAM kam: ko'p servislar uchun cheklov bo'lishi mumkin", "Мало ОЗУ: может ограничить число сервисов"))
		}
		if profile.cpuScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "CPU past: yuqori yukda sekinlashadi", "Слабый CPU: медленнее при высокой нагрузке"))
		}
		if profile.storageScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "Disk sekin: IO/DB ishlashi pasayadi", "Медленный диск: IO/БД будут медленнее"))
		}
	case "Office":
		score = profile.cpuScore*0.3 + profile.ramScore*0.3 + profile.storageScore*0.3 + profile.gpuScore*0.1
		if profile.ramGB < 8 {
			score -= 1.0
		}
		if profile.storageScore < 5 {
			score -= 0.5
		}
		if profile.cpuScore < 4.5 {
			score -= 0.4
		}

		if profile.cpuScore >= 5 {
			strengths = append(strengths, pickLang(lang, "CPU yetarli: ofis ishlari uchun", "CPU достаточно: для офисных задач"))
		}
		if profile.ramGB >= 8 {
			strengths = append(strengths, pickLang(lang, "RAM yetarli: ko'p tab va zoom uchun", "ОЗУ достаточно: много вкладок и zoom"))
		}
		if profile.storageScore >= 6 {
			strengths = append(strengths, pickLang(lang, "SSD bor: tizim tez yuklanadi", "Есть SSD: система быстро загружается"))
		}

		if profile.ramGB < 8 {
			weaknesses = append(weaknesses, pickLang(lang, "RAM kam: brauzerda sekinlashishi mumkin", "Мало ОЗУ: браузер может тормозить"))
		}
		if profile.storageScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "Disk sekin: ochilishlar sekinlashadi", "Медленный диск: запуск программ дольше"))
		}
		if profile.cpuScore < 4.5 {
			weaknesses = append(weaknesses, pickLang(lang, "CPU past: ofis ishlarida ham sekin", "Слабый CPU: даже офисные задачи медленнее"))
		}
	default: // Gaming
		score = profile.gpuScore*0.45 + profile.cpuScore*0.3 + profile.ramScore*0.15 + profile.storageScore*0.1
		if profile.gpuScore < 6 {
			score -= 1.0
		}
		if profile.cpuScore < 6 {
			score -= 0.6
		}
		if profile.ramGB < 16 {
			score -= 0.5
		}

		if profile.gpuScore >= 7 {
			strengths = append(strengths, pickLang(lang, "GPU kuchli: yuqori FPS uchun", "Сильный GPU: для высокого FPS"))
		}
		if profile.cpuScore >= 7 {
			strengths = append(strengths, pickLang(lang, "CPU yaxshi: stabil FPS", "Хороший CPU: стабильный FPS"))
		}
		if profile.ramGB >= 16 {
			strengths = append(strengths, pickLang(lang, "RAM yetarli: AAA o'yinlar uchun", "ОЗУ достаточно: для AAA игр"))
		}

		if profile.gpuScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "GPU zaif: og'ir o'yinlarda FPS past bo'lishi mumkin", "Слабый GPU: низкий FPS в тяжелых играх"))
		}
		if profile.cpuScore < 6 {
			weaknesses = append(weaknesses, pickLang(lang, "CPU bottleneck bo'lishi mumkin", "Возможен bottleneck по CPU"))
		}
		if profile.ramGB < 16 {
			weaknesses = append(weaknesses, pickLang(lang, "RAM kam: stutter bo'lishi mumkin", "Мало ОЗУ: возможны подлагивания"))
		}
	}

	score = clampScore(score)

	return entity.UseCaseScore{
		Score:       score,
		Description: scoreDescription(score, lang),
		Strengths:   strengths,
		Weaknesses:  weaknesses,
	}
}

func normalizeUseCase(purpose string) string {
	lower := strings.ToLower(strings.TrimSpace(purpose))
	if lower == "" {
		return ""
	}
	switch {
	case strings.Contains(lower, "gaming") || strings.Contains(lower, "o'yin") || strings.Contains(lower, "oʻyin") ||
		strings.Contains(lower, "o‘yin") || strings.Contains(lower, "игр") || strings.Contains(lower, "game"):
		return "Gaming"
	case strings.Contains(lower, "developer") || strings.Contains(lower, "dev") || strings.Contains(lower, "dasturchi") ||
		strings.Contains(lower, "program") || strings.Contains(lower, "coding") || strings.Contains(lower, "backend"):
		return "Developer"
	case strings.Contains(lower, "design") || strings.Contains(lower, "designer") || strings.Contains(lower, "dizayn") ||
		strings.Contains(lower, "montaj") || strings.Contains(lower, "editing") || strings.Contains(lower, "render"):
		return "Design"
	case strings.Contains(lower, "server") || strings.Contains(lower, "сервер") || strings.Contains(lower, "hosting") ||
		strings.Contains(lower, "vps") || strings.Contains(lower, "nas"):
		return "Server"
	case strings.Contains(lower, "office") || strings.Contains(lower, "ofis") || strings.Contains(lower, "офис") ||
		strings.Contains(lower, "work") || strings.Contains(lower, "study"):
		return "Office"
	case strings.Contains(lower, "stream"):
		return "Gaming"
	default:
		return ""
	}
}

func scoreDescription(score float64, lang string) string {
	switch {
	case score >= 8.5:
		return pickLang(lang, "A'lo", "Отлично")
	case score >= 7.0:
		return pickLang(lang, "Juda yaxshi", "Очень хорошо")
	case score >= 5.5:
		return pickLang(lang, "Yaxshi", "Хорошо")
	case score >= 4.0:
		return pickLang(lang, "O'rtacha", "Средне")
	default:
		return pickLang(lang, "Zaif", "Слабо")
	}
}

func scoreCPU(name string) (float64, string, bool) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "threadripper") || strings.Contains(lower, "epyc"):
		return 9.5, "Workstation", true
	case strings.Contains(lower, "xeon"):
		return 9.0, "Server", true
	case strings.Contains(lower, "ryzen 9") || strings.Contains(lower, "i9"):
		return 9.0, "High", false
	case strings.Contains(lower, "ryzen 7") || strings.Contains(lower, "i7"):
		return 7.8, "Upper CPU", false
	case strings.Contains(lower, "ryzen 5") || strings.Contains(lower, "i5"):
		return 6.5, "Mid CPU", false
	case strings.Contains(lower, "ryzen 3") || strings.Contains(lower, "i3"):
		return 5.0, "Entry CPU", false
	case strings.Contains(lower, "pentium") || strings.Contains(lower, "celeron") || strings.Contains(lower, "athlon"):
		return 3.5, "Low CPU", false
	default:
		if strings.TrimSpace(lower) == "" {
			return 4.5, "Unknown", false
		}
		return 5.5, "Mid CPU", false
	}
}

func scoreGPU(name string) (float64, string) {
	lower := strings.ToLower(name)
	if strings.TrimSpace(lower) == "" {
		return 3.5, "Unknown"
	}

	if containsAny(lower, "uhd", "iris", "vega", "radeon graphics", "integrated") {
		return 3.0, "iGPU"
	}
	if containsAny(lower, "rtx 4090", "rtx4090", "rx 7900 xtx", "7900 xtx", "a6000") {
		return 9.8, "Ultra GPU"
	}
	if containsAny(lower, "rtx 4080", "rtx4080", "rx 7900 xt", "7900 xt") {
		return 9.2, "High GPU"
	}
	if containsAny(lower, "rtx 4070", "rtx4070", "rx 7800", "rx 7700", "rtx 3090", "rtx 3080", "rx 6900", "rx 6800") {
		return 8.2, "Upper GPU"
	}
	if containsAny(lower, "rtx 3070", "rtx3070", "rx 6750", "rx 6700", "rtx 2080") {
		return 7.2, "Upper-mid GPU"
	}
	if containsAny(lower, "rtx 4060", "rtx4060", "rtx 3060", "rtx3060", "rx 6650", "rx 6600", "rtx 2070") {
		return 6.3, "Mid GPU"
	}
	if containsAny(lower, "rtx 3050", "rtx3050", "gtx 1660", "gtx 1070", "rx 580") {
		return 5.3, "Entry GPU"
	}
	if containsAny(lower, "gtx 1650", "gtx 1050", "rx 560") {
		return 4.5, "Low GPU"
	}
	if containsAny(lower, "quadro", "rtx a2000", "a2000", "a4000", "a5000") {
		return 7.5, "Workstation GPU"
	}

	return 5.5, "Mid GPU"
}

func extractRAMSizeGB(name string) int {
	lower := strings.ToLower(name)
	if lower == "" {
		return 0
	}

	rePair := regexp.MustCompile(`(?i)(\d+)\s*[x×]\s*(\d+)\s*gb`)
	if matches := rePair.FindStringSubmatch(lower); len(matches) == 3 {
		first, _ := strconv.Atoi(matches[1])
		second, _ := strconv.Atoi(matches[2])
		if first > 0 && second > 0 {
			return first * second
		}
	}

	reGB := regexp.MustCompile(`(?i)(\d+)\s*gb`)
	matches := reGB.FindAllStringSubmatch(lower, -1)
	max := 0
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		val, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		if val > max {
			max = val
		}
	}
	if max > 0 {
		return max
	}

	reTB := regexp.MustCompile(`(?i)(\d+)\s*tb`)
	if matches := reTB.FindStringSubmatch(lower); len(matches) == 2 {
		val, _ := strconv.Atoi(matches[1])
		if val > 0 {
			return val * 1024
		}
	}

	return 0
}

func scoreRAM(ramGB int) float64 {
	switch {
	case ramGB >= 128:
		return 10.0
	case ramGB >= 64:
		return 9.2
	case ramGB >= 32:
		return 8.3
	case ramGB >= 16:
		return 6.8
	case ramGB >= 8:
		return 5.2
	case ramGB > 0:
		return 3.8
	default:
		return 5.0
	}
}

func scoreStorage(name string) (float64, string, int, int) {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "gen5"):
		return 9.5, "NVMe Gen5", 10000, 9000
	case strings.Contains(lower, "gen4"):
		return 9.0, "NVMe Gen4", 5000, 4500
	case strings.Contains(lower, "gen3"):
		return 8.0, "NVMe Gen3", 3500, 3000
	case strings.Contains(lower, "nvme") || strings.Contains(lower, "m.2") || strings.Contains(lower, "pcie"):
		return 8.2, "NVMe", 3200, 2800
	case strings.Contains(lower, "ssd"):
		return 6.5, "SATA SSD", 550, 500
	case strings.Contains(lower, "hdd"):
		return 3.5, "HDD", 150, 120
	default:
		if strings.TrimSpace(lower) == "" {
			return 5.0, "Storage", 0, 0
		}
		return 5.5, "Storage", 0, 0
	}
}

func inferStorageRating(read int, storageType string) string {
	switch {
	case read >= 6000:
		return "Excellent (Gen4)"
	case read >= 3500:
		return "Excellent"
	case read >= 2500:
		return "Good"
	case read >= 1000:
		return "Average"
	case read > 0:
		return "Slow"
	}

	lower := strings.ToLower(storageType)
	switch {
	case strings.Contains(lower, "nvme"):
		return "Good"
	case strings.Contains(lower, "ssd"):
		return "Average"
	case strings.Contains(lower, "hdd"):
		return "Slow"
	default:
		return "Average"
	}
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 10 {
		return 10
	}
	return score
}

func containsAny(haystack string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(haystack, token) {
			return true
		}
	}
	return false
}
