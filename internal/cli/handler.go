package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/anomalyco/mihomo-manager/internal/manager"
)

type Handler struct {
	control   manager.ServiceControl
	lifecycle manager.LifecycleManager
	config    manager.ConfigManager
	schedule  manager.ScheduleManager
	stdout    io.Writer
	stderr    io.Writer
}

func New(ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, config manager.ConfigManager, sched manager.ScheduleManager, stdout, stderr io.Writer) *Handler {
	return &Handler{control: ctrl, lifecycle: lifecycle, config: config, schedule: sched, stdout: stdout, stderr: stderr}
}

func (h *Handler) printf(format string, a ...interface{}) {
	fmt.Fprintf(h.stdout, format, a...)
}

func (h *Handler) println(a ...interface{}) {
	fmt.Fprintln(h.stdout, a...)
}

func (h *Handler) errorf(format string, a ...interface{}) {
	fmt.Fprintf(h.stderr, format, a...)
}

func (h *Handler) Status(ctx context.Context) int {
	status, err := h.control.Status(ctx)
	if err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}

	if !status.Installed {
		h.println("mihomo: not installed")
		return 2
	}

	autostart := "off"
	if status.AutoStartEnabled {
		autostart = "on"
	}

	switch status.InstanceState {
	case manager.Running:
		h.printf("mihomo: running  (version: %s)  autostart: %s\n", status.Version, autostart)
		return 0
	case manager.Stopped:
		h.printf("mihomo: stopped  (version: %s)  autostart: %s\n", status.Version, autostart)
		return 1
	case manager.Upgrading:
		h.println("mihomo: upgrading")
		return 1
	default:
		h.printf("mihomo: %s\n", strings.ToLower(status.InstanceState.String()))
		return 1
	}
}

func (h *Handler) AutoStart(ctx context.Context, enabled bool) int {
	if err := h.control.SetAutoStart(ctx, enabled); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	state := "off"
	if enabled {
		state = "on"
	}
	h.printf("auto-start set to %s\n", state)
	return 0
}

func (h *Handler) Install(ctx context.Context, version string, autoStart bool) int {
	err := h.lifecycle.Install(ctx, version, autoStart, func(e manager.ProgressEvent) {
		prefix := "  "
		if e.Error != nil {
			prefix = "✗ "
		} else if strings.HasSuffix(e.Message, "complete") || strings.HasSuffix(e.Message, "running") {
			prefix = "✓ "
		}
		h.printf("%s[%s] %s\n", prefix, e.Phase, e.Message)
	})
	if err != nil {
		h.errorf("install failed: %v\n", err)
		return 1
	}
	h.println("mihomo installed successfully")
	return 0
}

func (h *Handler) Uninstall(ctx context.Context, keepBackup bool) int {
	err := h.lifecycle.Uninstall(ctx, keepBackup, func(e manager.ProgressEvent) {
		prefix := "  "
		if e.Error != nil {
			prefix = "✗ "
		}
		h.printf("%s[%s] %s\n", prefix, e.Phase, e.Message)
	})
	if err != nil {
		h.errorf("uninstall failed: %v\n", err)
		return 1
	}
	h.println("mihomo uninstalled")
	return 0
}

func (h *Handler) Upgrade(ctx context.Context, version string) int {
	err := h.lifecycle.Upgrade(ctx, version, func(e manager.ProgressEvent) {
		prefix := "  "
		if e.Error != nil {
			prefix = "✗ "
		}
		h.printf("%s[%s] %s\n", prefix, e.Phase, e.Message)
	})
	if err != nil {
		h.errorf("upgrade failed: %v\n", err)
		return 1
	}
	h.println("upgrade complete")
	return 0
}

func (h *Handler) Start(ctx context.Context) int {
	if err := h.control.Start(ctx); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println("mihomo started")
	return 0
}

func (h *Handler) Stop(ctx context.Context) int {
	if err := h.control.Stop(ctx); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println("mihomo stopped")
	return 0
}

func (h *Handler) Restart(ctx context.Context) int {
	if err := h.control.Restart(ctx); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println("mihomo restarted")
	return 0
}

func (h *Handler) Reload(ctx context.Context) int {
	if err := h.control.Reload(ctx); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println("mihomo reloaded")
	return 0
}

func (h *Handler) PreviewConfig(ctx context.Context) int {
	preview, err := h.config.PreviewConfig(ctx)
	if err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println(preview)
	return 0
}

func (h *Handler) SetSubscription(ctx context.Context, data string) int {
	if err := h.config.SetSubscriptionSource(ctx, data); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println("subscription saved")
	return 0
}

func (h *Handler) UpdateConfig(ctx context.Context) int {
	if err := h.config.UpdateConfig(ctx); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println("config updated")
	return 0
}

func (h *Handler) ScheduleStatus(ctx context.Context) int {
	interval, active, err := h.schedule.ScheduleStatus(ctx)
	if err != nil {
		// not treated as error — the file may not exist
	}
	if active {
		h.printf("schedule: every %v\n", interval)
	} else {
		h.println("schedule: off")
	}
	return 0
}

func (h *Handler) SetSchedule(ctx context.Context, interval time.Duration) int {
	if err := h.schedule.SetSchedule(ctx, interval); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.printf("schedule set to every %v\n", interval)
	return 0
}

func (h *Handler) StopSchedule(ctx context.Context) int {
	if err := h.schedule.StopSchedule(ctx); err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	h.println("schedule stopped")
	return 0
}

func (h *Handler) Versions(ctx context.Context) int {
	versions, err := h.lifecycle.ListVersions(ctx)
	if err != nil {
		h.errorf("error: %v\n", err)
		return 1
	}
	for _, v := range versions {
		h.println(v.Tag)
	}
	return 0
}
