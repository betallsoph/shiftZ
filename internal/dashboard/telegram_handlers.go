package dashboard

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

// ownerLinkIssuer issues one-time owner Telegram deep-link tokens.
type ownerLinkIssuer interface {
	IssueOwnerLinkToken(ctx context.Context, shopID uuid.UUID) (string, error)
}

// TelegramPanelView is the dedicated Telegram linking tab.
type TelegramPanelView struct {
	IsActive bool
	Owner    TelegramSetupView
}

// TelegramSetupView is the owner Telegram connection status fragment
// (partial: telegram_owner_setup.html, swap target #telegram-setup).
type TelegramSetupView struct {
	OwnerLinked        bool
	OwnerTelegramID    int64
	BroadcastConnected bool
	TelegramGroupID    int64
	TeamChatConnected  bool
	TelegramTeamChatID int64
	OwnerLinkURL       string
	Error              string
	Notice             string
}

func buildTelegramSetupView(shop *store.Shop) TelegramSetupView {
	return TelegramSetupView{
		OwnerLinked:        shop.OwnerTelegramID != 0,
		OwnerTelegramID:    shop.OwnerTelegramID,
		BroadcastConnected: shop.TelegramGroupID != 0,
		TelegramGroupID:    shop.TelegramGroupID,
		TeamChatConnected:  shop.TelegramTeamChatID != 0,
		TelegramTeamChatID: shop.TelegramTeamChatID,
	}
}

func (s *Server) renderTelegramSetup(ctx context.Context, shopID uuid.UUID, notice, errMsg string, w http.ResponseWriter) {
	shop, err := s.shops.ByID(ctx, shopID)
	if err != nil {
		s.log.Error("load shop for telegram setup", "err", err)
		s.renderTelegramSetupView(w, TelegramSetupView{Error: "không tải được trạng thái Telegram"})
		return
	}
	view := buildTelegramSetupView(shop)
	view.Notice = notice
	view.Error = errMsg
	s.renderTelegramSetupView(w, view)
}

func (s *Server) renderTelegramSetupView(w http.ResponseWriter, view TelegramSetupView) {
	// Render owner fragment only so HTMX outerHTML swaps #telegram-setup
	// without replacing the Telegram tab panel / is-active state.
	if err := s.tmpl.render(w, "telegram_owner_setup.html", view); err != nil {
		s.log.Error("render telegram owner setup", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleOwnerTelegramLink(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	if s.ownerLinks == nil {
		s.renderTelegramSetup(r.Context(), sess.ShopID, "", "chưa cấu hình liên kết Telegram", w)
		return
	}
	if s.botUsername == "" {
		s.renderTelegramSetup(r.Context(), sess.ShopID, "", "chưa cấu hình bot Telegram", w)
		return
	}

	token, err := s.ownerLinks.IssueOwnerLinkToken(r.Context(), sess.ShopID)
	if err != nil {
		s.log.Error("issue owner telegram link", "err", err)
		s.renderTelegramSetup(r.Context(), sess.ShopID, "", "không tạo được link liên kết", w)
		return
	}

	shop, err := s.shops.ByID(r.Context(), sess.ShopID)
	if err != nil {
		s.log.Error("load shop after owner link", "err", err)
		s.renderTelegramSetupView(w, TelegramSetupView{Error: "không tải được trạng thái Telegram"})
		return
	}
	view := buildTelegramSetupView(shop)
	view.OwnerLinkURL = ownerTelegramLink(s.botUsername, token)
	view.Notice = "Link đã sẵn sàng — mở Telegram để hoàn tất liên kết (hết hạn sau 15 phút)."
	s.renderTelegramSetupView(w, view)
}

func (s *Server) handleTelegramStatusRefresh(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	s.renderTelegramSetup(r.Context(), sess.ShopID, "đã làm mới trạng thái", "", w)
}
