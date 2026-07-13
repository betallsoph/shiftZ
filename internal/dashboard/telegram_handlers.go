package dashboard

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/betallsoph/shiftz/internal/store"
)

const telegramSetupTTL = 30 * time.Minute

type shopTelegramSetup interface {
	RotateTelegramSetupCode(ctx context.Context, shopID uuid.UUID, expiresAt time.Time) (string, error)
}

// TelegramSetupView is the owner Telegram group connection panel.
type TelegramSetupView struct {
	Connected         bool
	TelegramGroupID   int64
	SetupCode         string
	SetupExpiresAt    string
	HasPendingSetup   bool
}

func buildTelegramSetupView(shop *store.Shop, plaintextCode string, expiresAt time.Time, now time.Time) TelegramSetupView {
	view := TelegramSetupView{
		Connected:       shop.TelegramGroupID != 0,
		TelegramGroupID: shop.TelegramGroupID,
	}
	if shop.TelegramSetupCodeExpiresAt != nil && shop.TelegramSetupCodeExpiresAt.After(now) {
		view.HasPendingSetup = true
	}
	if plaintextCode != "" && expiresAt.After(now) {
		view.SetupCode = plaintextCode
		loc, err := time.LoadLocation(shop.Timezone)
		if err != nil {
			loc = time.UTC
		}
		view.SetupExpiresAt = expiresAt.In(loc).Format("2006-01-02 15:04")
	}
	return view
}

func (s *Server) handleRotateTelegramSetupCode(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	shop, err := s.shops.ByID(r.Context(), sess.ShopID)
	if err != nil {
		s.log.Error("load shop for telegram setup", "err", err)
		http.Error(w, "shop error", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	expiresAt := now.Add(telegramSetupTTL)
	code, err := s.shopTelegram.RotateTelegramSetupCode(r.Context(), sess.ShopID, expiresAt)
	if err != nil {
		s.log.Error("rotate telegram setup code", "err", err)
		http.Error(w, "setup error", http.StatusInternalServerError)
		return
	}
	view := buildTelegramSetupView(shop, code, expiresAt, now)
	s.renderTelegramSetup(w, view)
}

func (s *Server) renderTelegramSetup(w http.ResponseWriter, view TelegramSetupView) {
	if err := s.tmpl.render(w, "telegram_setup.html", view); err != nil {
		s.log.Error("render telegram setup", "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
