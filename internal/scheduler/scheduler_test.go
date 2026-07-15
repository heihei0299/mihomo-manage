package scheduler

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

type fakeFileSystem struct {
	mu      sync.Mutex
	written map[string][]byte
}

func (f *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.written == nil {
		return nil, os.ErrNotExist
	}
	data, ok := f.written[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (f *fakeFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.written == nil {
		f.written = make(map[string][]byte)
	}
	f.written[path] = data
	return nil
}

func TestStartWithValidInterval(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt")

	err := s.Start(context.Background(), time.Hour, func(ctx context.Context) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s.Stop(context.Background())
}

func TestStartBelowMinInterval(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt")

	err := s.Start(context.Background(), 30*time.Minute, func(ctx context.Context) {})
	if err == nil {
		t.Fatal("expected error for interval below 1h")
	}
}

func TestStopWritesOff(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt")

	s.Start(context.Background(), time.Hour, func(ctx context.Context) {})
	s.Stop(context.Background())

	data, err := fs.ReadFile("/tmp/schedule.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "off" {
		t.Errorf("expected 'off', got %q", string(data))
	}
}

func TestStatusInactiveWhenNoFile(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt")

	_, active, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected inactive when no file")
	}
}

func TestStatusActiveAfterStart(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt")

	s.Start(context.Background(), time.Hour, func(ctx context.Context) {})
	defer s.Stop(context.Background())

	interval, active, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !active {
		t.Fatal("expected active after start")
	}
	if interval != time.Hour {
		t.Errorf("expected 1h interval, got %v", interval)
	}
}

func TestTaskIsCalled(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt", WithMinInterval(time.Millisecond))

	called := make(chan struct{}, 5)
	task := func(ctx context.Context) {
		called <- struct{}{}
	}

	err := s.Start(context.Background(), 10*time.Millisecond, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("task was not called within timeout")
	}

	s.Stop(context.Background())
}

func TestStatusOff(t *testing.T) {
	fs := &fakeFileSystem{}
	fs.WriteFile("/tmp/schedule.txt", []byte("off"), 0644)

	s := New(fs, "/tmp/schedule.txt")

	_, active, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected inactive for 'off' file")
	}
}

func TestStatusEmpty(t *testing.T) {
	fs := &fakeFileSystem{}
	fs.WriteFile("/tmp/schedule.txt", []byte(""), 0644)

	s := New(fs, "/tmp/schedule.txt")

	_, active, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active {
		t.Error("expected inactive for empty file")
	}
}

func TestStopIsIdempotent(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt")

	s.Start(context.Background(), time.Hour, func(ctx context.Context) {})

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}
	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}
}

func TestStartReplacesRunning(t *testing.T) {
	fs := &fakeFileSystem{}
	s := New(fs, "/tmp/schedule.txt")

	calls := make(chan struct{}, 10)
	task := func(ctx context.Context) {
		calls <- struct{}{}
	}

	if err := s.Start(context.Background(), time.Hour, task); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	if err := s.Start(context.Background(), 2*time.Hour, task); err != nil {
		t.Fatalf("second Start failed: %v", err)
	}

	interval, active, err := s.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !active {
		t.Fatal("expected active")
	}
	if interval != 2*time.Hour {
		t.Errorf("expected 2h interval, got %v", interval)
	}

	s.Stop(context.Background())
}
