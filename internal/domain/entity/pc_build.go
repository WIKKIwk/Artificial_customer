package entity

import "time"

// PCBuild to'liq PC konfiguratsiya
type PCBuild struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"user_id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`

	// Komponentlar
	CPU         Product  `json:"cpu"`
	GPU         Product  `json:"gpu"`
	RAM         Product  `json:"ram"`
	SSD         Product  `json:"ssd"`
	HDD         *Product `json:"hdd,omitempty"` // Ixtiyoriy
	Motherboard Product  `json:"motherboard"`
	PSU         Product  `json:"psu"`
	Case        *Product `json:"case,omitempty"`    // Ixtiyoriy
	Cooler      *Product `json:"cooler,omitempty"`  // Ixtiyoriy
	Monitor     *Product `json:"monitor,omitempty"` // Ixtiyoriy

	// Foydalanuvchi talablari
	Budget      float64 `json:"budget"`
	Purpose     string  `json:"purpose"`      // Gaming, Developer, Design, Server, Office
	ColorScheme string  `json:"color_scheme"` // RGB, Black, White, Custom
	HasMonitor  bool    `json:"has_monitor"`  // Monitor kerakmi?

	// Hisoblangan ma'lumotlar (Analyze PC)
	Analytics *PCAnalytics `json:"analytics,omitempty"`
}

// PCAnalytics PC tahlil natijalari
type PCAnalytics struct {
	// FPS (Frames Per Second) - 7 ta o'yin uchun
	FPS map[string]FPSData `json:"fps"` // Game name -> FPS data

	// Temperatura
	CPUTemp TemperatureData `json:"cpu_temp"`
	GPUTemp TemperatureData `json:"gpu_temp"`

	// Bottleneck tahlili
	Bottleneck BottleneckAnalysis `json:"bottleneck"`

	// Quvvat sarfi
	PowerConsumption PowerData `json:"power_consumption"`

	// Umumiy reyting
	OverallScore float64 `json:"overall_score"` // 0-10

	// Upgrade yo'li
	UpgradePath []UpgradeSuggestion `json:"upgrade_path"`

	// Boot vaqti
	BootTime BootTimeData `json:"boot_time"`

	// Shovqin darajasi
	NoiseLevel NoiseLevelData `json:"noise_level"`

	// Storage tezligi
	StorageSpeed StorageSpeedData `json:"storage_speed"`

	// Use case match
	UseCaseMatch UseCaseMatchData `json:"use_case_match"`
}

// FPSData o'yin uchun FPS ma'lumotlari
type FPSData struct {
	GameName   string `json:"game_name"`
	FPS1080p   int    `json:"fps_1080p"`   // 1080p settings (Usually Ultra or Competitive)
	FPS1440p   int    `json:"fps_1440p"`   // 1440p settings (Ultra)
	Resolution string `json:"resolution"`  // Deprecated/Generic
	IsPlayable bool   `json:"is_playable"` // 60+ FPS bo'lsa true
	Smoothness string `json:"smoothness"`  // Smooth, Playable, Stuttering
}

// TemperatureData temperatura ma'lumotlari
type TemperatureData struct {
	Idle       int    `json:"idle"`              // Idle holatda (°C)
	Load       int    `json:"load"`              // Full load holatda (°C)
	CoolerType string `json:"cooler_type"`       // Stock, Tower, AIO, Custom
	Status     string `json:"status"`            // Excellent, Good, Warm, Hot
	Warning    string `json:"warning,omitempty"` // Ogohantirish (agar hot bo'lsa)
}

// BottleneckAnalysis bottleneck tahlili
type BottleneckAnalysis struct {
	HasBottleneck  bool    `json:"has_bottleneck"`
	BottleneckType string  `json:"bottleneck_type"` // CPU, GPU, RAM, None
	Percentage     float64 `json:"percentage"`      // Bottleneck foizi
	Description    string  `json:"description"`
	Recommendation string  `json:"recommendation"`
}

// PowerData quvvat sarfi
type PowerData struct {
	TotalWattage   int     `json:"total_wattage"`  // Umumiy quvvat (W)
	PSUWattage     int     `json:"psu_wattage"`    // PSU quvvati (W)
	PSUEfficiency  string  `json:"psu_efficiency"` // 80+ Bronze, Gold, Platinum
	HeadRoom       float64 `json:"headroom"`       // Qolgan zaxira (%)
	IsAdequate     bool    `json:"is_adequate"`    // PSU yetarlimi?
	Recommendation string  `json:"recommendation,omitempty"`
}

// UpgradeSuggestion upgrade tavsiyasi
type UpgradeSuggestion struct {
	Component     string  `json:"component"`      // CPU, GPU, RAM, SSD
	CurrentSpec   string  `json:"current_spec"`   // Hozirgi spec
	SuggestedSpec string  `json:"suggested_spec"` // Tavsiya etilgan spec
	Priority      string  `json:"priority"`       // High, Medium, Low
	Benefit       string  `json:"benefit"`        // Qanday foyda
	EstimatedCost float64 `json:"estimated_cost"` // Taxminiy narx
}

// BootTimeData boot vaqti
type BootTimeData struct {
	StorageType string `json:"storage_type"` // NVMe, SATA SSD, HDD
	BootTime    int    `json:"boot_time"`    // Sekundda
	Description string `json:"description"`  // Fast, Normal, Slow
}

// NoiseLevelData shovqin darajasi
type NoiseLevelData struct {
	IdleDB      int    `json:"idle_db"`     // Idle holatda (dB)
	LoadDB      int    `json:"load_db"`     // Load holatda (dB)
	CoolerType  string `json:"cooler_type"` // Stock, Tower, AIO
	Description string `json:"description"` // Silent, Quiet, Moderate, Loud
}

// StorageSpeedData storage tezligi
type StorageSpeedData struct {
	Type       string `json:"type"`        // NVMe, SATA SSD, HDD
	ReadSpeed  int    `json:"read_speed"`  // MB/s
	WriteSpeed int    `json:"write_speed"` // MB/s
	Rating     string `json:"rating"`      // Excellent, Good, Average, Slow
}

// UseCaseMatchData use case mos kelishi
type UseCaseMatchData struct {
	RequestedUseCase string                  `json:"requested_use_case"` // Gaming, Developer, Design, Server, Office
	Matches          map[string]UseCaseScore `json:"matches"`            // Use case -> score
	BestFor          string                  `json:"best_for"`           // Eng mos kelgan use case
	Limitations      []string                `json:"limitations"`        // Cheklovlar
}

// UseCaseScore use case uchun ball
type UseCaseScore struct {
	Score       float64  `json:"score"`       // 0-10
	Description string   `json:"description"` // Excellent, Good, Fair, Poor
	Strengths   []string `json:"strengths"`   // Kuchli tomonlar
	Weaknesses  []string `json:"weaknesses"`  // Zaif tomonlar
}

// GetTotalPrice umumiy narxni hisoblash
func (pc *PCBuild) GetTotalPrice() float64 {
	total := 0.0

	total += pc.CPU.Price
	total += pc.GPU.Price
	total += pc.RAM.Price
	total += pc.SSD.Price
	total += pc.Motherboard.Price
	total += pc.PSU.Price

	if pc.HDD != nil {
		total += pc.HDD.Price
	}
	if pc.Case != nil {
		total += pc.Case.Price
	}
	if pc.Cooler != nil {
		total += pc.Cooler.Price
	}
	if pc.Monitor != nil {
		total += pc.Monitor.Price
	}

	return total
}

// IsComplete barcha majburiy komponentlar to'liqmi?
func (pc *PCBuild) IsComplete() bool {
	return pc.CPU.Name != "" &&
		pc.GPU.Name != "" &&
		pc.RAM.Name != "" &&
		pc.SSD.Name != "" &&
		pc.Motherboard.Name != "" &&
		pc.PSU.Name != ""
}

// GetComponentList barcha komponentlarni string sifatida
func (pc *PCBuild) GetComponentList() []string {
	components := []string{}

	if pc.CPU.Name != "" {
		components = append(components, "CPU: "+pc.CPU.Name)
	}
	if pc.GPU.Name != "" {
		components = append(components, "GPU: "+pc.GPU.Name)
	}
	if pc.RAM.Name != "" {
		components = append(components, "RAM: "+pc.RAM.Name)
	}
	if pc.SSD.Name != "" {
		components = append(components, "SSD: "+pc.SSD.Name)
	}
	if pc.HDD != nil && pc.HDD.Name != "" {
		components = append(components, "HDD: "+pc.HDD.Name)
	}
	if pc.Motherboard.Name != "" {
		components = append(components, "Motherboard: "+pc.Motherboard.Name)
	}
	if pc.PSU.Name != "" {
		components = append(components, "PSU: "+pc.PSU.Name)
	}
	if pc.Case != nil && pc.Case.Name != "" {
		components = append(components, "Case: "+pc.Case.Name)
	}
	if pc.Cooler != nil && pc.Cooler.Name != "" {
		components = append(components, "Cooler: "+pc.Cooler.Name)
	}
	if pc.Monitor != nil && pc.Monitor.Name != "" {
		components = append(components, "Monitor: "+pc.Monitor.Name)
	}

	return components
}
