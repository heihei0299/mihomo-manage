package manager

import (
	"context"
	"time"
)

type ScheduleManager interface {
	SetSchedule(ctx context.Context, interval time.Duration) error
	StopSchedule(ctx context.Context) error
	ScheduleStatus(ctx context.Context) (time.Duration, bool, error)
}
