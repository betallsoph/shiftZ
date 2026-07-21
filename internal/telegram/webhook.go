package telegram

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// WebhookHandler returns the HTTP handler Telegram calls with updates
// (webhook mode). If secret is non-empty, the
// X-Telegram-Bot-Api-Secret-Token header must match — set the same value
// when registering the webhook via setWebhook.
//
// Ops: when calling setWebhook, include my_chat_member in allowed_updates
// so group-bind prompts work, e.g.:
//
//	allowed_updates=["message","callback_query","my_chat_member"]
//
// Handlers always answer 200 quickly; processing errors are logged, not
// returned, so Telegram doesn't endlessly retry poisoned updates.
func WebhookHandler(bot *Bot, secret string, log *slog.Logger) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if secret != "" && r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != secret {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var u Update
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := bot.HandleUpdate(r.Context(), u); err != nil {
			log.Error("update handling failed", "update_id", u.UpdateID, "err", err)
		}
		w.WriteHeader(http.StatusOK)
	})
}
