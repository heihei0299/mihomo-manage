package manager

import "context"

type noopConfigUpdateLock struct{}

func (noopConfigUpdateLock) Acquire(context.Context) (func(), error) {
	return func() {}, nil
}

func newTestConfigManager(fs FileSystem, source ReleaseSource, validate ConfigValidator, onReload func(context.Context) error, options ...configManagerOption) ConfigManager {
	options = append([]configManagerOption{WithConfigUpdateLock(noopConfigUpdateLock{})}, options...)
	return NewConfigManager(fs, source, validate, onReload, options...)
}
