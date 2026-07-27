package schedmgr

import (
	"context"
	"os"
	"testing"
	"time"
)

// fakeFileSystem implements infra.FileSystem for testing.
type fakeFileSystem struct {
	fileExists  map[string]bool
	written     map[string][]byte
	writeErr    error
	removeErr   error
	renameErr   error
	removed     []string
	renamed     map[string]string
	readFileErr map[string]error
}

func (m *fakeFileSystem) FileExists(path string) bool {
	return m.fileExists[path]
}

func (m *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	if m.readFileErr != nil {
		if err, ok := m.readFileErr[path]; ok {
			return nil, err
		}
	}
	if m.written == nil {
		return nil, os.ErrNotExist
	}
	data, ok := m.written[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (m *fakeFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	if m.written == nil {
		m.written = make(map[string][]byte)
	}
	m.written[path] = data
	return nil
}

func (m *fakeFileSystem) Remove(path string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removed = append(m.removed, path)
	return nil
}

func (m *fakeFileSystem) Rename(oldPath, newPath string) error {
	if m.renameErr != nil {
		return m.renameErr
	}
	if m.renamed == nil {
		m.renamed = make(map[string]string)
	}
	m.renamed[oldPath] = newPath
	if m.fileExists == nil {
		m.fileExists = make(map[string]bool)
	}
	if m.written != nil {
		if data, ok := m.written[oldPath]; ok {
			m.written[newPath] = data
			delete(m.written, oldPath)
		}
	}
	m.fileExists[newPath] = true
	delete(m.fileExists, oldPath)
	return nil
}

func (m *fakeFileSystem) MkdirAll(path string, perm uint32) error {
	return nil
}

func (m *fakeFileSystem) Chmod(path string, perm uint32) error {
	return nil
}

func TestScheduleSetAndStop(t *testing.T) {
	fs := &fakeFileSystem{}
	m := NewManager(fs, func(ctx context.Context) {})

	err := m.SetSchedule(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("SetSchedule failed: %v", err)
	}

	interval, active, err := m.ScheduleStatus(context.Background())
	if err != nil {
		t.Fatalf("ScheduleStatus failed: %v", err)
	}
	if !active {
		t.Error("expected schedule to be active")
	}
	if interval != time.Hour {
		t.Errorf("expected interval 1h, got %v", interval)
	}

	err = m.StopSchedule(context.Background())
	if err != nil {
		t.Fatalf("StopSchedule failed: %v", err)
	}

	_, active, err = m.ScheduleStatus(context.Background())
	if err != nil {
		t.Fatalf("ScheduleStatus after stop failed: %v", err)
	}
	if active {
		t.Error("expected schedule to be inactive after stop")
	}
}

func TestScheduleRejectsShortInterval(t *testing.T) {
	fs := &fakeFileSystem{}
	m := NewManager(fs, func(ctx context.Context) {})

	err := m.SetSchedule(context.Background(), time.Minute)
	if err == nil {
		t.Fatal("expected error for interval < 1h")
	}
}
