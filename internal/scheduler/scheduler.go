package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm uint32) error
}

type Scheduler interface {
	Start(ctx context.Context, interval time.Duration, task func(context.Context)) error
	Stop(ctx context.Context) error
	Status(ctx context.Context) (time.Duration, bool, error)
}

type scheduler struct {
	fs          FileSystem
	filePath    string
	minInterval time.Duration

	mu     sync.Mutex
	ticker *time.Ticker
	stopCh chan struct{}
}

const offMarker = "off"

type Option func(*scheduler)

func WithMinInterval(d time.Duration) Option {
	return func(s *scheduler) {
		s.minInterval = d
	}
}

func New(fs FileSystem, filePath string, opts ...Option) *scheduler {
	s := &scheduler{fs: fs, filePath: filePath, minInterval: time.Hour}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *scheduler) Start(ctx context.Context, interval time.Duration, task func(context.Context)) error {
	if interval < s.minInterval {
		return fmt.Errorf("minimum interval is %v, got %v", s.minInterval, interval)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ticker != nil {
		s.ticker.Stop()
		if s.stopCh != nil {
			close(s.stopCh)
		}
	}

	data := fmt.Sprintf("%d", int64(interval.Seconds()))
	if err := s.fs.WriteFile(s.filePath, []byte(data), 0644); err != nil {
		return err
	}

	stopCh := make(chan struct{})
	ticker := time.NewTicker(interval)
	s.stopCh = stopCh
	s.ticker = ticker

	go func() {
		for {
			select {
			case <-ticker.C:
				task(ctx)
			case <-stopCh:
				ticker.Stop()
				return
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (s *scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.stopCh != nil {
		close(s.stopCh)
		s.stopCh = nil
	}
	s.ticker = nil
	s.mu.Unlock()

	return s.fs.WriteFile(s.filePath, []byte(offMarker), 0644)
}

func (s *scheduler) Status(ctx context.Context) (time.Duration, bool, error) {
	data, err := s.fs.ReadFile(s.filePath)
	if err != nil {
		return 0, false, nil
	}
	raw := strings.TrimSpace(string(data))
	if raw == offMarker || raw == "" {
		return 0, false, nil
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, nil
	}
	return time.Duration(secs) * time.Second, true, nil
}
