package main

import "context"

// telegramIncidentMessenger adapts telegram.Client to dashboard incident delivery.
type telegramIncidentMessenger struct {
	send func(ctx context.Context, chatID int64, text string) error
}

func (m telegramIncidentMessenger) SendMessage(ctx context.Context, chatID int64, text string) error {
	return m.send(ctx, chatID, text)
}
