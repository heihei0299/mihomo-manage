package scheduler

import (
	"context"
	"time"
)

const filePermUserRW = 0644

type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm uint32) error
	Remove(path string) error
}

type CommandRunner interface {
	RunCommand(ctx context.Context, name string, args ...string) (string, error)
}

type Platform interface {
	Set(ctx context.Context, interval time.Duration, commandPath string) error
	Stop(ctx context.Context) error
	Status(ctx context.Context) (time.Duration, bool, error)
}
