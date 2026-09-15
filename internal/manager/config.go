package manager

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type ConfigValidator interface {
	Validate(ctx context.Context, configPath string) error
}

type configValidator struct{}

func (v *configValidator) Validate(ctx context.Context, configPath string) error {
	cmd := exec.Command(binaryPath, "-t", "-d", configDir)
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
}

type ConfigPipelineOptions struct {
	OnReload  func(ctx context.Context) error
	Validator ConfigValidator
	Warn      func(msg string)
}

type configPipeline struct {
	fs       FileSystem
	gh       GitHubReleases
	onReload func(ctx context.Context) error
	validate ConfigValidator
	warn     func(msg string)
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

func (p *configPipeline) Apply(ctx context.Context) error {
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
			p.fs.WriteFile(subscriptionDataFile, fetched, filePermUserRW)
			p.fs.Remove(tmpPath)
		}
	}

	preview, err := p.Preview(ctx)
	if err != nil {
		return err
	}

	if strings.TrimSpace(preview) == "" {
		return fmt.Errorf("generated config is empty")
	}

	var backupPath string
	if p.fs.FileExists(configYAML) {
		backupPath = configYAML + ".bak." + timestamp()
		existing, err := p.fs.ReadFile(configYAML)
		if err != nil {
			return err
		}
		if err := p.fs.WriteFile(backupPath, existing, filePermUserRW); err != nil {
			return err
		}
	}

	if err := p.fs.WriteFile(configYAML, []byte(preview), filePermUserRW); err != nil {
		return err
	}

	if p.validate != nil {
		if err := p.validate.Validate(ctx, configYAML); err != nil {
			if backupPath != "" {
				if bak, rErr := p.fs.ReadFile(backupPath); rErr == nil {
					p.fs.WriteFile(configYAML, bak, filePermUserRW)
				}
			}
			return err
		}
	}

	if p.onReload != nil {
		p.onReload(ctx)
	}

	return nil
}

// Validate runs the configured ConfigValidator against the generated config.
// A nil validator means validation is a no-op (validation not configured).
func (p *configPipeline) Validate(ctx context.Context) error {
	if p.validate == nil {
		return nil
	}
	return p.validate.Validate(ctx, configYAML)
}
