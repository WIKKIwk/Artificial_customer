package telegram

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	userHistoryInlinePrefix       = "user_chat_history:"
	userHistoryInlineResultsLimit = 20
	userHistorySelectionTag       = "user_id="
	userHistoryDefaultLimit       = 20
)

type userSearchStore interface {
	SearchUsers(ctx context.Context, query string, limit int) ([]chatUserEntry, error)
}

func (h *BotHandler) setAwaitingUserHistory(userID int64, awaiting bool) {
	h.userHistoryMu.Lock()
	if awaiting {
		h.userHistoryAwait[userID] = true
	} else {
		delete(h.userHistoryAwait, userID)
	}
	h.userHistoryMu.Unlock()
}

func (h *BotHandler) isAwaitingUserHistory(userID int64) bool {
	h.userHistoryMu.RLock()
	awaiting := h.userHistoryAwait[userID]
	h.userHistoryMu.RUnlock()
	return awaiting
}

func (h *BotHandler) handleUserChatHistoryInlineQuery(ctx context.Context, query *tgbotapi.InlineQuery) {
	if query == nil || query.From == nil {
		return
	}
	userID := query.From.ID
	if !h.isAwaitingUserHistory(userID) {
		h.answerInlineQuery(query.ID, nil)
		return
	}
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		h.answerInlineQuery(query.ID, nil)
		return
	}
	q := strings.TrimSpace(query.Query)
	if q == "" {
		h.answerInlineQuery(query.ID, nil)
		return
	}

	users, err := h.searchUserHistoryCandidates(ctx, q, userHistoryInlineResultsLimit)
	if err != nil || len(users) == 0 {
		h.answerInlineQuery(query.ID, nil)
		return
	}

	lang := h.getUserLang(userID)
	now := time.Now()
	results := make([]interface{}, 0, len(users))
	for _, entry := range users {
		title := userHistoryDisplayName(entry)
		messageText := fmt.Sprintf("✅ Tanlandi: %s%d", userHistorySelectionTag, entry.UserID)
		if entry.Username != "" {
			messageText += " @" + entry.Username
		}
		resultID := userHistoryInlinePrefix + strconv.FormatInt(entry.UserID, 10)
		result := tgbotapi.NewInlineQueryResultArticle(resultID, title, messageText)

		descParts := []string{fmt.Sprintf("ID:%d", entry.UserID)}
		if !entry.LastSeen.IsZero() {
			descParts = append(descParts, fmt.Sprintf("%s: %s", t(lang, "oxirgi", "посл."), formatAgo(now.Sub(entry.LastSeen), lang)))
		}
		result.Description = strings.Join(descParts, " | ")
		results = append(results, result)
	}

	h.answerInlineQuery(query.ID, results)
}

func (h *BotHandler) handleUserChatHistoryInlineSelectionMessage(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg == nil || msg.From == nil {
		return false
	}
	userID := msg.From.ID
	if !h.isAwaitingUserHistory(userID) {
		return false
	}
	isAdmin, _ := h.adminUseCase.IsAdmin(ctx, userID)
	if !isAdmin {
		return false
	}

	selectedID := extractUserHistorySelectionID(msg.Text)
	if selectedID == 0 {
		return false
	}

	h.setAwaitingUserHistory(userID, false)
	h.sendUserChatHistory(ctx, msg.Chat.ID, strconv.FormatInt(selectedID, 10), userHistoryDefaultLimit)
	return true
}

func (h *BotHandler) searchUserHistoryCandidates(ctx context.Context, query string, limit int) ([]chatUserEntry, error) {
	if limit <= 0 {
		limit = userHistoryInlineResultsLimit
	}
	if h.chatStore != nil {
		if store, ok := h.chatStore.(userSearchStore); ok {
			res, err := store.SearchUsers(ctx, query, limit)
			if err != nil {
				return nil, err
			}
			res = filterUserEntriesSinceStart(res, h.botStartedAt)
			if len(res) > 0 {
				return res, nil
			}
		}
	}
	return h.searchUsersFromLastSeen(query, limit), nil
}

func (h *BotHandler) searchUsersFromLastSeen(query string, limit int) []chatUserEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var list []chatUserEntry

	h.lastSeenMu.RLock()
	h.nameMu.RLock()
	for id, ts := range h.lastSeen {
		if !h.botStartedAt.IsZero() && ts.Before(h.botStartedAt) {
			continue
		}
		name := strings.TrimSpace(h.lastName[id])
		if !matchUserQuery(id, name, q) {
			continue
		}
		list = append(list, chatUserEntry{
			UserID:   id,
			Username: name,
			LastSeen: ts,
		})
	}
	h.nameMu.RUnlock()
	h.lastSeenMu.RUnlock()

	sortChatUserEntries(list)
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

func userHistoryDisplayName(entry chatUserEntry) string {
	if entry.Username == "" {
		return fmt.Sprintf("ID:%d", entry.UserID)
	}
	if strings.HasPrefix(entry.Username, "@") {
		return entry.Username
	}
	return "@" + entry.Username
}

func extractUserHistorySelectionID(text string) int64 {
	idx := strings.Index(text, userHistorySelectionTag)
	if idx < 0 {
		return 0
	}
	rest := text[idx+len(userHistorySelectionTag):]
	var digits strings.Builder
	for _, r := range rest {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return 0
	}
	id, ok := parseUserID(digits.String())
	if !ok {
		return 0
	}
	return id
}

func matchUserQuery(userID int64, username, query string) bool {
	if strings.Contains(strconv.FormatInt(userID, 10), query) {
		return true
	}
	if username == "" {
		return false
	}
	return strings.Contains(strings.ToLower(username), query)
}

func sortChatUserEntries(entries []chatUserEntry) {
	for i := range entries {
		if entries[i].Username != "" && strings.HasPrefix(entries[i].Username, "@") {
			entries[i].Username = strings.TrimPrefix(entries[i].Username, "@")
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastSeen.After(entries[j].LastSeen)
	})
}

func filterUserEntriesSinceStart(entries []chatUserEntry, startedAt time.Time) []chatUserEntry {
	if startedAt.IsZero() {
		return entries
	}
	out := entries[:0]
	for _, entry := range entries {
		if entry.LastSeen.IsZero() || entry.LastSeen.Before(startedAt) {
			continue
		}
		out = append(out, entry)
	}
	return out
}
