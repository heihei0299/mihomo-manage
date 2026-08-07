package manager

import (
	"bytes"
	"context"
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
	SetRoutingRules(ctx context.Context, rules string) error
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
	return p
}

func renderConfig(template, subscription, routingRules string) (string, error) {
	result := strings.ReplaceAll(template, "{{subscription}}", subscription)
	result = strings.ReplaceAll(result, "{{routing_rules}}", routingRules)
	return result, nil
}

func (p *configPipeline) SetSubscriptionSource(ctx context.Context, source string) error {
	if err := p.fs.MkdirAll(stateDir, filePermUserRWX); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	if looksLikeURL(source) {
		return p.fs.WriteFile(subscriptionURLFile, []byte(source), filePermUserRW)
	}
	return p.fs.WriteFile(subscriptionDataFile, []byte(source), filePermUserRW)
}

func (p *configPipeline) SetRoutingRules(ctx context.Context, rules string) error {
	return p.fs.WriteFile(RoutingRulesPath, []byte(rules), filePermUserRW)
}

func (p *configPipeline) Preview(ctx context.Context) (string, error) {
	subData, err := p.fs.ReadFile(subscriptionDataFile)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	tmpl, tmplErr := p.fs.ReadFile(ConfigTemplatePath)
	if tmplErr != nil {
		return "", tmplErr
	}

	tmplStr := string(tmpl)
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
	data, err := p.fs.ReadFile(subscriptionURLFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading subscription URL: %w", err)
		}
	} else {
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

	if !p.fs.FileExists(ConfigTemplatePath) {
		if err := p.fs.MkdirAll(configDir, filePermUserRWX); err != nil {
			return err
		}
		if err := p.fs.WriteFile(ConfigTemplatePath, defaultTemplate, filePermUserRW); err != nil {
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
