package manager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ConfigValidator interface {
	Validate(ctx context.Context, configPath string) error
}

type ConfigUpdateLock interface {
	Acquire(ctx context.Context) (release func(), err error)
}

type ConfigManagerOption func(*ConfigPipelineOptions)

func WithConfigUpdateLock(lock ConfigUpdateLock) ConfigManagerOption {
	return func(opts *ConfigPipelineOptions) { opts.Lock = lock }
}

var ErrConfigUpdateBusy = errors.New("configuration update already in progress")

type noopConfigUpdateLock struct{}

func (noopConfigUpdateLock) Acquire(context.Context) (func(), error) {
	return func() {}, nil
}

type configValidator struct{}

func (v *configValidator) Validate(ctx context.Context, configPath string) error {
	cmd := exec.CommandContext(ctx, binaryPath, "-t", "-d", filepath.Dir(configPath))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("config validation failed:\n%s", string(out))
	}
	return nil
}

type ConfigPipeline interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	Preview(ctx context.Context) (string, error)
	Apply(ctx context.Context) error
	LastConfigApply(ctx context.Context) (ConfigApplyStatus, error)
}

type ConfigPipelineOptions struct {
	OnReload  func(ctx context.Context) error
	Validator ConfigValidator
	Warn      func(msg string)
	Lock      ConfigUpdateLock
}

type configPipeline struct {
	fs       FileSystem
	gh       GitHubReleases
	onReload func(ctx context.Context) error
	validate ConfigValidator
	warn     func(msg string)
	lock     ConfigUpdateLock
}

func newConfigPipeline(fs FileSystem, gh GitHubReleases, opts ConfigPipelineOptions) *configPipeline {
	p := &configPipeline{fs: fs, gh: gh}
	if opts.OnReload != nil {
		p.onReload = opts.OnReload
	}
	if opts.Validator != nil {
		p.validate = opts.Validator
	}
	if opts.Warn != nil {
		p.warn = opts.Warn
	} else {
		p.warn = func(msg string) { fmt.Fprintln(os.Stderr, "warning:", msg) }
	}
	if opts.Lock != nil {
		p.lock = opts.Lock
	} else {
		p.lock = noopConfigUpdateLock{}
	}
	p.migrateLegacyTemplate()
	return p
}

// migrateLegacyTemplate renames the old config-template.yaml to the override
// file on first use, so existing setups carry over without manual steps. It
// runs once: after a successful rename the legacy path no longer exists.
func (p *configPipeline) migrateLegacyTemplate() {
	if !p.fs.FileExists(legacyTemplatePath) || p.fs.FileExists(OverrideFilePath) {
		return
	}
	if err := p.fs.Rename(legacyTemplatePath, OverrideFilePath); err != nil {
		p.warn(fmt.Sprintf("failed to migrate %s: %v", legacyTemplatePath, err))
		return
	}
	p.warn("migrated config-template.yaml to override.yaml. The old file name is no longer recognized.")
}

func renderConfig(template, subscription, routingRules string) (string, error) {
	result := strings.ReplaceAll(template, "{{subscription}}", subscription)
	result = strings.ReplaceAll(result, "{{routing_rules}}", routingRules)
	return result, nil
}

type fileSnapshot struct {
	data   []byte
	exists bool
}

func (p *configPipeline) snapshotFile(path string) (fileSnapshot, error) {
	data, err := p.fs.ReadFile(path)
	if err == nil {
		return fileSnapshot{data: data, exists: true}, nil
	}
	if os.IsNotExist(err) {
		return fileSnapshot{}, nil
	}
	return fileSnapshot{}, err
}

func (p *configPipeline) restoreFile(path string, snapshot fileSnapshot) error {
	if snapshot.exists {
		return p.fs.WriteFile(path, snapshot.data, filePermUserRW)
	}
	return p.fs.Remove(path)
}

func (p *configPipeline) restoreSubscriptionState(snapshots map[string]fileSnapshot) error {
	var restoreErrs []error
	for _, path := range []string{subscriptionDataFile, subscriptionURLFile, subscriptionSourceFile} {
		if err := p.restoreFile(path, snapshots[path]); err != nil {
			restoreErrs = append(restoreErrs, fmt.Errorf("restore %s: %w", path, err))
		}
	}
	return errors.Join(restoreErrs...)
}

func (p *configPipeline) SetSubscriptionSource(ctx context.Context, source string) error {
	if err := p.fs.MkdirAll(stateDir, filePermUserRWX); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return fmt.Errorf("subscription source cannot be empty")
	}

	snapshots := make(map[string]fileSnapshot, 3)
	for _, path := range []string{subscriptionDataFile, subscriptionURLFile, subscriptionSourceFile} {
		snapshot, err := p.snapshotFile(path)
		if err != nil {
			return fmt.Errorf("reading subscription state: %w", err)
		}
		snapshots[path] = snapshot
	}

	rollback := func(err error) error {
		if restoreErr := p.restoreSubscriptionState(snapshots); restoreErr != nil {
			return errors.Join(err, restoreErr)
		}
		return err
	}

	if looksLikeURL(trimmed) {
		if err := p.fs.WriteFile(subscriptionURLFile, []byte(trimmed), filePermUserRW); err != nil {
			return rollback(err)
		}
		if err := p.fs.Remove(subscriptionDataFile); err != nil {
			return rollback(fmt.Errorf("removing local subscription data: %w", err))
		}
		if err := p.fs.WriteFile(subscriptionSourceFile, []byte(remoteSubscriptionSource+"\n"), filePermUserRW); err != nil {
			return rollback(fmt.Errorf("recording subscription source: %w", err))
		}
		return nil
	}

	if err := p.fs.WriteFile(subscriptionDataFile, []byte(source), filePermUserRW); err != nil {
		return rollback(err)
	}
	if err := p.fs.Remove(subscriptionURLFile); err != nil {
		return rollback(fmt.Errorf("removing remote subscription URL: %w", err))
	}
	if err := p.fs.WriteFile(subscriptionSourceFile, []byte(localSubscriptionSource+"\n"), filePermUserRW); err != nil {
		return rollback(fmt.Errorf("recording subscription source: %w", err))
	}
	return nil
}

const (
	remoteSubscriptionSource = "remote"
	localSubscriptionSource  = "local"
)

func (p *configPipeline) filePresent(path string) bool {
	if p.fs.FileExists(path) {
		return true
	}
	_, err := p.fs.ReadFile(path)
	return err == nil
}

func (p *configPipeline) requireSourceValue(path, missingMessage, emptyMessage string) error {
	data, err := p.fs.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", missingMessage, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("%s", emptyMessage)
	}
	return nil
}

func (p *configPipeline) subscriptionSource() (string, error) {
	marker, err := p.fs.ReadFile(subscriptionSourceFile)
	if err == nil {
		source := strings.TrimSpace(string(marker))
		switch source {
		case remoteSubscriptionSource:
			if !p.filePresent(subscriptionURLFile) {
				return "", fmt.Errorf("remote subscription source is configured but its URL is missing")
			}
			if err := p.requireSourceValue(subscriptionURLFile, "reading remote subscription URL", "remote subscription URL is empty"); err != nil {
				return "", err
			}
		case localSubscriptionSource:
			if !p.filePresent(subscriptionDataFile) {
				return "", fmt.Errorf("local subscription source is configured but its data is missing")
			}
			if err := p.requireSourceValue(subscriptionDataFile, "reading local subscription data", "local subscription data is empty"); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("invalid subscription source %q", source)
		}
		return source, nil
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading subscription source: %w", err)
	}

	hasRemote := p.filePresent(subscriptionURLFile)
	hasLocal := p.filePresent(subscriptionDataFile)
	switch {
	case hasRemote && hasLocal:
		return "", fmt.Errorf("conflicting legacy subscription sources; choose remote or local explicitly")
	case hasRemote:
		if err := p.requireSourceValue(subscriptionURLFile, "reading legacy remote subscription URL", "legacy remote subscription URL is empty"); err != nil {
			return "", err
		}
		if err := p.fs.WriteFile(subscriptionSourceFile, []byte(remoteSubscriptionSource+"\n"), filePermUserRW); err != nil {
			return "", fmt.Errorf("migrating remote subscription source: %w", err)
		}
		return remoteSubscriptionSource, nil
	case hasLocal:
		if err := p.requireSourceValue(subscriptionDataFile, "reading legacy local subscription data", "legacy local subscription data is empty"); err != nil {
			return "", err
		}
		if err := p.fs.WriteFile(subscriptionSourceFile, []byte(localSubscriptionSource+"\n"), filePermUserRW); err != nil {
			return "", fmt.Errorf("migrating local subscription source: %w", err)
		}
		return localSubscriptionSource, nil
	default:
		return "", nil
	}
}

func (p *configPipeline) Preview(ctx context.Context) (string, error) {
	if _, err := p.subscriptionSource(); err != nil {
		return "", err
	}
	subData, err := p.fs.ReadFile(subscriptionDataFile)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	tmpl, tmplErr := p.fs.ReadFile(OverrideFilePath)
	if tmplErr != nil && !os.IsNotExist(tmplErr) {
		return "", tmplErr
	}

	tmplStr := ""
	if tmplErr == nil {
		tmplStr = string(tmpl)
	}

	subStr := ""
	if err == nil {
		subStr = string(subData)
	}

	if strings.Contains(tmplStr, "{{subscription}}") || strings.Contains(tmplStr, "{{routing_rules}}") {
		p.warn("config-template.yaml uses old placeholder format. Please migrate to YAML overlay format.")
	}

	return mergeConfig(subStr, tmplStr)
}

func configContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

func (p *configPipeline) writeConfigApplyStatus(status ConfigApplyStatus) error {
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("encoding config apply status: %w", err)
	}
	if err := p.fs.MkdirAll(stateDir, filePermUserRWX); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	tmpPath := configApplyStatusFile + ".tmp"
	if err := p.fs.WriteFile(tmpPath, data, filePermUserRW); err != nil {
		return fmt.Errorf("writing config apply status: %w", err)
	}
	if err := p.fs.Rename(tmpPath, configApplyStatusFile); err != nil {
		p.fs.Remove(tmpPath)
		return fmt.Errorf("committing config apply status: %w", err)
	}
	return nil
}

func (p *configPipeline) recordConfigApply(state ConfigApplyState, preview string, applyErr error) error {
	status := ConfigApplyStatus{
		State:       state,
		AttemptedAt: time.Now().UTC(),
		ConfigHash:  configContentHash(preview),
	}
	if applyErr != nil {
		status.ErrorSummary = applyErr.Error()
	}
	return p.writeConfigApplyStatus(status)
}

func (p *configPipeline) Apply(ctx context.Context) (applyErr error) {
	release, err := p.lock.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	preview := ""
	statusRecorded := false
	defer func() {
		if applyErr != nil && !statusRecorded {
			statusErr := p.recordConfigApply(ConfigValidationFailed, preview, applyErr)
			statusRecorded = true
			if statusErr != nil {
				applyErr = errors.Join(applyErr, statusErr)
			}
		}
	}()

	source, err := p.subscriptionSource()
	if err != nil {
		return err
	}
	if source == "" {
		return fmt.Errorf("subscription source is not configured")
	}
	if source == remoteSubscriptionSource {
		data, err := p.fs.ReadFile(subscriptionURLFile)
		if err != nil {
			return fmt.Errorf("reading subscription URL: %w", err)
		}
		url := strings.TrimSpace(string(data))
		if url != "" {
			tmpPath := subscriptionDataFile + ".tmp"
			if err := p.gh.Download(ctx, url, tmpPath); err != nil {
				return fmt.Errorf("fetching subscription: %w", err)
			}
			fetched, err := p.fs.ReadFile(tmpPath)
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(fetched)) == 0 {
				p.fs.Remove(tmpPath)
				return fmt.Errorf("fetched subscription content is empty")
			}
			if err := p.fs.WriteFile(subscriptionDataFile, fetched, filePermUserRW); err != nil {
				return errors.Join(fmt.Errorf("writing subscription data: %w", err), p.fs.Remove(tmpPath))
			}
			if err := p.fs.Remove(tmpPath); err != nil {
				return fmt.Errorf("removing downloaded subscription: %w", err)
			}
		}
	}

	preview, err = p.Preview(ctx)
	if err != nil {
		return err
	}

	if strings.TrimSpace(preview) == "" {
		return fmt.Errorf("generated config is empty")
	}

	stageDir := filepath.Join(configDir, fmt.Sprintf(".mihomo-config-staging-%d", time.Now().UnixNano()))
	stagePath := filepath.Join(stageDir, "config.yaml")
	cleanupStage := func(primary error) error {
		if cleanupErr := p.fs.Remove(stageDir); cleanupErr != nil {
			return errors.Join(primary, fmt.Errorf("cleanup staged config: %w", cleanupErr))
		}
		return primary
	}
	if err := p.fs.MkdirAll(stageDir, filePermUserRWX); err != nil {
		return fmt.Errorf("creating config staging directory: %w", err)
	}
	if err := p.fs.WriteFile(stagePath, []byte(preview), filePermUserRW); err != nil {
		return cleanupStage(fmt.Errorf("writing staged config: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return cleanupStage(err)
	}

	if p.validate != nil {
		if err := p.validate.Validate(ctx, stagePath); err != nil {
			failure := cleanupStage(err)
			statusErr := p.recordConfigApply(ConfigValidationFailed, preview, err)
			statusRecorded = true
			if statusErr != nil {
				failure = errors.Join(failure, statusErr)
			}
			return failure
		}
	}
	if err := ctx.Err(); err != nil {
		return cleanupStage(err)
	}

	if p.fs.FileExists(configYAML) {
		backupPath := configYAML + ".bak." + timestamp()
		existing, err := p.fs.ReadFile(configYAML)
		if err != nil {
			return cleanupStage(err)
		}
		if err := p.fs.WriteFile(backupPath, existing, filePermUserRW); err != nil {
			return cleanupStage(err)
		}
	}
	if err := p.fs.Rename(stagePath, configYAML); err != nil {
		return cleanupStage(fmt.Errorf("committing generated config: %w", err))
	}
	commitCleanupErr := p.fs.Remove(stageDir)

	if p.onReload != nil {
		if err := p.onReload(ctx); err != nil {
			failure := err
			if commitCleanupErr != nil {
				failure = errors.Join(failure, fmt.Errorf("cleanup staged config: %w", commitCleanupErr))
			}
			statusErr := p.recordConfigApply(ConfigPendingReload, preview, failure)
			statusRecorded = true
			if statusErr != nil {
				failure = errors.Join(failure, statusErr)
			}
			return failure
		}
	}

	if commitCleanupErr != nil {
		statusErr := p.recordConfigApply(ConfigApplied, preview, commitCleanupErr)
		statusRecorded = true
		if statusErr != nil {
			return errors.Join(commitCleanupErr, statusErr)
		}
		return commitCleanupErr
	}
	statusErr := p.recordConfigApply(ConfigApplied, preview, nil)
	statusRecorded = true
	return statusErr
}

func (p *configPipeline) LastConfigApply(ctx context.Context) (ConfigApplyStatus, error) {
	data, err := p.fs.ReadFile(configApplyStatusFile)
	if os.IsNotExist(err) {
		return ConfigApplyStatus{State: ConfigUnknown}, nil
	}
	if err != nil {
		return ConfigApplyStatus{State: ConfigUnknown, ErrorSummary: err.Error()}, err
	}
	var status ConfigApplyStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return ConfigApplyStatus{State: ConfigUnknown, ErrorSummary: fmt.Sprintf("invalid status file: %v", err)}, nil
	}
	switch status.State {
	case ConfigApplied, ConfigPendingReload, ConfigValidationFailed:
		return status, nil
	default:
		return ConfigApplyStatus{State: ConfigUnknown, ErrorSummary: "unknown config apply state"}, nil
	}
}

// Validate runs the configured ConfigValidator against the generated config.
// A nil validator means validation is a no-op (validation not configured).
func (p *configPipeline) Validate(ctx context.Context) error {
	if p.validate == nil {
		return nil
	}
	return p.validate.Validate(ctx, configYAML)
}
