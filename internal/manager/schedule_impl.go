package manager

import (
	"context"
	"time"

	"mihomo-manager/internal/scheduler"
)

type scheduleManager struct {
	fs        FileSystem
	scheduler scheduler.Scheduler
	taskFn    func(ctx context.Context)
}

func NewScheduleManager(fs FileSystem, task func(ctx context.Context)) ScheduleManager {
	return &scheduleManager{
		fs:        fs,
		scheduler: scheduler.New(fs, scheduleFile),
		taskFn:    task,
	}
}

func (m *scheduleManager) SetSchedule(ctx context.Context, interval time.Duration) error {
	return m.scheduler.Start(ctx, interval, m.taskFn)
}

func (m *scheduleManager) StopSchedule(ctx context.Context) error {
	return m.scheduler.Stop(ctx)
}

func (m *scheduleManager) ScheduleStatus(ctx context.Context) (time.Duration, bool, error) {
	return m.scheduler.Status(ctx)
}
