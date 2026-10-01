package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type windowsServiceStrategy struct{}

func newWindowsServiceStrategy() osStrategy { return windowsServiceStrategy{} }

func checkWindowsServiceTarget(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager, service, err := openWindowsService(name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return err
	}
	service.Close()
	manager.Disconnect()
	return ErrMihomoAlreadyInstalled
}

func openWindowsService(name string) (*mgr.Mgr, *mgr.Service, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to Windows service manager: %w", err)
	}
	service, err := manager.OpenService(name)
	if err != nil {
		manager.Disconnect()
		return nil, nil, err
	}
	return manager, service, nil
}

func (w windowsServiceStrategy) isActive(ctx context.Context, name string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	manager, service, err := openWindowsService(name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer manager.Disconnect()
	defer service.Close()
	status, err := service.Query()
	return status.State == svc.Running, err
}

func (w windowsServiceStrategy) enable(ctx context.Context, name, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	service, err := manager.CreateService(name, executable, mgr.Config{DisplayName: "Mihomo proxy", Description: "Mihomo core managed by mihomo-manager", StartType: mgr.StartManual}, "service-run")
	if err != nil {
		return fmt.Errorf("registering Windows service: %w", err)
	}
	if err := service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 5 * time.Second}}, 86400); err != nil {
		return errors.Join(err, service.Delete(), service.Close())
	}
	if err := service.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return errors.Join(err, service.Delete(), service.Close())
	}
	return service.Close()
}

func (w windowsServiceStrategy) disable(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager, service, err := openWindowsService(name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	defer service.Close()
	return service.Delete()
}

func waitWindowsService(ctx context.Context, service *mgr.Service, desired svc.State) error {
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == desired {
			return nil
		}
		if desired == svc.Running && status.State == svc.Stopped {
			return fmt.Errorf("Windows service stopped during startup (exit code %d)", status.Win32ExitCode)
		}
		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func (w windowsServiceStrategy) start(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager, service, err := openWindowsService(name)
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Running {
		return nil
	}
	if status.State != svc.StartPending {
		if err := service.Start(); err != nil {
			return err
		}
	}
	return waitWindowsService(ctx, service, svc.Running)
}

func (w windowsServiceStrategy) stop(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager, service, err := openWindowsService(name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return err
	}
	if status.State == svc.Stopped {
		return nil
	}
	if status.State != svc.StopPending {
		if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
			return err
		}
	}
	return waitWindowsService(ctx, service, svc.Stopped)
}

func (w windowsServiceStrategy) restart(ctx context.Context, name string) error {
	if err := w.stop(ctx, name); err != nil {
		return err
	}
	return w.start(ctx, name)
}

func (w windowsServiceStrategy) reload(ctx context.Context, name string) error {
	// Windows has no SIGHUP; restart the hosted core and wait for Running.
	return w.restart(ctx, name)
}

func (w windowsServiceStrategy) isEnabled(ctx context.Context, name string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	manager, service, err := openWindowsService(name)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer manager.Disconnect()
	defer service.Close()
	config, err := service.Config()
	return config.StartType == mgr.StartAutomatic, err
}

func updateWindowsServiceStart(ctx context.Context, name string, startType uint32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager, service, err := openWindowsService(name)
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	defer service.Close()
	config, err := service.Config()
	if err != nil {
		return err
	}
	config.StartType = startType
	return service.UpdateConfig(config)
}

func (w windowsServiceStrategy) enableAutoStart(ctx context.Context, name, _ string) error {
	return updateWindowsServiceStart(ctx, name, mgr.StartAutomatic)
}

func (w windowsServiceStrategy) disableAutoStart(ctx context.Context, name string) error {
	return updateWindowsServiceStart(ctx, name, mgr.StartManual)
}
