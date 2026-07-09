// Package scheduler runs shiftbot's recurring jobs on a simple cron-style
// runner: the weekly availability reminder, nagging non-responders, and
// closing the vote to finalize the schedule.
//
// The runner ticks once per minute and fires every job whose Due method
// matches the current minute. Job dependencies are expressed as small
// interfaces so wiring stays in cmd.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Job is one recurring task.
type Job interface {
	Name() string
	// Due reports whether the job should run at t. The runner evaluates it
	// once per minute, so implementations match on weekday/hour/minute.
	Due(t time.Time) bool
	Run(ctx context.Context) error
}

// Runner drives jobs on a one-minute tick.
type Runner struct {
	jobs []Job
	log  *slog.Logger
	now  func() time.Time
	tick time.Duration
}

// NewRunner builds a runner over the given jobs.
func NewRunner(log *slog.Logger, jobs ...Job) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{jobs: jobs, log: log, now: time.Now, tick: time.Minute}
}

// Start blocks, firing due jobs each minute, until ctx is cancelled.
func (r *Runner) Start(ctx context.Context) {
	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := r.now()
			for _, job := range r.jobs {
				if !job.Due(now) {
					continue
				}
				r.log.Info("running job", "job", job.Name())
				if err := job.Run(ctx); err != nil {
					r.log.Error("job failed", "job", job.Name(), "err", err)
				}
			}
		}
	}
}
