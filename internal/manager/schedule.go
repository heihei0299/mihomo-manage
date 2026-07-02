package manager

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (m *manager) SetSchedule(ctx context.Context, interval time.Duration) error {
	if interval < time.Hour {
		return fmt.Errorf("minimum interval is 1h, got %v", interval)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ticker != nil {
		m.ticker.Stop()
		if m.stopCh != nil {
			close(m.stopCh)
		}
	}

	data := fmt.Sprintf("%d", int64(interval.Seconds()))
	if err := m.sys.WriteFile(scheduleFile, []byte(data), filePermUserRW); err != nil {
		return err
	}

	stopCh := make(chan struct{})
	ticker := time.NewTicker(interval)
	m.stopCh = stopCh
	m.ticker = ticker
	go func() {
		for {
			select {
			case <-ticker.C:
				m.UpdateConfig(context.Background())
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (m *manager) StopSchedule(ctx context.Context) error {
	m.mu.Lock()
	if m.ticker != nil {
		m.ticker.Stop()
	}
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	m.mu.Unlock()
	m.sys.WriteFile(scheduleFile, []byte("off"), filePermUserRW)
	return nil
}

func (m *manager) ScheduleStatus(ctx context.Context) (time.Duration, bool, error) {
	data, err := m.sys.ReadFile(scheduleFile)
	if err != nil {
		return 0, false, nil
	}
	s := strings.TrimSpace(string(data))
	if s == "off" || s == "" {
		return 0, false, nil
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, nil
	}
	return time.Duration(secs) * time.Second, true, nil
}
