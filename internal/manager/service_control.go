package manager

import "context"

type ServiceControl interface {
	Status(ctx context.Context) (*Status, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Reload(ctx context.Context) error
	SetAutoStart(ctx context.Context, enabled bool) error
}
