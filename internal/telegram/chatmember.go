package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/betallsoph/shiftz/internal/store"
)

const (
	bindBroadcastPrefix = "bind_bc:"
	bindTeamPrefix      = "bind_team:"
	bindIgnorePrefix    = "bind_ignore:"
)

const (
	msgBindAsk = "Bot đã được thêm vào nhóm này với quyền admin.\n" +
		"Nhóm này dùng để làm gì?"

	msgBindBroadcastOK = "Đã gắn nhóm này làm nhóm Thông báo (lịch / vote)."

	msgBindTeamOK = "Đã gắn nhóm này làm Chat đội."

	msgBindIgnored = "Đã bỏ qua. Có thể thêm bot vào nhóm khác khi cần."

	msgBindNotOwner = "Chỉ chủ quán đã liên kết bot mới gắn nhóm được."

	msgBindExpired = "Yêu cầu gắn nhóm không còn hiệu lực."
)

// handleMyChatMember prompts the linked owner to classify a group when the bot
// becomes an admin there.
func (b *Bot) handleMyChatMember(ctx context.Context, u *ChatMemberUpdated) error {
	if u == nil {
		return nil
	}
	if !isGroupChat(u.Chat) {
		return nil
	}
	if !isAdminStatus(u.NewChatMember.Status) || isAdminStatus(u.OldChatMember.Status) {
		return nil
	}

	shop, err := b.shops.ByOwnerTelegramID(ctx, u.From.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("telegram: lookup owner for chat member: %w", err)
	}
	_ = shop // shop identity is re-resolved on callback via owner telegram id

	return b.api.SendMessage(ctx, u.Chat.ID, msgBindAsk, GroupBindKeyboard(u.Chat.ID))
}

func (b *Bot) handleBindCallback(ctx context.Context, q *CallbackQuery) error {
	chatID, kind, ok := parseBindCallback(q.Data)
	if !ok {
		return b.api.AnswerCallbackQuery(ctx, q.ID, msgBindExpired)
	}

	shop, err := b.shops.ByOwnerTelegramID(ctx, q.From.ID)
	if errors.Is(err, store.ErrNotFound) {
		return b.api.AnswerCallbackQuery(ctx, q.ID, msgBindNotOwner)
	}
	if err != nil {
		return fmt.Errorf("telegram: lookup owner for bind: %w", err)
	}

	switch kind {
	case "broadcast":
		if err := b.shops.BindTelegramGroup(ctx, shop.ID, chatID); err != nil {
			return fmt.Errorf("telegram: bind broadcast group: %w", err)
		}
		if err := b.api.AnswerCallbackQuery(ctx, q.ID, "Đã gắn Thông báo"); err != nil {
			return err
		}
		return b.api.SendMessage(ctx, callbackChatID(q, chatID), msgBindBroadcastOK, nil)
	case "team":
		if err := b.shops.BindTelegramTeamChat(ctx, shop.ID, chatID); err != nil {
			return fmt.Errorf("telegram: bind team chat: %w", err)
		}
		if err := b.api.AnswerCallbackQuery(ctx, q.ID, "Đã gắn Chat đội"); err != nil {
			return err
		}
		return b.api.SendMessage(ctx, callbackChatID(q, chatID), msgBindTeamOK, nil)
	case "ignore":
		if err := b.api.AnswerCallbackQuery(ctx, q.ID, "Đã bỏ qua"); err != nil {
			return err
		}
		return b.api.SendMessage(ctx, callbackChatID(q, chatID), msgBindIgnored, nil)
	default:
		return b.api.AnswerCallbackQuery(ctx, q.ID, msgBindExpired)
	}
}

// GroupBindKeyboard asks whether a group is broadcast, team, or ignored.
func GroupBindKeyboard(chatID int64) *InlineKeyboardMarkup {
	id := strconv.FormatInt(chatID, 10)
	return &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{
			{Text: "Thông báo", Data: bindBroadcastPrefix + id},
			{Text: "Chat đội", Data: bindTeamPrefix + id},
		},
		{
			{Text: "Bỏ qua", Data: bindIgnorePrefix + id},
		},
	}}
}

func parseBindCallback(data string) (chatID int64, kind string, ok bool) {
	switch {
	case strings.HasPrefix(data, bindBroadcastPrefix):
		kind = "broadcast"
		data = strings.TrimPrefix(data, bindBroadcastPrefix)
	case strings.HasPrefix(data, bindTeamPrefix):
		kind = "team"
		data = strings.TrimPrefix(data, bindTeamPrefix)
	case strings.HasPrefix(data, bindIgnorePrefix):
		kind = "ignore"
		data = strings.TrimPrefix(data, bindIgnorePrefix)
	default:
		return 0, "", false
	}
	id, err := strconv.ParseInt(data, 10, 64)
	if err != nil || id == 0 {
		return 0, "", false
	}
	return id, kind, true
}

func isAdminStatus(status string) bool {
	return status == "administrator" || status == "creator"
}

func isBindCallback(data string) bool {
	return strings.HasPrefix(data, bindBroadcastPrefix) ||
		strings.HasPrefix(data, bindTeamPrefix) ||
		strings.HasPrefix(data, bindIgnorePrefix)
}
