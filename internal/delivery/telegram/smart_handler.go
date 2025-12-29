package telegram

import (
	"context"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/yourusername/telegram-ai-bot/internal/domain/entity"
	"github.com/yourusername/telegram-ai-bot/internal/usecase"
)

// chatUseCaseWrapper ChatUseCase ni ChatRepository interface ga mos keltiradigan adapter
type chatUseCaseWrapper struct {
	useCase usecase.ChatUseCase
}

func (w *chatUseCaseWrapper) SaveMessage(ctx context.Context, message entity.Message) error {
	// SmartRouter faqat GetHistory ishlatadi, SaveMessage kerak emas
	return nil
}

func (w *chatUseCaseWrapper) GetHistory(ctx context.Context, userID int64, limit int) ([]entity.Message, error) {
	// ChatUseCase dan history olish
	history, err := w.useCase.GetHistory(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Limit qo'llash
	if len(history) > limit {
		return history[len(history)-limit:], nil
	}
	return history, nil
}

func (w *chatUseCaseWrapper) ClearHistory(ctx context.Context, userID int64) error {
	return w.useCase.ClearHistory(ctx, userID)
}

func (w *chatUseCaseWrapper) ClearAll(ctx context.Context) error {
	// SmartRouter ishlatmaydi
	return nil
}

func (w *chatUseCaseWrapper) GetContext(ctx context.Context, userID int64) (*entity.ChatContext, error) {
	// SmartRouter ishlatmaydi, faqat GetHistory ishlatadi
	return nil, nil
}

// handleSmartTextMessage Smart Router bilan xabarni qayta ishlash
func (h *BotHandler) handleSmartTextMessage(ctx context.Context, userID int64, username, text string, chatID int64, msg *tgbotapi.Message, lang string) {
	// Smart Router yaratish (AI client bilan)
	// chatRepo ni chatUseCase orqali olamiz
	router := &SmartRouter{
		aiClient: h.geminiClient,
		chatRepo: &chatUseCaseWrapper{useCase: h.chatUseCase},
	}

	// Intent aniqlash
	intent := router.DetectIntent(userID, text)

	// Debug log
	log.Printf("[SmartRouter] UserID=%d, Intent=%s, Text=%q", userID, router.GetRouteDescription(intent), text)

	// Intent bo'yicha yo'naltirish
	switch intent {
	case IntentPCBuildRequest:
		// PC YIG'ISH SO'ROVI - /configuratsiya ga yo'naltirish
		h.sendMessage(chatID, t(lang,
			"To'liq PC konfiguratsiyasi uchun /configuratsiya komandasini bosing 😊\n\nU yerda sizga professional maslahat berib, budjet va maqsadingizga mos PC tuzib beraman!",
			"Для полной конфигурации ПК нажмите /configuratsiya 😊\n\nТам я дам вам профессиональную консультацию и соберу ПК по вашему бюджету и целям!"))
		return

	case IntentProductSearch:
		// MAHSULOT QIDIRUV - oddiy chat AI (savollar beradi)
		h.handleNormalAIChat(ctx, userID, username, text, chatID, msg, lang)
		return

	case IntentQuestion:
		// SAVOL - oldingi konfiguratsiya haqida
		// Chat history'dan konfiguratsiya topish va AI ga yuborish
		h.handleNormalAIChat(ctx, userID, username, text, chatID, msg, lang)
		return

	case IntentHistoryReference:
		// OLDINGI XABARGA MUROJAAT - chat history'dan topish
		h.handleNormalAIChat(ctx, userID, username, text, chatID, msg, lang)
		return

	case IntentNormalChat:
		// ODDIY SUHBAT
		h.handleNormalAIChat(ctx, userID, username, text, chatID, msg, lang)
		return

	default:
		// Default - oddiy chat
		h.handleNormalAIChat(ctx, userID, username, text, chatID, msg, lang)
		return
	}
}

// handleNormalAIChat oddiy AI chat (mahsulot qidiruv, savol-javob)
func (h *BotHandler) handleNormalAIChat(ctx context.Context, userID int64, username, text string, chatID int64, msg *tgbotapi.Message, lang string) {
	if idx, ok := parseSelectionIndex(text); ok {
		if h.handleVariantSelection(ctx, userID, chatID, idx) {
			return
		}
	}
	// Agar foydalanuvchi mahsulot yoqqanini 👍 bilan bildirsa
	if containsThumbsUp(text) {
		h.handlePurchaseIntentPrompt(ctx, userID, username, chatID, text)
		return
	}

	if isAffirmativeShort(text) {
		if last, ok := h.getLastSuggestion(userID); ok && strings.TrimSpace(last) != "" {
			if isLikelyProductSuggestion(last) && !isConfigLikeResponse(last) {
				h.sendPurchaseConfirmationButtons(chatID, userID, last, "")
				return
			}
		}
		if h.promptVariantSelection(ctx, userID, chatID) {
			return
		}
	}

	// Agar foydalanuvchi matnda sotib olishga tayyorligini bildirsa
	if isPurchaseIntent(text) {
		// Narx yoki aniq SKU bo'lmasa, avval aniqlashtiramiz
		if !isProductModelMention(text) || !hasPriceInfo(text) {
			h.handlePurchaseIntentPrompt(ctx, userID, username, chatID, text)
			return
		}
	}

	// Agar AI javobi tayyorlanayotgan bo'lsa, yangi so'rovlarni to'xtatamiz
	if h.isProcessing(userID) {
		if msg != nil {
			del := tgbotapi.NewDeleteMessage(chatID, msg.MessageID)
			h.bot.Request(del)
		}
		warn := h.incProcessingWarn(userID)
		newText := t(lang, "⏳ Javob tayyorlayapman. Iltimos, bir oz kuting.", "⏳ Готовлю ответ. Пожалуйста, подождите.")
		if warn >= 2 {
			newText = t(lang, "⏳ Hali javob tayyor emas. Iltimos, kuting — tez orada yuboraman.", "⏳ Ответ ещё не готов. Пожалуйста, подождите — скоро отправлю.")
		}
		if waitMsg, ok := h.getWaitingMessage(userID); ok {
			edit := tgbotapi.NewEditMessageText(waitMsg.ChatID, waitMsg.MessageID, newText)
			h.bot.Request(edit)
		} else {
			if m, err := h.sendMessageWithResp(chatID, newText); err == nil {
				h.setWaitingMessage(userID, chatID, m.MessageID)
			}
		}
		return
	}

	// PC yig'ish so'rovi bo'lsa va hali reminder yuborilmagan bo'lsa
	// FAQAT birinchi marta reminder yuborish va to'xtatish
	isCfgReq := isConfigRequest(text)
	if isCfgReq && !h.wasConfigReminded(chatID) {
		h.markConfigReminded(chatID)
		h.sendConfigCTA(chatID, lang)
		return
	}

	// Submit to worker pool for parallel processing
	if !h.startProcessing(userID) {
		h.sendMessage(chatID, t(lang, "⏳ Oldingi so'rov yakunlanmoqda, iltimos kuting.", "⏳ Предыдущий запрос обрабатывается, подождите."))
		return
	}

	waitMsg, err := h.sendMessageWithResp(chatID, t(lang, "⏳ Iltimos, javobni kuting.", "⏳ Пожалуйста, подождите."))
	if err == nil {
		h.setWaitingMessage(userID, chatID, waitMsg.MessageID)
	}

	if ok := h.workerPool.submit(&messageRequest{
		ctx:      ctx,
		userID:   userID,
		username: username,
		text:     text,
		chatID:   chatID,
		message:  msg,
		lang:     lang,
	}); !ok {
		h.endProcessing(userID)
		h.clearWaitingMessage(userID)
		h.sendMessage(chatID, "⚠️ Bot band, birozdan so'ng qayta urinib ko'ring.")
	}
}
