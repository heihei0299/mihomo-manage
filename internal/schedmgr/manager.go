package schedmgr

import (
	"context"
	"time"

	"github.com/anomalyco/mihomo-manager/internal/config"
	"github.com/anomalyco/mihomo-manager/internal/domain"
	"github.com/anomalyco/mihomo-manager/internal/infra"
	"github.com/anomalyco/mihomo-manager/internal/scheduler"
)

type scheduleManager struct {
	fs        infra.FileSystem
	scheduler scheduler.Scheduler
	taskFn    func(ctx context.Context)
}

func NewManager(fs infra.FileSystem, task func(ctx context.Context)) domain.ScheduleManager {
	return &scheduleManager{
		fs:        fs,
		scheduler: scheduler.New(fs, config.ScheduleFile),
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
