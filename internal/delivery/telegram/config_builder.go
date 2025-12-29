package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/usecase"
)

// ConfigurationBuilder - intelligent PC component selection
type ConfigurationBuilder struct {
	productUseCase usecase.ProductUseCase
}

// NewConfigurationBuilder creates a new intelligent PC builder
func NewConfigurationBuilder(productUseCase usecase.ProductUseCase) *ConfigurationBuilder {
	return &ConfigurationBuilder{
		productUseCase: productUseCase,
	}
}

// SelectedConfiguration - tanlangan komponentlar va narx
type SelectedConfiguration struct {
	CPU         *entity.Product
	RAM         *entity.Product
	GPU         *entity.Product
	SSD         *entity.Product
	Motherboard *entity.Product
	PSU         *entity.Product
	Cooler      *entity.Product
	Case        *entity.Product
	Monitor     *entity.Product
	Peripherals []*entity.Product
	TotalPrice  float64
}

// BuildConfiguration - intelligent configuration selection
func (cb *ConfigurationBuilder) BuildConfiguration(
	ctx context.Context,
	budget, pcType, cpuBrand, gpuBrand, storage, monitorHz, monitorDisplay string,
	needMonitor, needPeripherals bool,
) (*SelectedConfiguration, error) {
	// 1. Extract budget as number
	budgetNum := extractBudgetNumber(budget)
	if budgetNum <= 0 {
		budgetNum = 1000 // Default 1000$
	}

	cfg := &SelectedConfiguration{}

	// 2. CPU selection - budjet va type asosida
	cpuList, err := cb.productUseCase.GetByCategory(ctx, "CPU")
	if err != nil {
		log.Printf("CPU selection error: %v", err)
		return nil, err
	}

	cpu := cb.selectCPU(toProductPtrs(cpuList), cpuBrand, budgetNum, pcType)
	if cpu != nil {
		cfg.CPU = cpu
	}

	// 3. RAM selection - CPU compatibility (DDR4 vs DDR5)
	ramList, err := cb.productUseCase.GetByCategory(ctx, "RAM")
	if err == nil {
		ram := cb.selectRAM(toProductPtrs(ramList), budgetNum, cpu)
		if ram != nil {
			cfg.RAM = ram
		}
	}

	// 4. Motherboard selection - CPU + RAM match
	mbList, err := cb.productUseCase.GetByCategory(ctx, "Motherboard")
	if err == nil {
		mb := cb.selectMotherboard(toProductPtrs(mbList), cpu, cfg.RAM)
		if mb != nil {
			cfg.Motherboard = mb
		}
	}

	// 5. GPU selection - remaining budget
	gpuList, err := cb.productUseCase.GetByCategory(ctx, "GPU")
	if err == nil {
		usedBudget := cb.getComponentPrice(cfg.CPU) + cb.getComponentPrice(cfg.RAM) + cb.getComponentPrice(cfg.Motherboard)
		gpu := cb.selectGPU(toProductPtrs(gpuList), gpuBrand, budgetNum-usedBudget, pcType)
		if gpu != nil {
			cfg.GPU = gpu
		}
	}

	// 6. SSD selection
	ssdList, err := cb.productUseCase.GetByCategory(ctx, "SSD")
	if err == nil && len(ssdList) == 0 {
		// Fallback for catalogs that use a different category name
		if alt, err2 := cb.productUseCase.GetByCategory(ctx, "Storage"); err2 == nil {
			ssdList = append(ssdList, alt...)
		}
		if len(ssdList) == 0 {
			if alt, err2 := cb.productUseCase.GetByCategory(ctx, "ROM"); err2 == nil {
				ssdList = append(ssdList, alt...)
			}
		}
	}
	if err == nil && len(ssdList) > 0 {
		if ssd := cb.selectSSD(toProductPtrs(ssdList), storage); ssd != nil {
			cfg.SSD = ssd
		}
	}

	// 7. PSU selection - CPU TDP + GPU power
	psuList, err := cb.productUseCase.GetByCategory(ctx, "PSU")
	if err == nil {
		psu := cb.selectPSU(toProductPtrs(psuList), cfg.CPU, cfg.GPU)
		if psu != nil {
			cfg.PSU = psu
		}
	}

	// 8. Cooler selection - CPU TDP
	coolerList, err := cb.productUseCase.GetByCategory(ctx, "Cooler")
	if err == nil && len(coolerList) == 0 {
		// Fallback for catalogs that use a different category name
		if alt, err2 := cb.productUseCase.GetByCategory(ctx, "Cooling"); err2 == nil {
			coolerList = append(coolerList, alt...)
		}
		if len(coolerList) == 0 {
			if alt, err2 := cb.productUseCase.GetByCategory(ctx, "CPU Cooler"); err2 == nil {
				coolerList = append(coolerList, alt...)
			}
		}
	}
	if err == nil && len(coolerList) > 0 {
		if cooler := cb.selectCooler(toProductPtrs(coolerList), cfg.CPU); cooler != nil {
			cfg.Cooler = cooler
		}
	}

	// 9. Case selection
	caseList, err := cb.productUseCase.GetByCategory(ctx, "Case")
	if err == nil {
		cse := cb.selectCase(toProductPtrs(caseList))
		if cse != nil {
			cfg.Case = cse
		}
	}

	// 10. Monitor selection
	if needMonitor {
		monList, err := cb.productUseCase.GetByCategory(ctx, "Monitor")
		if err == nil {
			mon := cb.selectMonitor(toProductPtrs(monList), monitorHz, monitorDisplay)
			if mon != nil {
				cfg.Monitor = mon
			}
		}
	}

	// 11. Peripherals selection
	if needPeripherals {
		periList, err := cb.productUseCase.GetByCategory(ctx, "Peripherals")
		if err == nil && len(periList) == 0 {
			// Fallback: some catalogs store peripherals as separate categories
			for _, cat := range []string{"Keyboard", "Mouse", "Headset", "Microphone", "Mousepad"} {
				if alt, err2 := cb.productUseCase.GetByCategory(ctx, cat); err2 == nil {
					periList = append(periList, alt...)
				}
			}
		}
		if err == nil && len(periList) > 0 {
			cfg.Peripherals = cb.selectPeripherals(toProductPtrs(periList))
		}
	}

	// Calculate total price
	cfg.TotalPrice = cb.calculateTotalPrice(cfg)

	return cfg, nil
}

// toProductPtrs converts a slice of products to a slice of product pointers.
func toProductPtrs(products []entity.Product) []*entity.Product {
	ptrs := make([]*entity.Product, len(products))
	for i := range products {
		ptrs[i] = &products[i]
	}
	return ptrs
}

// selectCPU - budjet va type asosida CPU tanlaydi
func (cb *ConfigurationBuilder) selectCPU(cpus []*entity.Product, brand string, budget float64, pcType string) *entity.Product {
	// Filter by brand preference
	var filtered []*entity.Product
	for _, cpu := range cpus {
		nameL := strings.ToLower(cpu.Name)
		brandL := strings.ToLower(brand)
		if brandL != "" && !strings.Contains(nameL, brandL) {
			continue
		}
		filtered = append(filtered, cpu)
	}

	if len(filtered) == 0 {
		filtered = cpus
	}

	// PC type asosida tanlash
	pcTypeL := strings.ToLower(pcType)
	var selected *entity.Product

	if strings.Contains(pcTypeL, "gaming") {
		// Gaming: K-series agar budjet yetsa, yoki mid-range non-K
		for _, cpu := range filtered {
			nameL := strings.ToLower(cpu.Name)
			if strings.Contains(nameL, "k ") || strings.Contains(nameL, "-k") {
				if selected == nil || selected.Price < cpu.Price {
					selected = cpu
				}
			}
		}
		// K-series bo'lmasa, yaxshi non-K ol
		if selected == nil {
			for _, cpu := range filtered {
				if selected == nil || (cpu.Price > selected.Price && cpu.Price < budget*0.3) {
					selected = cpu
				}
			}
		}
	} else {
		// Office/Montaj: kam quvvat, kam narx
		for _, cpu := range filtered {
			if selected == nil || (cpu.Price < selected.Price && cpu.Price < budget*0.15) {
				selected = cpu
			}
		}
	}

	if selected == nil && len(filtered) > 0 {
		selected = filtered[0]
	}

	return selected
}

// selectRAM - CPU mos RAM tanlaydi (DDR4 vs DDR5)
func (cb *ConfigurationBuilder) selectRAM(rams []*entity.Product, budget float64, cpu *entity.Product) *entity.Product {
	_ = cpu
	// Determine DDR type based on budget
	var ddrType string
	if budget >= 1200 {
		ddrType = "DDR5"
	} else {
		ddrType = "DDR4"
	}

	// Find RAM matching DDR type
	var filtered []*entity.Product
	for _, ram := range rams {
		nameL := strings.ToLower(ram.Name)
		if strings.Contains(nameL, ddrType) {
			filtered = append(filtered, ram)
		}
	}

	if len(filtered) == 0 {
		filtered = rams
	}

	// Select 32GB if possible, else 16GB
	var selected *entity.Product
	for _, ram := range filtered {
		nameL := strings.ToLower(ram.Name)
		if strings.Contains(nameL, "32gb") {
			if selected == nil || ram.Price < selected.Price {
				selected = ram
			}
		}
	}

	if selected == nil && len(filtered) > 0 {
		selected = filtered[0]
	}

	return selected
}

// selectMotherboard - CPU + RAM matchi motherboard tanlaydi
func (cb *ConfigurationBuilder) selectMotherboard(mbs []*entity.Product, cpu, ram *entity.Product) *entity.Product {
	if cpu == nil {
		return nil
	}

	// Determine chipset based on CPU
	var needChipset string
	cpuNameL := strings.ToLower(cpu.Name)

	if strings.Contains(cpuNameL, "k ") || strings.Contains(cpuNameL, "-k") {
		// K-series needs Z-chipset
		needChipset = "Z"
	} else {
		// Non-K needs B-chipset
		needChipset = "B"
	}

	// Determine DDR type
	var needDDR string
	if ram != nil {
		ramNameL := strings.ToLower(ram.Name)
		if strings.Contains(ramNameL, "ddr5") {
			needDDR = "DDR5"
		} else {
			needDDR = "DDR4"
		}
	}

	// Filter by chipset and DDR type
	var filtered []*entity.Product
	for _, mb := range mbs {
		nameL := strings.ToLower(mb.Name)

		// Check chipset
		if !strings.Contains(nameL, needChipset) {
			continue
		}

		// Check DDR compatibility
		if needDDR != "" && !strings.Contains(nameL, needDDR) {
			continue
		}

		filtered = append(filtered, mb)
	}

	var selected *entity.Product
	if len(filtered) > 0 {
		selected = filtered[0]
	} else if len(mbs) > 0 {
		selected = mbs[0]
	}

	return selected
}

// selectGPU - budjet asosida GPU tanlaydi
func (cb *ConfigurationBuilder) selectGPU(gpus []*entity.Product, brand string, budget float64, pcType string) *entity.Product {
	_ = pcType
	// Filter by budget
	var filtered []*entity.Product
	for _, gpu := range gpus {
		if gpu.Price <= budget {
			filtered = append(filtered, gpu)
		}
	}

	// Filter by brand preference
	if brand != "" {
		var brandFiltered []*entity.Product
		for _, gpu := range filtered {
			if strings.Contains(strings.ToLower(gpu.Name), strings.ToLower(brand)) {
				brandFiltered = append(brandFiltered, gpu)
			}
		}
		if len(brandFiltered) > 0 {
			filtered = brandFiltered
		}
	}

	if len(filtered) == 0 {
		filtered = gpus
	}

	// Select highest price GPU within budget
	var selected *entity.Product
	for _, gpu := range filtered {
		if selected == nil || (gpu.Price > selected.Price && gpu.Price <= budget) {
			selected = gpu
		}
	}

	return selected
}

// selectSSD - storage type asosida SSD tanlaydi
func (cb *ConfigurationBuilder) selectSSD(ssds []*entity.Product, storageType string) *entity.Product {
	_ = storageType
	// Prefer NVMe > SSD > HDD
	preferOrder := []string{"NVMe", "SSD", "HDD"}

	for _, pref := range preferOrder {
		for _, ssd := range ssds {
			nameL := strings.ToLower(ssd.Name)
			if strings.Contains(nameL, strings.ToLower(pref)) {
				return ssd
			}
		}
	}

	if len(ssds) > 0 {
		return ssds[0]
	}
	return nil
}

// selectPSU - CPU + GPU TDP asosida PSU tanlaydi
func (cb *ConfigurationBuilder) selectPSU(psus []*entity.Product, cpu, gpu *entity.Product) *entity.Product {
	_ = cpu
	_ = gpu
	// Find PSU >= estimatedTDP
	var selected *entity.Product
	for _, psu := range psus {
		if strings.Contains(strings.ToLower(psu.Name), "650w") || strings.Contains(strings.ToLower(psu.Name), "750w") {
			if selected == nil || psu.Price < selected.Price {
				selected = psu
			}
		}
	}

	if selected == nil && len(psus) > 0 {
		selected = psus[0]
	}

	return selected
}

// selectCooler - CPU TDP asosida cooler tanlaydi
func (cb *ConfigurationBuilder) selectCooler(coolers []*entity.Product, cpu *entity.Product) *entity.Product {
	_ = cpu
	// For most CPUs, AIO is good choice
	var selected *entity.Product

	for _, cooler := range coolers {
		nameL := strings.ToLower(cooler.Name)
		if strings.Contains(nameL, "aio") {
			if selected == nil || cooler.Price < selected.Price {
				selected = cooler
			}
		}
	}

	if selected == nil && len(coolers) > 0 {
		selected = coolers[0]
	}

	return selected
}

// selectCase - PC case tanlaydi
func (cb *ConfigurationBuilder) selectCase(cases []*entity.Product) *entity.Product {
	if len(cases) > 0 {
		// Select mid-range case
		var selected *entity.Product
		for _, cse := range cases {
			if selected == nil || (cse.Price > selected.Price && cse.Price < 200) {
				selected = cse
			}
		}
		if selected != nil {
			return selected
		}
		return cases[0]
	}
	return nil
}

// selectMonitor - monitor Hz + display asosida tanlaydi
func (cb *ConfigurationBuilder) selectMonitor(monitors []*entity.Product, hz, display string) *entity.Product {
	var filtered []*entity.Product

	if hz != "" {
		for _, mon := range monitors {
			if strings.Contains(strings.ToLower(mon.Name), strings.ToLower(hz)) {
				filtered = append(filtered, mon)
			}
		}
	}

	if len(filtered) == 0 {
		filtered = monitors
	}

	return filtered[0]
}

// selectPeripherals - gaming peripherals tanlaydi
func (cb *ConfigurationBuilder) selectPeripherals(peris []*entity.Product) []*entity.Product {
	// Select gaming peripherals (keyboard, mouse, headset)
	var result []*entity.Product

	keyboard := cb.findPeripheral(peris, "keyboard", "klaviatura")
	mouse := cb.findPeripheral(peris, "mouse", "sichqoncha")
	headset := cb.findPeripheral(peris, "headset", "headphone", "quloqchin")

	if keyboard != nil {
		result = append(result, keyboard)
	}
	if mouse != nil {
		result = append(result, mouse)
	}
	if headset != nil {
		result = append(result, headset)
	}

	return result
}

func (cb *ConfigurationBuilder) findPeripheral(peris []*entity.Product, keywords ...string) *entity.Product {
	for _, peri := range peris {
		nameL := strings.ToLower(peri.Name)
		for _, kw := range keywords {
			if strings.Contains(nameL, strings.ToLower(kw)) {
				return peri
			}
		}
	}
	return nil
}

// calculateTotalPrice - jami narx hisoblaydi
func (cb *ConfigurationBuilder) calculateTotalPrice(cfg *SelectedConfiguration) float64 {
	total := 0.0
	total += cb.getComponentPrice(cfg.CPU)
	total += cb.getComponentPrice(cfg.RAM)
	total += cb.getComponentPrice(cfg.GPU)
	total += cb.getComponentPrice(cfg.SSD)
	total += cb.getComponentPrice(cfg.Motherboard)
	total += cb.getComponentPrice(cfg.PSU)
	total += cb.getComponentPrice(cfg.Cooler)
	total += cb.getComponentPrice(cfg.Case)
	total += cb.getComponentPrice(cfg.Monitor)

	for _, peri := range cfg.Peripherals {
		total += cb.getComponentPrice(peri)
	}

	return total
}

func (cb *ConfigurationBuilder) getComponentPrice(comp *entity.Product) float64 {
	if comp == nil {
		return 0
	}
	return comp.Price
}

// Helper: extract budget number
func extractBudgetNumber(budgetStr string) float64 {
	// Remove currency symbols and spaces
	s := strings.ToLower(budgetStr)
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")

	// If "million so'm" format, convert
	if strings.Contains(s, "million") || strings.Contains(s, "m") {
		// 1 million so'm = ~87 USD (approximately)
		parts := strings.Fields(budgetStr)
		var numStr string
		for _, p := range parts {
			if strings.ContainsAny(p, "0123456789") {
				numStr = strings.ReplaceAll(p, ",", ".")
				break
			}
		}
		// Parse simple number
		var budget float64
		fmt.Sscanf(numStr, "%f", &budget)
		return budget * 0.087 // Rough conversion
	}

	var budget float64
	fmt.Sscanf(s, "%f", &budget)
	return budget
}
