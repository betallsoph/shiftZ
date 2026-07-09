// Package telegram implements shiftbot's Telegram bot: availability intake,
// the weekly reminder flow, schedule voting via inline buttons, and owner
// approval/veto. The bot runs in webhook mode; see WebhookHandler.
//
// Dependencies on the store and LLM layers are expressed as small interfaces
// so handlers stay testable and wiring happens in cmd/bot.
package telegram

// Update is an incoming Telegram update (the subset shiftbot handles).
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// Message is an incoming chat message.
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text,omitempty"`
}

// User is a Telegram account.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Chat is the conversation a message belongs to.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// CallbackQuery is a press on an inline keyboard button (used for schedule
// voting and owner approval).
type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

// InlineKeyboardMarkup renders buttons under a message.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// InlineKeyboardButton is one button; Data is echoed back in the
// CallbackQuery when pressed.
type InlineKeyboardButton struct {
	Text string `json:"text"`
	Data string `json:"callback_data"`
}
