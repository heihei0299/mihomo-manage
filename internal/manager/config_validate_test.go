package manager

import (
	"context"
	"errors"
	"testing"
)

// failValidator is a ConfigValidator that always fails with a fixed error.
type failValidator struct {
	err error
}

func (v *failValidator) Validate(ctx context.Context, configPath string) error {
	return v.err
}

// recordingValidator records the config path it validated and can fail on demand.
type recordingValidator struct {
	path string
	err  error
}

func (v *recordingValidator) Validate(ctx context.Context, configPath string) error {
	v.path = configPath
	return v.err
}

func TestConfigManagerValidateConfigPropagatesFailure(t *testing.T) {
	fs := &fakeFileSystem{}
	want := errors.New("config validation failed")
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &failValidator{err: want}, noopReload)

	err := m.ValidateConfig(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("expected error %v to propagate, got %v", want, err)
	}
}

func TestConfigManagerValidateConfigSuccessNoSideEffects(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{configYAML: true},
	}
	val := &recordingValidator{}
	reloaded := false
	m := newTestConfigManager(fs, &fakeReleaseSource{}, val, func(ctx context.Context) error {
		reloaded = true
		return nil
	})

	if err := m.ValidateConfig(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.path != configYAML {
		t.Errorf("validator should receive config path %s, got %q", configYAML, val.path)
	}
	if reloaded {
		t.Error("ValidateConfig should not trigger reload")
	}
	if len(fs.written) != 0 {
		t.Errorf("ValidateConfig should not write files, got %v", fs.written)
	}
}

func TestConfigManagerValidateConfigExplicitNoopValidatorPasses(t *testing.T) {
	fs := &fakeFileSystem{}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.ValidateConfig(context.Background()); err != nil {
		t.Fatalf("no-op validator should pass validation, got %v", err)
	}
}
