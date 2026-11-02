package manager

import (
	"context"
	"time"
)

type ScheduleManager interface {
	SetSchedule(ctx context.Context, interval time.Duration) error
	StopSchedule(ctx context.Context) error
	ScheduleStatus(ctx context.Context) (time.Duration, bool, error)
	// ScheduleAdvice reports a maintainer-facing warning about the registered
	// schedule, such as a task still invoking a command whose behavior changed.
	ScheduleAdvice(ctx context.Context) (string, error)
}
