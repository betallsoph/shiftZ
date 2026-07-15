package dashboard

import (
	"github.com/betallsoph/shiftz/internal/store"
)

// TelegramSetupView is the owner Telegram group connection status in the employees panel.
type TelegramSetupView struct {
	Connected       bool
	TelegramGroupID int64
}

func buildTelegramSetupView(shop *store.Shop) TelegramSetupView {
	return TelegramSetupView{
		Connected:       shop.TelegramGroupID != 0,
		TelegramGroupID: shop.TelegramGroupID,
	}
}
