package scheduler

import (
	"context"
	"errors"
	"time"
)

// Notifier sends messages to Telegram chats. Satisfied by *telegram.Client
// via a thin adapter (the client method takes an extra markup argument).
type Notifier interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// ReminderTargets lists who should receive availability reminders.
// Satisfied by *store.EmployeeRepo (ActiveTelegramIDs).
type ReminderTargets interface {
	ActiveTelegramIDs(ctx context.Context) ([]int64, error)
}

// WeeklyReminderJob asks every active employee for next week's availability.
type WeeklyReminderJob struct {
	Weekday time.Weekday // when to fire, e.g. time.Thursday
	Hour    int          // 24h clock, runner's local time

	Targets ReminderTargets
	Notify  Notifier
}

func (j *WeeklyReminderJob) Name() string { return "weekly-availability-reminder" }

func (j *WeeklyReminderJob) Due(t time.Time) bool {
	return t.Weekday() == j.Weekday && t.Hour() == j.Hour && t.Minute() == 0
}

func (j *WeeklyReminderJob) Run(ctx context.Context) error {
	ids, err := j.Targets.ActiveTelegramIDs(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, chatID := range ids {
		if err := j.Notify.SendMessage(ctx, chatID,
			"Hey! Time to send me your availability for next week — plain language is fine."); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// NagJob re-pings employees who haven't submitted availability yet.
// Skeleton: needs a store query for non-responders of the upcoming week.
type NagJob struct {
	Weekday time.Weekday
	Hour    int
}

func (j *NagJob) Name() string { return "nag-non-responders" }

func (j *NagJob) Due(t time.Time) bool {
	return t.Weekday() == j.Weekday && t.Hour() == j.Hour && t.Minute() == 0
}

func (j *NagJob) Run(ctx context.Context) error {
	// TODO: query employees without availability rows for the upcoming week
	// and remind only them.
	return nil
}

// FinalizeJob closes voting, picks the winning candidate (owner veto beats
// votes), marks the schedule final and announces it.
type FinalizeJob struct {
	Weekday time.Weekday
	Hour    int
}

func (j *FinalizeJob) Name() string { return "close-voting-and-finalize" }

func (j *FinalizeJob) Due(t time.Time) bool {
	return t.Weekday() == j.Weekday && t.Hour() == j.Hour && t.Minute() == 0
}

func (j *FinalizeJob) Run(ctx context.Context) error {
	// TODO: tally schedule_votes per shop, respect owner approval/veto,
	// set schedules.status = 'final' and send the announcement (formatted
	// via llm.Service.FormatAnnouncement) to the shop's chat.
	return nil
}
