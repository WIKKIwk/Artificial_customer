package usecase

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/telegram-ai-bot/internal/domain/constants"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/domain/repository"
)

// ChatUseCase chat bilan bog'liq business logic
type ChatUseCase interface {
	ProcessMessage(ctx context.Context, userID int64, username, text string) (string, error)
	ProcessConfigMessage(ctx context.Context, userID int64, username, text string) (string, error)
	ClearHistory(ctx context.Context, userID int64) error
	GetHistory(ctx context.Context, userID int64) ([]entity.Message, error)
}

type chatUseCase struct {
	aiRepo      repository.AIRepository
	chatRepo    repository.ChatRepository
	productRepo repository.ProductRepository
}

var (
	reIntelCoreShortModel = regexp.MustCompile(`\b(?:core\s*)?i\s*[3579]\b`)
	reAlphaNumToken       = regexp.MustCompile(`[a-z0-9]+`)
	reAlphaToken          = regexp.MustCompile(`[a-zа-яё]+`)
	reTotalLine           = regexp.MustCompile(`(?i)^(?:jami|итого|summa|total|overall\s*(?:price|total)?|umumiy\s*(?:narx|summa)?|общая\s*(?:цена|стоимость)?|всего)(?:\s|:|-|$)`)
	rePriceWithCurrency   = regexp.MustCompile(`(?i)(?:(?:\$|usd|so['’]?m|sum|сум|eur|€|rub|₽)\s*[0-9][0-9\s,.]*|[0-9][0-9\s,.]*\s*(?:\$|usd|so['’]?m|sum|сум|eur|€|rub|₽))`)
)

// NewChatUseCase yangi ChatUseCase yaratish
func NewChatUseCase(
	aiRepo repository.AIRepository,
	chatRepo repository.ChatRepository,
	productRepo repository.ProductRepository,
) ChatUseCase {
	return &chatUseCase{
		aiRepo:      aiRepo,
		chatRepo:    chatRepo,
		productRepo: productRepo,
	}
}

// ProcessMessage foydalanuvchi xabarini qayta ishlash
func (u *chatUseCase) ProcessMessage(ctx context.Context, userID int64, username, text string) (string, error) {
	// Oldingi tarixni olish (oxirgi 10 ta xabar)
	history, err := u.chatRepo.GetHistory(ctx, userID, 20)
	if err != nil {
		return "", fmt.Errorf("failed to get history: %w", err)
	}

	// CSV ma'lumotlarini olish
	csvData, csvFilename, csvErr := u.productRepo.GetCSV(ctx)
	availableCSV := ""
	if csvErr == nil && csvData != "" {
		availableCSV = filterProductsByStock(csvData)
	}
	hasCSV := csvErr == nil && strings.TrimSpace(availableCSV) != ""

	// Foydalanuvchi xabariga mahsulot katalogini qo'shish
	enrichedText := text
	includePhone := needsPhoneContact(text)
	forceRussian := strings.HasPrefix(strings.TrimSpace(text), "Отвечай только на русском языке.")
	budgetFromText := extractBudgetFromText(text)
	budget := budgetFromText
	budgetExplicit := budgetFromText > 0
	purpose := extractPurposeFromText(text)
	requestedCategory := detectRequestedCategory(text)
	isSearch := isProductInquiry(text)
	modelTokens := extractModelTokens(text)
	brandTokens := extractBrandTokens(text)
	queryTokens := extractQueryTokens(text)
	enableFastMatch := false

	saveAndReturn := func(resp string) (string, error) {
		message := entity.Message{
			ID:        uuid.New().String(),
			UserID:    userID,
			Username:  username,
			Text:      text,
			Response:  resp,
			Timestamp: time.Now(),
		}
		if err := u.chatRepo.SaveMessage(ctx, message); err != nil {
			log.Printf("failed to save message: %v", err)
		}
		return resp, nil
	}

	// Missing kontekstlarni tarixdan topamiz (oxirgi 5 ta user xabar).
	// Faqat mahsulot so'rovida ishlatamiz (oddiy chatga eski budjet aralashib ketmasin).
	if isSearch && len(history) > 0 {
		for i := len(history) - 1; i >= 0 && i >= len(history)-5; i-- {
			histText := history[i].Text
			histCategory := detectRequestedCategory(histText)

			// Budjetni tarixdan faqat follow-up holatlarda ko'chiramiz:
			// - user aniq model yozmagan bo'lsa (i5/13400f/rtx4070...)
			// - va kategoriya mos bo'lsa (yoki hozir kategoriya aniqlanmagan bo'lsa)
			if budget == 0 && !budgetExplicit && len(modelTokens) == 0 {
				if requestedCategory == "" || (histCategory != "" && strings.EqualFold(histCategory, requestedCategory)) {
					if b := extractBudgetFromText(histText); b > 0 {
						budget = b
					}
				}
			}
			if purpose == "" {
				if p := extractPurposeFromText(histText); p != "" {
					purpose = p
				}
			}
			if requestedCategory == "" {
				if histCategory != "" {
					requestedCategory = histCategory
				}
			}
			if len(brandTokens) == 0 {
				if histBrands := extractBrandTokens(histText); len(histBrands) > 0 {
					if requestedCategory == "" || (histCategory != "" && strings.EqualFold(histCategory, requestedCategory)) {
						brandTokens = histBrands
					}
				}
			}
			if budget > 0 && purpose != "" && requestedCategory != "" {
				break
			}
		}
	}

	// Mahsulot qidiruvda doim budjet va maqsadni aniqlab olamiz (AI ga og'ir bo'lmasin).
	// Kategoriya so'rashni AI'ga qoldiramiz — bot statik savol qaytarmaydi.

	if hasCSV {
		log.Printf("💰 [DEBUG] budget=%d, purpose=%q, category=%q, inquiry=%v, text=%q", budget, purpose, requestedCategory, isSearch, text)

		// MUAMMOLI KOD - OLIB TASHLANDI!
		// Budjet so'rash AI ga yuboriladi, shu yerda return qilmaymiz
		// AI o'zi do'stona tarzda budjet so'raydi

		brandTokens = normalizeBrandTokens(brandTokens)
		filteredCSV := availableCSV
		if strings.TrimSpace(requestedCategory) != "" {
			filteredCSV = filterProductsByCategory(availableCSV, requestedCategory)
		}
		if budget > 0 {
			// Agar budjet mavjud bo'lsa, faqat budjet doirasidagi mahsulotlarni ko'rsat
			if requestedCategory != "" {
				filteredCSV = filterProductsByBudgetAndCategory(availableCSV, budget, requestedCategory)
				if strings.TrimSpace(filteredCSV) == "" {
					filteredCSV = filterProductsByBudget(availableCSV, budget)
				}
			} else {
				filteredCSV = filterProductsByBudget(availableCSV, budget)
			}
			log.Printf("📊 Budget filter qo'llandi: %d$ (cat=%q) → %d bytes (original: %d bytes)\n📝 User text: %s\n📋 Filtered CSV sample (first 500 chars): %.500s", budget, requestedCategory, len(filteredCSV), len(availableCSV), text, filteredCSV)

			// Yangi: Foydalanuvchi xabarida aniq model so'ralgan va u budjetdan qimmatmi?
			lines := strings.Split(availableCSV, "\n")
			overBudgetProducts := make(map[string]float64)
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				name, price, ok := parseCatalogLineNameAndPrice(line)
				if ok && name != "" && price > float64(budget) {
					overBudgetProducts[strings.ToLower(name)] = price
				}
			}
			log.Printf("[DEBUG] Over-budget products detected: %+v", overBudgetProducts)

			// Foydalanuvchi xabarida qimmat model so'ralganmi? (improved matching)
			lowerText := strings.ToLower(text)
			for prodName, prodPrice := range overBudgetProducts {
				if prodName != "" && (strings.Contains(lowerText, prodName) || fuzzyMatch(lowerText, prodName)) {
					log.Printf("[DEBUG] User requested over-budget product: %s (%.0f$ > %d$)", prodName, prodPrice, budget)
					msg := fmt.Sprintf("Kechirasiz, '%s' modeli sizning budjetingizdan (%d$) qimmat: %.0f$.\nIltimos, budjetga mos variantlardan birini tanlang yoki budjetni oshiring.\n\nBudjetga mos eng yaxshi variantlarni taklif qilaman:", prodName, budget, prodPrice)
					if requestedCategory != "" {
						filteredCSV = filterProductsByBudgetAndCategory(availableCSV, budget, requestedCategory)
						if strings.TrimSpace(filteredCSV) == "" {
							filteredCSV = filterProductsByBudget(availableCSV, budget)
						}
					} else {
						filteredCSV = filterProductsByBudget(availableCSV, budget)
					}
					top := getTopProductsFromCSV(filteredCSV, 5)
					resp := msg
					if len(top) > 0 {
						resp += "\n\n" + strings.Join(top, "\n")
					}
					return saveAndReturn(resp)
				}
			}

			// Fuzzy match helper
			// (add this function at the end of the file)
		} else {
			log.Printf("⚠️ Budget NOT extracted from: %s", text)
		}

		brandFilteredCSV := ""
		if len(brandTokens) > 0 {
			brandFilteredCSV = filterProductsByBrand(filteredCSV, brandTokens)
			if strings.TrimSpace(brandFilteredCSV) != "" {
				filteredCSV = brandFilteredCSV
			}
		}

		// Fast path: aniq model/brend nomi ko'rsatilganda CSV'dan o'zimiz topib beramiz.
		if isSearch && enableFastMatch {
			matchTokens := mergeTokens(append(brandTokens, queryTokens...), modelTokens, 4)
			if len(matchTokens) > 0 {
				matches := findMatchingProductsFromCSVNumbered(filteredCSV, matchTokens, 5)
				// Agar kategoriya/budjet filtr noto'g'ri bo'lsa (budjet explicit emas) full CSV'dan ham tekshiramiz.
				if len(matches) == 0 && !budgetExplicit && filteredCSV != availableCSV {
					matches = findMatchingProductsFromCSVNumbered(availableCSV, matchTokens, 5)
				}

				log.Printf("🔎 [FastMatch] userID=%d, tokens=%v, category=%q, budget=%d (explicit=%v), matches=%d",
					userID, matchTokens, requestedCategory, budget, budgetExplicit, len(matches))

				if len(matches) > 0 {
					labelTokens := modelTokens
					if len(labelTokens) == 0 {
						labelTokens = matchTokens
					}
					label := strings.Join(labelTokens, " ")
					if len(matches) == 1 {
						if forceRussian {
							resp := fmt.Sprintf("В наличии: %s\n\nЧтобы купить, отправьте 1.", matches[0])
							return saveAndReturn(resp)
						}
						resp := fmt.Sprintf("Bizda bor: %s\n\nSotib olish uchun 1 ni yuboring.", matches[0])
						return saveAndReturn(resp)
					}
					if forceRussian {
						resp := fmt.Sprintf("%s: доступные варианты:\n\n%s\n\nКакой выберете? Напишите номер (1-%d).",
							strings.ToUpper(label), strings.Join(matches, "\n"), len(matches))
						return saveAndReturn(resp)
					}
					resp := fmt.Sprintf("%s bo'yicha mavjud variantlar:\n\n%s\n\nQaysi birini tanlaysiz? Raqamini yozing (1-%d).",
						strings.ToUpper(label), strings.Join(matches, "\n"), len(matches))
					return saveAndReturn(resp)
				}

				// Aniq so'rov bo'lsa va topilmasa — qisqa javob qaytaramiz.
				if forceRussian {
					return saveAndReturn(fmt.Sprintf("Извините, по запросу '%s' ничего не найдено.", strings.Join(matchTokens, " ")))
				}
				return saveAndReturn(fmt.Sprintf("Kechirasiz, '%s' bo'yicha mahsulot topilmadi.", strings.Join(matchTokens, " ")))
			}
		}

		// CSV ma'lumotlaridan foydalanish - BUDGET SHARTINI TAQDIM QILISH
		// IMPORTANT: Use filteredCSV (budget-filtered) instead of csvData
		// Agar budjet eng arzon mahsulotdan ham past bo'lsa, eng yaqin variantlarni ko'rsatamiz
		if budget > 0 && strings.TrimSpace(filteredCSV) == "" {
			cheapest, minPrice := getCheapestProducts(availableCSV, 5)
			if len(cheapest) > 0 {
				notice := fmt.Sprintf("⚠️ Budjetingiz eng arzon mahsulotdan past (min: %.2f$). Eng yaqin variantlar:", minPrice)
				filteredCSV = notice + "\n" + strings.Join(cheapest, "\n")
			}
		}

		enrichedText = fmt.Sprintf(`%s

	%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📦 SIZNING DO'KONINGIZDA MAVJUD MAHSULOTLAR (CSV FORMAT):
Fayl: %s

%s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

		📋 QOIDALAR:
			• CSV dan ANIQ narx va nomlar ol
			• Har variant: [NOM] - [NARX]$
			• Javobni 1-2 qisqa do'stona gap bilan boshlang; "X uchun 5 ta variant" shablonini ishlatma
			• 3-5 variant ko'rsat (imkon bo'lsa 5 ta) va raqamla (1-5)
			• Agar budjet berilgan bo'lsa: budjetdan ortiq ko'rsatma va eng yaqin variantlarni yuqoriga qo'y
			• Agar budjet berilmagan bo'lsa: turli narx oralig'idan 5 ta variant ber (arzon→qimmat) va oxirida budjetni so'rab qo'y (majburiy emas)
			• Agar maqsad berilgan bo'lsa, maqsadga mos tanla; bo'lmasa 1 ta qisqa savol bilan aniqlashtir (majburiy emas)
			• Agar brend ko'rsatilgan bo'lsa, faqat shu brenddan tanla (topilmasa "mavjud emas" deb ayt)
			• Agar so'ralgan mahsulot ro'yxatda bo'lmasa yoki omborda 0 bo'lsa, "mavjud emas" deb ayt

			Mijozga javob ber:`, text,
			func() string {
				var sb strings.Builder
				if strings.TrimSpace(requestedCategory) != "" {
					sb.WriteString("\n🛍️ Mahsulot turi: " + requestedCategory)
				}
				if len(brandTokens) > 0 {
					sb.WriteString("\n🏷️ Brend: " + strings.Join(brandTokens, ", "))
				}
				if strings.TrimSpace(purpose) != "" {
					sb.WriteString("\n🎯 Maqsad: " + purpose)
				} else if isSearch {
					sb.WriteString("\n🎯 Maqsad: (aniqlanmagan)")
				}
				if budget > 0 {
					sb.WriteString(fmt.Sprintf("\n💰 Budjet: %d$", budget))
				} else if isSearch {
					sb.WriteString("\n💰 Budjet: (aniqlanmagan)")
				}
				return sb.String()
			}(), csvFilename, filteredCSV)
		if includePhone {
			enrichedText += "\nAdmin telefon raqami (aloqa uchun): " + constants.AdminContactPhone
		}

		// CSV ma'lumot yuborilmoqda (debug)
		fmt.Printf("📦 CSV Katalog yuborildi: %s (%d bytes, budget filter: %d$)\n", csvFilename, len(filteredCSV), budget)
	} else {
		// Fallback: Eski usul - product list dan
		products, err := u.productRepo.GetAll(ctx)
		availableProducts := filterInStockProducts(products)
		hasProducts := err == nil && len(availableProducts) > 0

		if hasProducts {
			// HAR DOIM mahsulot ma'lumotini AI ga yuborish
			productsInfo := u.buildProductsContext(availableProducts)
			includePhone := needsPhoneContact(text)
			enrichedText = fmt.Sprintf(`Mijoz: %s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📦 SIZNING DO'KONINGIZDA MAVJUD MAHSULOTLAR:
%s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	📋 QOIDALAR:
	• Ro'yxatdan ANIQ narx va nomlar ol
	• Budjetdan ortiq ko'rsatma
	• Javobni 1-2 qisqa do'stona gap bilan boshlang; kerak bo'lsa 1 qisqa savol ber, keyin 3-5 variant ko'rsat (imkon bo'lsa 5 ta)
	• Har variant: [NOM] - [NARX]$
	• Agar brend ko'rsatilgan bo'lsa, faqat shu brenddan tanla (topilmasa "mavjud emas" deb ayt)
	• Agar so'ralgan mahsulot ro'yxatda bo'lmasa yoki omborda 0 bo'lsa, "mavjud emas" deb ayt
	• Agar budjet bo'lsa, eng yaqin (budjetga yaqin) variantlarni yuqoriga qo'y

	Mijozga javob ber:`, text, productsInfo)
			if includePhone {
				enrichedText += "\nAdmin telefon raqami (aloqa uchun): " + constants.AdminContactPhone
			}

			// Product katalog yuborilmoqda (debug)
			fmt.Printf("📦 Product ro'yxat yuborildi (%d mahsulot)\n", len(availableProducts))
		}
	}

	// AI dan javob olish
	response, err := u.aiRepo.GenerateResponseWithHistory(ctx, userID, enrichedText, history)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	// ✅ POST-PROCESSING: AI javobini validatsiya qilish va noto'g'ri narxlarni to'g'rilash
	if hasCSV {
		response = u.validateAndFixPrices(response, availableCSV)
		response = syncTotalLineWithSinglePrice(response)

		// The 'budget' variable is already correctly scoped from the top of the function.
		// No need to re-extract it here.

		if budget > 0 {
			// Validate that the AI response only contains products that are within the budget.
			filteredCSV := filterProductsByBudget(availableCSV, budget)
			validationResult := validateProductsExistInCSV(response, filteredCSV, budget)

			// If the validation fails (i.e., the AI recommended an over-budget product),
			// discard the AI's response and generate a safe, template-based one.
			if validationResult != "" {
				log.Println("[VALIDATION] AI response discarded due to over-budget/invalid products.")

				// Get the top 5 products from the correctly filtered CSV.
				topProducts := getTopProductsFromCSV(filteredCSV, 5)

				if len(topProducts) > 0 {
					response = "Kechirasiz, AI tavsiyasida xatolik bo'ldi. Sizning budjetingizga mos keladigan eng yaxshi variantlar:\n\n" + strings.Join(topProducts, "\n")
				} else {
					response = "Kechirasiz, sizning budjetingizga mos mahsulot topilmadi. Boshqa budjet bilan harakat qilib ko'ring."
				}
			}
		}
	}

	// Agar AI 1-2 ta variant bilan cheklanib qolsa, CSV'dan 3-5 ta (imkon bo'lsa 5) budjetga yaqin variantlarni beramiz.
	if hasCSV && isProductInquiry(text) && strings.TrimSpace(requestedCategory) != "" {
		respLower := strings.ToLower(response)
		if strings.Contains(respLower, "jami:") || strings.Contains(respLower, "итого:") {
			// Checkout / savatcha matnlariga tegmaymiz.
		} else if budget > 0 && strings.TrimSpace(purpose) != "" && strings.Contains(response, "$") {
			variantCount := countPricedVariants(response)
			if variantCount > 0 && variantCount < 3 {
				filtered := filterProductsByBudgetAndCategory(availableCSV, budget, requestedCategory)
				if strings.TrimSpace(filtered) == "" {
					filtered = filterProductsByBudget(availableCSV, budget)
				}
				if len(brandTokens) > 0 {
					brandFiltered := filterProductsByBrand(filtered, brandTokens)
					if strings.TrimSpace(brandFiltered) != "" {
						filtered = brandFiltered
					}
				}
				top := getTopProductsFromCSVNumbered(filtered, 5)
				if len(top) >= 3 {
					label := strings.TrimSpace(requestedCategory)
					if len(brandTokens) > 0 {
						label = strings.TrimSpace(strings.ToUpper(strings.Join(brandTokens, " ")) + " " + label)
					}
					response = fmt.Sprintf("%d$ budjetga %s uchun eng yaqin variantlar:\n\n%s\n\nQaysi birini tanlaysiz? Raqamini yozing (1-%d).",
						budget, label, strings.Join(top, "\n"), len(top))
				}
			}
		} else if budget == 0 {
			// Budjet kiritilmagan bo'lsa ham majburlamasdan, 5 ta variant berib yuboramiz.
			variantCount := countPricedVariants(response)
			askedBudget := strings.Contains(respLower, "budjet") || strings.Contains(respLower, "budget") || strings.Contains(respLower, "бюджет")
			if askedBudget && variantCount < 3 {
				filtered := availableCSV
				if strings.TrimSpace(requestedCategory) != "" {
					filtered = filterProductsByCategory(availableCSV, requestedCategory)
				}
				brandMatched := false
				if len(brandTokens) > 0 {
					brandFiltered := filterProductsByBrand(filtered, brandTokens)
					if strings.TrimSpace(brandFiltered) != "" {
						filtered = brandFiltered
						brandMatched = true
					}
				}
				top := getRepresentativeProductsFromCSVNumbered(filtered, 5)
				if len(top) >= 3 {
					if forceRussian {
						intro := "Вот варианты по разным ценам:"
						if strings.TrimSpace(requestedCategory) != "" {
							intro = fmt.Sprintf("Варианты по категории %s:", requestedCategory)
						}
						if len(brandTokens) > 0 {
							brandLabel := strings.ToUpper(strings.Join(brandTokens, " "))
							if brandMatched {
								if strings.TrimSpace(requestedCategory) != "" {
									intro = fmt.Sprintf("%s — варианты по категории %s:", brandLabel, requestedCategory)
								} else {
									intro = fmt.Sprintf("%s — доступные варианты:", brandLabel)
								}
							} else {
								intro = fmt.Sprintf("По бренду %s точных совпадений нет. Вот доступные варианты:", brandLabel)
							}
						}
						followUp := "Если укажете бюджет и цель, подберу точнее."
						if strings.EqualFold(requestedCategory, "Storage") {
							followUp = "Укажите объем и тип (1TB/2TB, NVMe/SATA) — подберу точнее."
						}
						response = fmt.Sprintf("%s\n\n%s\n\n%s", intro, strings.Join(top, "\n"), followUp)
					} else {
						intro := "Mana turli narxdagi variantlar:"
						if strings.TrimSpace(requestedCategory) != "" {
							intro = fmt.Sprintf("%s bo'yicha variantlar:", requestedCategory)
						}
						if len(brandTokens) > 0 {
							brandLabel := strings.ToUpper(strings.Join(brandTokens, " "))
							if brandMatched {
								if strings.TrimSpace(requestedCategory) != "" {
									intro = fmt.Sprintf("%s %s bo'yicha variantlar:", brandLabel, requestedCategory)
								} else {
									intro = fmt.Sprintf("%s bo'yicha variantlar:", brandLabel)
								}
							} else {
								intro = fmt.Sprintf("%s bo'yicha aniq mos mahsulot topilmadi. Mana mavjud variantlar:", brandLabel)
							}
						}
						followUp := "Agar budjet va maqsadni aytsangiz, yanada aniqroq tavsiya qilaman."
						if strings.EqualFold(requestedCategory, "Storage") {
							followUp = "Hajm va turini yozsangiz (1TB/2TB, NVMe/SATA), aniqroq tavsiya qilaman."
						}
						response = fmt.Sprintf("%s\n\n%s\n\n%s", intro, strings.Join(top, "\n"), followUp)
					}
				}
			}
		}
	}

	message := entity.Message{
		ID:        uuid.New().String(),
		UserID:    userID,
		Username:  username,
		Text:      text, // Original text
		Response:  response,
		Timestamp: time.Now(),
	}

	if err := u.chatRepo.SaveMessage(ctx, message); err != nil {
		return "", fmt.Errorf("failed to save message: %w", err)
	}

	return response, nil
}

func extractModelTokens(text string) []string {
	lower := strings.ToLower(text)
	seen := make(map[string]struct{})
	var out []string

	// Special-case: "i 5" -> "i5"
	for _, m := range reIntelCoreShortModel.FindAllString(lower, -1) {
		tok := strings.ReplaceAll(strings.ReplaceAll(m, " ", ""), "\t", "")
		tok = strings.ReplaceAll(tok, "core", "")
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if _, ok := seen[tok]; !ok {
			seen[tok] = struct{}{}
			out = append(out, tok)
		}
	}

	// Generic alphanumeric tokens that include both letters and digits (e.g., i5, 13400f, rtx4070).
	for _, tok := range reAlphaNumToken.FindAllString(lower, -1) {
		if tok == "" {
			continue
		}
		hasLetter := false
		hasDigit := false
		for _, r := range tok {
			if r >= '0' && r <= '9' {
				hasDigit = true
			} else if r >= 'a' && r <= 'z' {
				hasLetter = true
			}
		}
		if !hasLetter || !hasDigit {
			continue
		}
		if len(tok) < 2 {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}

	// Keep it small so matching doesn't become too strict.
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func normalizeBrandTokens(tokens []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	if len(out) > 2 {
		out = out[:2]
	}
	return out
}

func extractBrandTokens(text string) []string {
	lower := strings.ToLower(text)
	normalized := normalizeBrandText(lower)
	tokens := reAlphaNumToken.FindAllString(normalized, -1)
	tokenSet := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		tokenSet[t] = struct{}{}
	}

	brandPhrases := []string{
		"western digital",
		"team group",
	}
	brandTokens := []string{
		"samsung", "lexar", "corsair", "kingston", "crucial", "adata", "xpg", "teamgroup",
		"patriot", "pny", "seagate", "toshiba", "wd", "gigabyte", "msi", "asus", "acer",
		"dell", "hp", "lenovo", "logitech", "razer", "steelseries", "intel", "amd", "nvidia",
		"gravastar", "vgn",
	}

	out := make(map[string]struct{})
	for _, phrase := range brandPhrases {
		if strings.Contains(normalized, phrase) {
			out[phrase] = struct{}{}
		}
	}
	for _, brand := range brandTokens {
		if len(brand) <= 2 {
			if _, ok := tokenSet[brand]; ok {
				out[brand] = struct{}{}
			}
			continue
		}
		if strings.Contains(normalized, brand) {
			out[brand] = struct{}{}
		}
	}

	result := make([]string, 0, len(out))
	for brand := range out {
		result = append(result, brand)
	}
	sort.Strings(result)
	return result
}

var queryStopwords = map[string]struct{}{
	"kerak": {}, "kerakli": {}, "bormi": {}, "bor": {}, "mavjud": {}, "mavjudmi": {},
	"qidir": {}, "qidiryap": {}, "qidiryapman": {}, "izlayman": {}, "izlayapman": {},
	"tavsiya": {}, "maslahat": {}, "nomli": {}, "model": {}, "modeli": {}, "brend": {},
	"brand": {}, "qaysi": {}, "qanday": {}, "nima": {}, "narx": {}, "qancha": {},
	"price": {}, "sotib": {}, "olmoqchi": {}, "olish": {}, "uchun": {}, "menga": {},
	"menda": {}, "men": {}, "bori": {}, "kerakmi": {},
	"yana": {}, "boshqa": {}, "boshqasi": {}, "boshqami": {}, "boshqacha": {}, "ham": {},
	"qaysilar": {}, "qaysilari": {}, "qaysilarini": {}, "qolgan": {}, "qolgani": {}, "qolganlari": {},
	"sizda": {}, "sizlarda": {}, "sizdami": {}, "sizlardami": {}, "sezda": {}, "sezlarda": {},
	"bizda": {}, "bizlarda": {}, "bizdami": {}, "bizning": {}, "sizning": {},
	"shu": {}, "usha": {}, "o'sha": {}, "bu": {}, "mana": {}, "qani": {}, "qanaqa": {},
	"borimi": {}, "bormikan": {}, "bormidi": {}, "bormidimi": {},
}

var queryCategoryWords = map[string]struct{}{
	"monitor": {}, "gpu": {}, "cpu": {}, "ram": {}, "ssd": {}, "hdd": {}, "nvme": {},
	"m2": {}, "motherboard": {}, "anakart": {}, "mobo": {}, "videokarta": {}, "vga": {},
	"korpus": {}, "case": {}, "cooler": {}, "sovutgich": {}, "psu": {}, "blok": {},
	"klaviatura": {}, "keyboard": {}, "mouse": {}, "sichqoncha": {}, "sichqon": {},
	"mishka": {}, "мыш": {}, "мышка": {}, "мишка": {}, "headset": {}, "naushnik": {},
	"quloqchin": {}, "chair": {}, "stul": {}, "кресло": {}, "pad": {}, "kovrik": {},
	"mousepad": {}, "storage": {}, "disk": {},
}

var queryCategoryStems = []string{
	"klaviatura", "keyboard", "monitor", "sichqon", "mouse", "videokart", "korpus",
	"motherboard", "anakart", "cooler", "sovutgich", "quloqchin", "naushnik", "headset",
	"storage", "mousepad", "kovrik", "chair", "stul", "kreslo", "protsessor", "processor",
}

func extractQueryTokens(text string) []string {
	normalized := normalizeBrandText(strings.ToLower(text))
	words := reAlphaToken.FindAllString(normalized, -1)
	seen := make(map[string]struct{})
	out := make([]string, 0, len(words))
	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		if _, ok := queryStopwords[w]; ok {
			continue
		}
		if _, ok := queryCategoryWords[w]; ok {
			continue
		}
		skip := false
		for _, stem := range queryCategoryStems {
			if strings.Contains(w, stem) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if _, ok := seen[w]; ok {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func normalizeBrandText(text string) string {
	replacer := strings.NewReplacer(
		"-", " ",
		"_", " ",
		".", " ",
		",", " ",
		"/", " ",
		"\\", " ",
		"’", "'",
	)
	out := replacer.Replace(text)
	out = strings.TrimSpace(strings.Join(strings.Fields(out), " "))
	return out
}

func mergeTokens(primary, secondary []string, limit int) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, t := range append(primary, secondary...) {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
		if limit > 0 && len(out) >= limit {
			return out
		}
	}
	return out
}

func findMatchingProductsFromCSVNumbered(csvData string, tokens []string, count int) []string {
	if strings.TrimSpace(csvData) == "" || len(tokens) == 0 || count <= 0 {
		return nil
	}
	cleanTokens := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		cleanTokens = append(cleanTokens, t)
	}
	if len(cleanTokens) == 0 {
		return nil
	}

	type item struct {
		name  string
		price float64
	}
	var items []item
	for _, line := range strings.Split(csvData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, price, ok := parseCatalogLineNameAndPrice(line)
		if !ok || strings.TrimSpace(name) == "" || price <= 0 {
			continue
		}
		nameLower := strings.ToLower(name)
		match := true
		for _, tok := range cleanTokens {
			if !strings.Contains(nameLower, tok) {
				match = false
				break
			}
		}
		if match {
			items = append(items, item{name: name, price: price})
		}
	}

	if len(items) == 0 {
		return nil
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].price < items[j].price
	})

	limit := count
	if limit > len(items) {
		limit = len(items)
	}

	var selected []item
	if limit == len(items) {
		selected = items
	} else if limit == 1 {
		selected = []item{items[len(items)/2]}
	} else {
		n := len(items)
		for k := 0; k < limit; k++ {
			idx := k * (n - 1) / (limit - 1)
			selected = append(selected, items[idx])
		}
	}

	out := make([]string, 0, len(selected))
	for i, it := range selected {
		out = append(out, fmt.Sprintf("%d. %s - %.2f$", i+1, it.name, it.price))
	}
	return out
}

func categoryAliases(category string) []string {
	cat := strings.TrimSpace(category)
	if cat == "" {
		return nil
	}
	switch strings.ToLower(cat) {
	case "storage", "rom", "ssd", "hdd":
		return []string{"Storage", "ROM"}
	case "cooling", "cooler", "cpu cooler", "case fan":
		return []string{"Cooling", "CPU Cooler", "Case Fan", "Cooler"}
	case "case", "cases":
		return []string{"Case", "CASE"}
	case "mousepad", "pad", "accessory":
		return []string{"Mousepad", "Accessory", "Pad"}
	default:
		return []string{cat}
	}
}

// validateAndFixPrices AI javobidagi noto'g'ri narxlarni CSV dan to'g'rilaydi
func (u *chatUseCase) validateAndFixPrices(response, csvData string) string {
	// CSV dan mahsulot nomlarini va narxlarini extract qilish
	priceMap := make(map[string]string) // mahsulot nomi -> to'g'ri narx

	for _, line := range strings.Split(csvData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, price, ok := parseCatalogLineNameAndPrice(line)
		if !ok || strings.TrimSpace(name) == "" || price <= 0 {
			continue
		}
		normalizedName := strings.ToLower(strings.TrimSpace(name))
		priceMap[normalizedName] = fmt.Sprintf("%.2f", price)
	}

	// AI javobida noto'g'ri narxlarni topish va to'g'rilash
	// Pattern: "Narxi: 1000$" yoki "- 1000.00$" yoki "Jami: 1000$"
	lines := strings.Split(response, "\n")
	fixed := make([]string, 0, len(lines))

	for _, line := range lines {
		fixedLine := line

		// Har bir qatorda mahsulot nomi va narx borligini tekshirish
		for productName, correctPrice := range priceMap {
			// Mahsulot nomi qatorda bormi?
			if strings.Contains(strings.ToLower(line), productName) {
				// Qatorda noto'g'ri narx bormi? (masalan, 899$ o'rniga 1000$)
				// Pattern: raqam + $ yoki raqam + .00$
				re := regexp.MustCompile(`(\d+(?:\.\d{1,2})?)\$`)
				matches := re.FindAllStringSubmatch(line, -1)

				if len(matches) > 0 {
					// To'g'ri narxni extract qilish
					correctPriceNum := regexp.MustCompile(`(\d+(?:\.\d{1,2})?)`).FindString(correctPrice)

					// Har bir topilgan narxni tekshirish
					for _, match := range matches {
						foundPrice := match[1]

						// Agar topilgan narx to'g'ri narxdan farq qilsa, almashtirish
						if correctPriceNum != "" && foundPrice != correctPriceNum {
							// Faqat katta farq bo'lsa almashtirish (10% dan ortiq)
							foundFloat, err1 := strconv.ParseFloat(foundPrice, 64)
							correctFloat, err2 := strconv.ParseFloat(correctPriceNum, 64)

							if err1 == nil && err2 == nil {
								diff := foundFloat - correctFloat
								if diff < 0 {
									diff = -diff
								}

								// Agar farq 10$ dan ko'p yoki 10% dan ko'p bo'lsa
								if diff > 10 || (diff/correctFloat)*100 > 10 {
									// Noto'g'ri narxni to'g'risiga almashtirish
									oldPattern := foundPrice + "$"
									newPattern := correctPriceNum + "$"
									fixedLine = strings.ReplaceAll(fixedLine, oldPattern, newPattern)

									log.Printf("⚠️ Narx to'g'irlandi: %s uchun %s → %s", productName, oldPattern, newPattern)
								}
							}
						}
					}
				}
			}
		}

		fixed = append(fixed, fixedLine)
	}

	return strings.Join(fixed, "\n")
}

type priceCandidate struct {
	raw      string
	currency string
	key      string
}

func syncTotalLineWithSinglePrice(response string) string {
	if strings.TrimSpace(response) == "" {
		return response
	}

	lines := strings.Split(response, "\n")
	totalIdx := -1
	totalLine := ""
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		base := stripBulletPrefixLine(trim)
		if reTotalLine.MatchString(base) {
			totalIdx = i
			totalLine = line
		}
	}
	if totalIdx == -1 {
		return response
	}

	totalMatches := rePriceWithCurrency.FindAllString(totalLine, -1)
	totalCurrency := ""
	if len(totalMatches) > 0 {
		totalCurrency = canonicalCurrencyFromMatch(totalMatches[0])
	}

	candidates := make(map[string]priceCandidate)
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		base := stripBulletPrefixLine(trim)
		if reTotalLine.MatchString(base) {
			continue
		}
		for _, match := range rePriceWithCurrency.FindAllString(line, -1) {
			key := normalizePriceKey(match)
			if key == "" {
				continue
			}
			if _, ok := candidates[key]; ok {
				continue
			}
			candidates[key] = priceCandidate{
				raw:      strings.TrimSpace(match),
				currency: canonicalCurrencyFromMatch(match),
				key:      key,
			}
		}
	}

	if len(candidates) == 0 {
		return response
	}

	filtered := make([]priceCandidate, 0, len(candidates))
	if totalCurrency != "" {
		for _, cand := range candidates {
			if cand.currency == totalCurrency {
				filtered = append(filtered, cand)
			}
		}
	} else {
		for _, cand := range candidates {
			filtered = append(filtered, cand)
		}
	}

	if len(filtered) != 1 {
		return response
	}
	chosen := filtered[0].raw

	updatedTotalLine := totalLine
	if len(totalMatches) > 0 {
		replaced := false
		updatedTotalLine = rePriceWithCurrency.ReplaceAllStringFunc(totalLine, func(s string) string {
			if replaced {
				return s
			}
			replaced = true
			return chosen
		})
	} else {
		trimmed := strings.TrimRight(updatedTotalLine, " ")
		if strings.Contains(trimmed, ":") || strings.Contains(trimmed, "-") {
			updatedTotalLine = trimmed + " " + chosen
		} else {
			updatedTotalLine = trimmed + ": " + chosen
		}
	}

	if updatedTotalLine == totalLine {
		return response
	}
	lines[totalIdx] = updatedTotalLine
	return strings.Join(lines, "\n")
}

func stripBulletPrefixLine(s string) string {
	return strings.TrimSpace(strings.TrimLeft(s, "*-•— "))
}

func canonicalCurrencyFromMatch(match string) string {
	lower := strings.ToLower(match)
	switch {
	case strings.Contains(lower, "$") || strings.Contains(lower, "usd"):
		return "$"
	case strings.Contains(lower, "€") || strings.Contains(lower, "eur"):
		return "€"
	case strings.Contains(lower, "₽") || strings.Contains(lower, "rub"):
		return "₽"
	case strings.Contains(lower, "so'm") || strings.Contains(lower, "so’m") ||
		strings.Contains(lower, "сум") || strings.Contains(lower, " sum") || strings.HasSuffix(lower, "sum"):
		return "so'm"
	default:
		return ""
	}
}

func normalizePriceKey(match string) string {
	currency := canonicalCurrencyFromMatch(match)
	if currency == "" {
		return ""
	}
	var digits strings.Builder
	for _, r := range match {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		return ""
	}
	return currency + "|" + digits.String()
}

// ProcessConfigMessage MAXSUS konfiguratsiya uchun xabar qayta ishlash
// Bu faqat /configuratsiya komandasi uchun ishlatiladi va PC yig'ishga ruxsat beradi
func (u *chatUseCase) ProcessConfigMessage(ctx context.Context, userID int64, username, text string) (string, error) {
	// Oldingi tarixni olish (oxirgi 10 ta xabar)
	history, err := u.chatRepo.GetHistory(ctx, userID, 10)
	if err != nil {
		return "", fmt.Errorf("failed to get history: %w", err)
	}

	// CSV ma'lumotlarini olish
	csvData, csvFilename, csvErr := u.productRepo.GetCSV(ctx)
	availableCSV := ""
	if csvErr == nil && csvData != "" {
		availableCSV = filterProductsByStock(csvData)
	}
	hasCSV := csvErr == nil && strings.TrimSpace(availableCSV) != ""

	// Foydalanuvchi xabariga mahsulot katalogini qo'shish
	enrichedText := text

	// Agar user savol bersa (yaxshimi, qanday, farqi, va h.k.), yangi konfiguratsiya emas, baholash kerak
	lower := strings.ToLower(text)
	isQuestion := strings.Contains(lower, "?") ||
		strings.Contains(lower, "yaxshimi") ||
		strings.Contains(lower, "yoqdimi") ||
		strings.Contains(lower, "qanday") ||
		strings.Contains(lower, "farqi") ||
		strings.Contains(lower, "yuqoridagi") ||
		strings.Contains(lower, "yuqorida") ||
		strings.Contains(lower, "yig'ilgan") ||
		strings.Contains(lower, "bu ") ||
		strings.Contains(lower, "shu ") ||
		strings.Contains(lower, "o'sha") ||
		strings.Contains(lower, "хорош") ||
		strings.Contains(lower, "нравится") ||
		strings.Contains(lower, "этот") ||
		strings.Contains(lower, "данн")

	if hasCSV {
		// CSV ma'lumotlaridan foydalanish
		var instruction string
		if isQuestion {
			// Savol bo'lsa - baholash, yangi konfiguratsiya emas
			instruction = "Mijoz savoli (oldingi konfiguratsiya haqida): " + text
		} else {
			// Yangi konfiguratsiya so'rovi
			instruction = "Mijozga to'liq PC konfiguratsiyasi yig'ib ber: " + text
		}

		enrichedText = fmt.Sprintf(`%s

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📦 SIZNING DO'KONINGIZDA MAVJUD MAHSULOTLAR (CSV FORMAT):
Fayl: %s

%s
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━`, instruction, csvFilename, availableCSV)

		fmt.Printf("📦 CSV Katalog yuborildi (CONFIG MODE): %s (%d bytes)\n", csvFilename, len(availableCSV))
	}

	// MAXSUS AI dan javob olish (konfiguratsiya rejimida)
	response, err := u.aiRepo.GenerateConfigResponse(ctx, userID, enrichedText, history)
	if err != nil {
		return "", fmt.Errorf("failed to generate config response: %w", err)
	}

	// ✅ POST-PROCESSING: AI javobini validatsiya qilish va noto'g'ri narxlarni to'g'rilash
	if hasCSV {
		response = u.validateAndFixPrices(response, availableCSV)
		response = syncTotalLineWithSinglePrice(response)
	}

	// Xabar va javobni saqlash
	message := entity.Message{
		ID:        uuid.New().String(),
		UserID:    userID,
		Username:  username,
		Text:      text, // Original text
		Response:  response,
		Timestamp: time.Now(),
	}

	if err := u.chatRepo.SaveMessage(ctx, message); err != nil {
		return "", fmt.Errorf("failed to save message: %w", err)
	}

	return response, nil
}

// needsPhoneContact foydalanuvchi matnida telefon/aloqa haqida so'rov bormi shuni aniqlaydi
func needsPhoneContact(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"telefon", "raqam", "aloqa", "bog'lan", "bog'lanish", "call", "phone", "contact", "qo'ng'iroq", "qongiroq", "zang",
		"admin bilan gap", "adminga yaz", "admin bilan aloq",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// buildProductsContext mahsulotlardan kontekst yaratish
func (u *chatUseCase) buildProductsContext(products []entity.Product) string {
	var sb strings.Builder

	// Kategoriyalar bo'yicha guruhlash
	categoryMap := make(map[string][]entity.Product)
	for _, p := range products {
		if p.Stock <= 0 {
			continue
		}
		cat := p.Category
		if cat == "" {
			cat = "Boshqa"
		}
		categoryMap[cat] = append(categoryMap[cat], p)
	}

	if len(categoryMap) == 0 {
		return ""
	}

	// Mahsulotlarni yozish
	for category, prods := range categoryMap {
		sb.WriteString(fmt.Sprintf("\n📂 %s:\n", category))
		for i, p := range prods {
			// Narxni dollar formatida ko'rsatish (Stock 0 bo'lsa ham ko'rsatamiz - product mavjud)
			sb.WriteString(fmt.Sprintf("  %d. %s - $%.2f", i+1, p.Name, p.Price))

			if p.Stock > 0 {
				sb.WriteString(fmt.Sprintf(" (Omborda: %d ta)", p.Stock))
			}

			if p.Description != "" {
				sb.WriteString(fmt.Sprintf("\n     └─ %s", p.Description))
			}

			// Specs borligini tekshirish
			if len(p.Specs) > 0 {
				sb.WriteString("\n     └─ ")
				specs := []string{}
				for key, value := range p.Specs {
					specs = append(specs, fmt.Sprintf("%s: %s", key, value))
				}
				sb.WriteString(strings.Join(specs, ", "))
			}

			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ClearHistory foydalanuvchi tarixini tozalash
func (u *chatUseCase) ClearHistory(ctx context.Context, userID int64) error {
	return u.chatRepo.ClearHistory(ctx, userID)
}

// GetHistory foydalanuvchi tarixini olish
func (u *chatUseCase) GetHistory(ctx context.Context, userID int64) ([]entity.Message, error) {
	return u.chatRepo.GetHistory(ctx, userID, 0)
}

// extractBudgetFromText matndan budjet raqamini extract qilish
// Masalan: "700$" → 700, "1k$" → 1000, "1.5k" → 1500, "1k so'm" → 1000
func extractBudgetFromText(text string) int {
	lower := strings.ToLower(text)

	// BIRINCHI: "1k$" yoki "1.5k" kabi shorthand format'ni topish
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*k\s*\$?`)
	matches := re.FindStringSubmatch(lower)
	if len(matches) > 1 {
		if num, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return int(num * 1000)
		}
	}

	// IKKINCHI: "1000000 so'm" yoki "1m" kabi million format'ni topish
	re = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*m(?:illion)?\s*\$?`)
	matches = re.FindStringSubmatch(lower)
	if len(matches) > 1 {
		if num, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return int(num * 1000000)
		}
	}

	// UCHINCHI: "budjet 1000" yoki "budjetim 150$" yoki "atrofida 700" kabi format'ni topish
	re = regexp.MustCompile(`(?i)(?:budjet(?:im)?|budgetim|atrofida|narx|pul(?:im)?)\s*:?\s*(\d+)\s*\$?`)
	matches = re.FindStringSubmatch(lower)
	if len(matches) > 1 {
		if budget, err := strconv.Atoi(matches[1]); err == nil {
			return budget
		}
	}

	// BESHINCHI: Agar hech narsa topilmasa, ixtiyoriy raqam + $ ni topish
	re = regexp.MustCompile(`(\d+)\s*\$`)
	matches = re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if budget, err := strconv.Atoi(matches[1]); err == nil {
			// Faqat agar raqam 50 dan katta bo'lsa (chunki 16$ RAM emas, balki budjet)
			if budget >= 50 {
				return budget
			}
		}
	}

	// OLTINCHI: Agar user faqat raqam yozsa (masalan: "1000"), budjet deb qabul qilamiz
	clean := strings.ReplaceAll(strings.TrimSpace(text), " ", "")
	if regexp.MustCompile(`^\d{2,7}$`).MatchString(clean) { // 2-7 raqam: 50..9 999 999
		if budget, err := strconv.Atoi(clean); err == nil && budget >= 50 {
			return budget
		}
	}

	return 0
}

func extractPurposeFromText(text string) string {
	lower := strings.ToLower(text)

	// Gaming
	for _, kw := range []string{
		"gaming", "game", "o'yin", "oʻyin", "o‘yin", "игр", "гейм",
		"cs2", "cs:go", "csgо", "valorant", "pubg", "dota", "gta", "fortnite",
	} {
		if strings.Contains(lower, kw) {
			return "Gaming"
		}
	}

	// Developer / Programming
	for _, kw := range []string{
		"developer", "dev", "dasturchi", "program", "coding",
		"backend", "frontend", "fullstack", "ide", "docker",
	} {
		if strings.Contains(lower, kw) {
			return "Developer"
		}
	}

	// Design / Editing / Rendering
	for _, kw := range []string{
		"dizayn", "design", "designer",
		"montaj", "монтаж",
		"editing", "video edit", "premiere", "after effects", "davinci", "davinchi",
		"render", "рендер", "3d", "blender", "maya", "3ds max",
		"photoshop", "illustrator", "aftereffects",
	} {
		if strings.Contains(lower, kw) {
			return "Design"
		}
	}

	// Server / Hosting
	for _, kw := range []string{
		"server", "сервер", "hosting", "vps", "dedicated",
		"data center", "datacenter", "nas", "storage server",
	} {
		if strings.Contains(lower, kw) {
			return "Server"
		}
	}

	// Streaming
	for _, kw := range []string{
		"stream", "стрим", "obs", "twitch", "youtube live",
	} {
		if strings.Contains(lower, kw) {
			return "Streaming"
		}
	}

	// Office / Work / Study
	if strings.Contains(lower, "ish uchun") || strings.Contains(lower, "для работы") || strings.Contains(lower, "for work") {
		return "Office"
	}
	for _, kw := range []string{
		"office", "ofis", "офис",
		"work", "работ", "учеб", "dars", "study",
		"word", "excel", "powerpoint", "zoom", "teams",
	} {
		if strings.Contains(lower, kw) {
			return "Office"
		}
	}

	return ""
}

func isProductInquiry(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}

	// Aniq kategoriya/model ko'rinsa - bu mahsulot so'rovi.
	if detectRequestedCategory(text) != "" {
		return true
	}

	// Follow-up: budjet yoki maqsad kiritilgan bo'lsa ham mahsulot so'rovi bo'lishi mumkin.
	if extractBudgetFromText(text) > 0 || extractPurposeFromText(text) != "" {
		return true
	}

	// So'rov fe'llari / kalit so'zlar (juda umumiy so'zlarni qo'shmaymiz).
	inquiryKeywords := []string{
		"kerak", "bormi", "bor mi", "mavjud", "mavjudmi",
		"qidir", "qidiryap", "izlayap", "izlayman",
		"tavsiya", "maslahat", "recommend", "suggest",
		"нуж", "есть ли", "покажи", "посовет",
		"narx", "qancha", "price", "цена", "сколько",
	}
	for _, kw := range inquiryKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	return false
}

var (
	csvStockHeaders = []string{"stock", "soni", "miqdor", "qolgan", "остаток", "количество", "qty", "quantity", "count"}
	csvPriceHeaders = []string{"price", "narx", "цена", "стоимость", "cost"}
	csvNameHeaders  = []string{"name", "название", "товар", "product", "mahsulot", "nomi"}
)

func filterProductsByStock(csvData string) string {
	if strings.TrimSpace(csvData) == "" {
		return csvData
	}
	lines := strings.Split(csvData, "\n")
	out := make([]string, 0, len(lines))
	stockCol := -1

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		rec, err := parseCSVLine(line)
		if err != nil || len(rec) == 0 {
			out = append(out, raw)
			continue
		}

		if stockCol == -1 {
			if col, ok := detectCSVStockHeader(rec); ok {
				stockCol = col
				out = append(out, raw)
				continue
			}
		}

		priceOk := false
		if len(rec) >= 2 {
			if _, ok := parseCatalogPrice(rec[len(rec)-1]); ok {
				priceOk = true
			}
		}
		if priceOk {
			if stockVal, ok := extractCSVStockValue(rec, stockCol); ok && stockVal <= 0 {
				continue
			}
		}
		out = append(out, raw)
	}

	return strings.Join(out, "\n")
}

func parseCSVLine(line string) ([]string, error) {
	r := csv.NewReader(strings.NewReader(line))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	return r.Read()
}

func detectCSVStockHeader(rec []string) (int, bool) {
	stockCol := -1
	hasHeader := false
	for i, cell := range rec {
		norm := normalizeCSVHeaderCell(cell)
		if norm == "" {
			continue
		}
		if stockCol == -1 && containsCSVKeyword(norm, csvStockHeaders) {
			stockCol = i
			hasHeader = true
		}
		if containsCSVKeyword(norm, csvPriceHeaders) || containsCSVKeyword(norm, csvNameHeaders) {
			hasHeader = true
		}
	}
	if !hasHeader {
		return -1, false
	}
	return stockCol, true
}

func extractCSVStockValue(rec []string, stockCol int) (float64, bool) {
	if stockCol >= 0 && stockCol < len(rec) {
		return parseCatalogPrice(rec[stockCol])
	}
	if len(rec) >= 3 {
		return parseCatalogPrice(rec[1])
	}
	return 0, false
}

func normalizeCSVHeaderCell(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("_", " ", "-", " ", "—", " ", "–", " ", ".", " ").Replace(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func containsCSVKeyword(value string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(value, kw) {
			return true
		}
	}
	return false
}

func filterInStockProducts(products []entity.Product) []entity.Product {
	if len(products) == 0 {
		return nil
	}
	out := make([]entity.Product, 0, len(products))
	for _, p := range products {
		if p.Stock > 0 {
			out = append(out, p)
		}
	}
	return out
}

// filterProductsByBudget CSV ma'lumotlarini budjet bo'yicha filter qilish.
// Eslatma: CSV qatorda mahsulot nomida ham raqamlar bo'lishi mumkin (RTX 4060),
// shuning uchun narxni doim oxirgi ustundan olishga harakat qilamiz.
func filterProductsByBudget(csvData string, budget int) string {
	if budget <= 0 {
		return csvData
	}
	lines := strings.Split(csvData, "\n")
	var filtered []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_, price, ok := parseCatalogLineNameAndPrice(line)
		if !ok {
			continue
		}
		if price <= float64(budget) {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

// filterProductsByCategory - faqat kategoriya bo'yicha filter (budjet bo'lmaganda ham).
// Natija ichida kategoriya heading'i va shu kategoriyadagi mahsulotlar bo'ladi.
func filterProductsByCategory(csvData string, category string) string {
	if strings.TrimSpace(category) == "" {
		return csvData
	}
	aliases := categoryAliases(category)
	matchCategory := func(s string) bool {
		for _, a := range aliases {
			if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(a)) {
				return true
			}
		}
		return false
	}
	lines := strings.Split(csvData, "\n")
	currentCategory := ""
	inCategory := false
	var out []string

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		_, _, ok := parseCatalogLineNameAndPrice(line)
		if !ok {
			// Kategoriya qatori
			currentCategory = line
			if matchCategory(currentCategory) {
				inCategory = true
				out = append(out, currentCategory)
			} else {
				inCategory = false
			}
			continue
		}

		if inCategory && matchCategory(currentCategory) {
			out = append(out, line)
		}
	}

	if len(out) == 0 {
		return csvData
	}
	return strings.Join(out, "\n")
}

// filterProductsByBudgetAndCategory - budjet + kategoriya bo'yicha filter (AI kontekst uchun).
// Natija budjetga eng yaqin narxlar yuqorida bo'ladi.
func filterProductsByBudgetAndCategory(csvData string, budget int, category string) string {
	if strings.TrimSpace(category) == "" {
		return filterProductsByBudget(csvData, budget)
	}
	if budget <= 0 {
		return ""
	}
	aliases := categoryAliases(category)
	matchCategory := func(s string) bool {
		for _, a := range aliases {
			if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(a)) {
				return true
			}
		}
		return false
	}
	lines := strings.Split(csvData, "\n")
	currentCategory := ""
	type item struct {
		line  string
		price float64
	}
	var items []item

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		name, price, ok := parseCatalogLineNameAndPrice(line)
		if ok {
			_ = name
			if !matchCategory(currentCategory) {
				continue
			}
			if price <= float64(budget) {
				items = append(items, item{line: line, price: price})
			}
			continue
		}
		// Kategoriya qatori
		currentCategory = line
	}

	if len(items) == 0 {
		return ""
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].price > items[j].price
	})

	var out []string
	out = append(out, category)
	for _, it := range items {
		out = append(out, it.line)
	}
	return strings.Join(out, "\n")
}

func filterProductsByBrand(csvData string, brands []string) string {
	if strings.TrimSpace(csvData) == "" || len(brands) == 0 {
		return csvData
	}
	brands = normalizeBrandTokens(brands)
	if len(brands) == 0 {
		return csvData
	}
	var out []string
	for _, line := range strings.Split(csvData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, _, ok := parseCatalogLineNameAndPrice(line)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		nameLower := strings.ToLower(name)
		for _, brand := range brands {
			if brandMatchInName(nameLower, brand) {
				out = append(out, line)
				break
			}
		}
	}
	return strings.Join(out, "\n")
}

func brandMatchInName(nameLower, brand string) bool {
	if brand == "" {
		return false
	}
	if len(brand) <= 2 {
		for _, tok := range reAlphaNumToken.FindAllString(nameLower, -1) {
			if tok == brand {
				return true
			}
		}
		return false
	}
	return strings.Contains(nameLower, brand)
}

func parseCatalogLineNameAndPrice(line string) (string, float64, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", 0, false
	}

	// 1) CSV format (name,price) - qo'shtirnoq va "," bo'lishi mumkin
	r := csv.NewReader(strings.NewReader(line))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	rec, err := r.Read()
	if err == nil && len(rec) >= 2 {
		name := strings.TrimSpace(rec[0])
		priceField := strings.TrimSpace(rec[len(rec)-1])
		price, ok := parseCatalogPrice(priceField)
		if ok {
			return name, price, true
		}
	}

	// 2) Fallback: "NAME - 250.00$" kabi formatlar
	// Narxni oxiridagi $ bilan yoki oxirgi raqam bilan olamiz.
	priceRe := regexp.MustCompile(`(\d+(?:[.,]\d{1,2})?)\s*\$`)
	matches := priceRe.FindAllStringSubmatch(line, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1][1]
		last = strings.ReplaceAll(last, ",", ".")
		if v, err := strconv.ParseFloat(last, 64); err == nil {
			// Name: "-" dan oldingi qism
			name := line
			if idx := strings.LastIndex(line, " - "); idx > 0 {
				name = strings.TrimSpace(line[:idx])
			}
			return name, v, true
		}
	}

	return "", 0, false
}

func parseCatalogPrice(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	lower := strings.ToLower(s)
	for _, tok := range []string{"usd", "$", "so'm", "soum", "sum", "сум"} {
		lower = strings.ReplaceAll(lower, tok, "")
	}
	lower = strings.ReplaceAll(lower, " ", "")
	lower = strings.ReplaceAll(lower, " ", "") // NBSP variants

	// "1,400.00" -> 1400.00, "1400,00" -> 1400.00
	if strings.Contains(lower, ",") && strings.Contains(lower, ".") {
		lower = strings.ReplaceAll(lower, ",", "")
	} else if strings.Contains(lower, ",") && !strings.Contains(lower, ".") {
		lower = strings.ReplaceAll(lower, ",", ".")
	}

	// Extract first numeric chunk
	num := regexp.MustCompile(`\d+(?:\.\d+)?`).FindString(lower)
	if num == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(num, 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

func detectRequestedCategory(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "monitor"):
		return "Monitor"
	case strings.Contains(lower, "gpu") || strings.Contains(lower, "rtx") || strings.Contains(lower, "radeon") ||
		strings.Contains(lower, "videokarta") || strings.Contains(lower, "video karta") || strings.Contains(lower, "vga"):
		return "GPU"
	case strings.Contains(lower, "anakart") || strings.Contains(lower, "motherboard") || strings.Contains(lower, "mat plata") || strings.Contains(lower, "mobo"):
		return "Motherboard"
	case reIntelCoreShortModel.MatchString(lower):
		return "CPU"
	case strings.Contains(lower, "cpu") || strings.Contains(lower, "protsessor") || strings.Contains(lower, "processor") ||
		strings.Contains(lower, "intel") || strings.Contains(lower, "ryzen") || strings.Contains(lower, "amd"):
		return "CPU"
	case strings.Contains(lower, "ram") || strings.Contains(lower, "operativ") || strings.Contains(lower, "оператив"):
		return "RAM"
	case strings.Contains(lower, "psu") || strings.Contains(lower, "power supply") || strings.Contains(lower, "quvvat bloki") || strings.Contains(lower, "blok pitaniya"):
		return "PSU"
	case strings.Contains(lower, "case") || strings.Contains(lower, "korpus") || strings.Contains(lower, "корпус"):
		return "Case"
	case strings.Contains(lower, "cooler") || strings.Contains(lower, "sovutgich") || strings.Contains(lower, "охлаж"):
		return "Cooling"
	case strings.Contains(lower, "ssd") || strings.Contains(lower, "hdd") || strings.Contains(lower, "nvme") || strings.Contains(lower, "m2") ||
		strings.Contains(lower, "disk"):
		return "Storage"
	case strings.Contains(lower, "keyboard") || strings.Contains(lower, "klaviatura") || strings.Contains(lower, "клавиатура"):
		return "Keyboard"
	case strings.Contains(lower, "mouse") || strings.Contains(lower, "sichqoncha") || strings.Contains(lower, "sichqon") ||
		strings.Contains(lower, "мыш") || strings.Contains(lower, "мышка") || strings.Contains(lower, "мишка") || strings.Contains(lower, "mishka"):
		return "Mouse"
	case strings.Contains(lower, "headset") || strings.Contains(lower, "naushnik") || strings.Contains(lower, "quloqchin"):
		return "Headset"
	case strings.Contains(lower, "chair") || strings.Contains(lower, "stul") || strings.Contains(lower, "кресло"):
		return "Chair"
	case strings.Contains(lower, "pad") || strings.Contains(lower, "kovrik") || strings.Contains(lower, "mousepad"):
		return "Mousepad"
	default:
		return ""
	}
}

// validateProductsExistInCSV Verifies that all recommended products exist in the CSV
// Returns a warning message if hallucinated products are found
func validateProductsExistInCSV(response, filteredCSV string, budget int) string {
	if filteredCSV == "" {
		return ""
	}

	// Extract all prices from the response
	pricePattern := regexp.MustCompile(`(\d+(?:\.\d{1,2})?)\$`)
	priceMatches := pricePattern.FindAllString(response, -1)
	log.Printf("[VALIDATION] Prices found in AI response: %v", priceMatches)

	if len(priceMatches) == 0 {
		return ""
	}

	// Build a map of products in the CSV with their prices
	csvProducts := make(map[float64]bool)
	lines := strings.Split(filteredCSV, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_, price, ok := parseCatalogLineNameAndPrice(line)
		if ok {
			csvProducts[price] = true
		}
	}
	log.Printf("[VALIDATION] Built csvProducts map from filtered CSV. Size: %d. Budget: %d", len(csvProducts), budget)

	// Check if any prices in the response are not in the filtered list
	var hallucinated []string
	for _, priceMatch := range priceMatches {
		priceStr := strings.TrimSuffix(priceMatch, "$")
		if price, err := strconv.ParseFloat(priceStr, 64); err == nil {
			if !csvProducts[price] {
				hallucinated = append(hallucinated, priceMatch)
			}
		}
	}
	log.Printf("[VALIDATION] Hallucinated/Over-budget prices found: %v", hallucinated)

	if len(hallucinated) > 0 {
		warning := fmt.Sprintf("\n\n⚠️ OGOHLANTIRISH: Quyidagi narxlar CSV da yo'q yoki budjetdan ortiq:\n%v", hallucinated)
		log.Printf("[VALIDATION] Appending warning: %s", warning)
		return warning
	}
	return ""
}

func getTopProductsFromCSV(csvData string, count int) []string {
	type item struct {
		name  string
		price float64
	}
	var items []item
	for _, line := range strings.Split(csvData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, price, ok := parseCatalogLineNameAndPrice(line)
		if !ok || strings.TrimSpace(name) == "" || price <= 0 {
			continue
		}
		items = append(items, item{name: name, price: price})
	}

	if len(items) == 0 {
		return nil
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].price > items[j].price
	})

	limit := count
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, fmt.Sprintf("- %s - %.2f$", items[i].name, items[i].price))
	}
	return out
}

func getTopProductsFromCSVNumbered(csvData string, count int) []string {
	type item struct {
		name  string
		price float64
	}
	var items []item
	for _, line := range strings.Split(csvData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, price, ok := parseCatalogLineNameAndPrice(line)
		if !ok || strings.TrimSpace(name) == "" || price <= 0 {
			continue
		}
		items = append(items, item{name: name, price: price})
	}

	if len(items) == 0 {
		return nil
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].price > items[j].price
	})

	limit := count
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, fmt.Sprintf("%d. %s - %.2f$", i+1, items[i].name, items[i].price))
	}
	return out
}

func getRepresentativeProductsFromCSVNumbered(csvData string, count int) []string {
	type item struct {
		name  string
		price float64
	}
	var items []item
	for _, line := range strings.Split(csvData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, price, ok := parseCatalogLineNameAndPrice(line)
		if !ok || strings.TrimSpace(name) == "" || price <= 0 {
			continue
		}
		items = append(items, item{name: name, price: price})
	}

	if len(items) == 0 {
		return nil
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].price < items[j].price
	})

	limit := count
	if limit <= 0 {
		limit = 5
	}
	if limit > len(items) {
		limit = len(items)
	}

	var selected []item
	if limit == 1 {
		selected = append(selected, items[len(items)/2])
	} else {
		n := len(items)
		for k := 0; k < limit; k++ {
			idx := k * (n - 1) / (limit - 1)
			selected = append(selected, items[idx])
		}
	}

	out := make([]string, 0, len(selected))
	for i, it := range selected {
		out = append(out, fmt.Sprintf("%d. %s - %.2f$", i+1, it.name, it.price))
	}
	return out
}

func countNumberedVariants(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' {
			if trimmed[1] == '.' || trimmed[1] == ')' {
				count++
			}
		}
	}
	return count
}

func countPricedVariants(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[1] == ')') && strings.Contains(trimmed, "$") {
			count++
			continue
		}
		if (strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "•")) && strings.Contains(trimmed, "$") {
			count++
			continue
		}
	}
	return count
}

// getCheapestProducts - CSV dan eng arzon mahsulotlarni qaytaradi (count ta) va minimal narxni
func getCheapestProducts(csvData string, count int) ([]string, float64) {
	type item struct {
		name  string
		price float64
	}
	var items []item
	for _, ln := range strings.Split(csvData, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		name, price, ok := parseCatalogLineNameAndPrice(ln)
		if !ok || strings.TrimSpace(name) == "" || price <= 0 {
			continue
		}
		items = append(items, item{name: name, price: price})
	}

	if len(items) == 0 {
		return nil, 0
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].price < items[j].price
	})

	limit := count
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	var out []string
	for i := 0; i < limit; i++ {
		out = append(out, fmt.Sprintf("- %s (%.2f$)", items[i].name, items[i].price))
	}
	return out, items[0].price
}

// fuzzyMatch returns true if all words in b are found in a (order-insensitive, simple contains)
func fuzzyMatch(a, b string) bool {
	aWords := strings.Fields(a)
	bWords := strings.Fields(b)
	matchCount := 0
	for _, bw := range bWords {
		for _, aw := range aWords {
			if strings.Contains(aw, bw) || strings.Contains(bw, aw) {
				matchCount++
				break
			}
		}
	}
	return matchCount == len(bWords) && len(bWords) > 0
}

// isProductSearchIntent checks if the user is asking for a product recommendation
func isProductSearchIntent(text string) bool {
	lower := strings.ToLower(text)
	log.Printf("[DEBUG] isProductSearchIntent called with: '%s'", lower)
	keywords := []string{
		"kerak", "tavsiya", "bo'li", "narx", "qanday", "qancha", "video", "karta", "gpu", "cpu", "ram", "ssd",
		"monitor", "laptop", "kompyuter", "processor", "motherboard", "power", "case", "cooler", "davinci",
		"davinchi", "premiere", "adobe", "editing", "montaj", "gaming", "cs2", "o'yniman", "3d", "rendering",
		"streaming", "work", "budget", "variantu", "yoqadigan", "taklif", "maslahat", "uchun", "o'yin", "edit",
		"uchun", "uchun deyapsizmi", "deyapsizmi", "kerak deyapsiz",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
