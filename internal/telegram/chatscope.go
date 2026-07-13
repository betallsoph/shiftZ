package telegram

// isGroupChat reports whether updates from this chat should use group-only handlers.
func isGroupChat(c Chat) bool {
	return c.Type == "group" || c.Type == "supergroup"
}

// isPrivateChat reports whether updates from this chat are one-to-one with the bot.
func isPrivateChat(c Chat) bool {
	return c.Type == "private"
}

func callbackChat(q *CallbackQuery) Chat {
	if q != nil && q.Message != nil {
		return q.Message.Chat
	}
	return Chat{}
}
