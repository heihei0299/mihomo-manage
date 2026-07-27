package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anomalyco/mihomo-manager/internal/domain"
	"github.com/anomalyco/mihomo-manager/internal/infra"
)

// validator implements domain.ConfigValidator using infra.CommandRunner.
type validator struct {
	cmd infra.CommandRunner
}

// NewValidator returns a domain.ConfigValidator that validates mihomo config via the binary.
func NewValidator(cmd infra.CommandRunner) domain.ConfigValidator {
	return &validator{cmd: cmd}
}

func (v *validator) Validate(ctx context.Context, configPath string) error {
	out, err := v.cmd.RunCommand(BinaryPath, "-t", "-d", ConfigDir)
	if err != nil {
		return fmt.Errorf("config validation failed:\n%s", out)
	}
	return nil
}

// ConfigPipelineOptions configures the config pipeline behaviour.
type ConfigPipelineOptions struct {
	OnReload  func(ctx context.Context) error
	Validator domain.ConfigValidator
}

// pipeline implements domain.ConfigPipeline.
type pipeline struct {
	fs       infra.FileSystem
	gh       infra.ReleaseRepo
	onReload func(ctx context.Context) error
	validate domain.ConfigValidator
}

// NewPipeline returns a domain.ConfigPipeline that generates and applies mihomo config.
func NewPipeline(fs infra.FileSystem, gh infra.ReleaseRepo, opts ConfigPipelineOptions) domain.ConfigPipeline {
	p := &pipeline{fs: fs, gh: gh}
	if opts.OnReload != nil {
		p.onReload = opts.OnReload
	}
	if opts.Validator != nil {
		p.validate = opts.Validator
	}
	return p
}

func renderConfig(template, subscription, routingRules string) (string, error) {
	result := strings.ReplaceAll(template, "{{subscription}}", subscription)
	result = strings.ReplaceAll(result, "{{routing_rules}}", routingRules)
	return result, nil
}

func hasTopLevelKeys(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if strings.Contains(trimmed, ":") {
			return true
		}
	}
	return false
}

func (p *pipeline) SetSubscriptionSource(ctx context.Context, source string) error {
	if err := p.fs.MkdirAll(StateDir, FilePermUserRWX); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	if domain.LooksLikeURL(source) {
		return p.fs.WriteFile(SubscriptionURLFile, []byte(source), FilePermUserRW)
	}
	return p.fs.WriteFile(SubscriptionDataFile, []byte(source), FilePermUserRW)
}

func (p *pipeline) SetRoutingRules(ctx context.Context, rules string) error {
	return p.fs.WriteFile(RoutingRulesPath, []byte(rules), FilePermUserRW)
}

func (p *pipeline) Preview(ctx context.Context) (string, error) {
	subData, err := p.fs.ReadFile(SubscriptionDataFile)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil && hasTopLevelKeys(subData) {
		return string(subData), nil
	}

	tmpl, err := p.fs.ReadFile(ConfigTemplatePath)
	if err != nil {
		return "", err
	}

	var subStr string
	if err == nil {
		subStr = string(subData)
	}

	rulesData, err := p.fs.ReadFile(RoutingRulesPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	return renderConfig(string(tmpl), subStr, string(rulesData))
}

func (p *pipeline) Apply(ctx context.Context) error {
	data, err := p.fs.ReadFile(SubscriptionURLFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading subscription URL: %w", err)
		}
	} else {
		url := strings.TrimSpace(string(data))
		if url != "" {
			tmpPath := SubscriptionDataFile + ".tmp"
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
			p.fs.WriteFile(SubscriptionDataFile, fetched, FilePermUserRW)
			p.fs.Remove(tmpPath)
		}
	}

	if !p.fs.FileExists(ConfigTemplatePath) {
		if err := p.fs.MkdirAll(ConfigDir, FilePermUserRWX); err != nil {
			return err
		}
		if err := p.fs.WriteFile(ConfigTemplatePath, DefaultTemplate, FilePermUserRW); err != nil {
			return err
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
	if p.fs.FileExists(ConfigYAML) {
		backupPath = ConfigYAML + ".bak." + domain.Timestamp()
		existing, err := p.fs.ReadFile(ConfigYAML)
		if err != nil {
			return err
		}
		if err := p.fs.WriteFile(backupPath, existing, FilePermUserRW); err != nil {
			return err
		}
	}

	if err := p.fs.WriteFile(ConfigYAML, []byte(preview), FilePermUserRW); err != nil {
		return err
	}

	if p.validate != nil {
		if err := p.validate.Validate(ctx, ConfigYAML); err != nil {
			if backupPath != "" {
				if bak, rErr := p.fs.ReadFile(backupPath); rErr == nil {
					p.fs.WriteFile(ConfigYAML, bak, FilePermUserRW)
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
