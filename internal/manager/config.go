package manager

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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

func (m *manager) SetSubscriptionSource(ctx context.Context, url string) error {
	if err := m.sys.MkdirAll(stateDir, filePermUserRWX); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	if looksLikeURL(url) {
		return m.sys.WriteFile(subscriptionURLFile, []byte(url), filePermUserRW)
	}
	return m.sys.WriteFile(subscriptionDataFile, []byte(url), filePermUserRW)
}

func (m *manager) SetRoutingRules(ctx context.Context, rules string) error {
	return m.sys.WriteFile(RoutingRulesPath, []byte(rules), filePermUserRW)
}

func (m *manager) PreviewConfig(ctx context.Context) (string, error) {
	subData, err := m.sys.ReadFile(subscriptionDataFile)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil && hasTopLevelKeys(subData) {
		return string(subData), nil
	}

	tmpl, err := m.sys.ReadFile(ConfigTemplatePath)
	if err != nil {
		return "", err
	}

	var subStr string
	if err == nil {
		subStr = string(subData)
	}

	rulesData, err := m.sys.ReadFile(RoutingRulesPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	return renderConfig(string(tmpl), subStr, string(rulesData))
}

func (m *manager) UpdateConfig(ctx context.Context) error {
	data, err := m.sys.ReadFile(subscriptionURLFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading subscription URL: %w", err)
		}
	} else {
		url := strings.TrimSpace(string(data))
		if url != "" {
			tmpPath := subscriptionDataFile + ".tmp"
			if err := m.sys.Download(ctx, url, tmpPath); err != nil {
				return fmt.Errorf("fetching subscription: %w", err)
			}
			fetched, err := m.sys.ReadFile(tmpPath)
			if err != nil {
				return err
			}
			if len(bytes.TrimSpace(fetched)) == 0 {
				m.sys.Remove(tmpPath)
				return fmt.Errorf("fetched subscription content is empty")
			}
			m.sys.WriteFile(subscriptionDataFile, fetched, filePermUserRW)
			m.sys.Remove(tmpPath)
		}
	}

	if !m.sys.FileExists(ConfigTemplatePath) {
		if err := m.sys.MkdirAll(configDir, filePermUserRWX); err != nil {
			return err
		}
		if err := m.sys.WriteFile(ConfigTemplatePath, defaultTemplate, filePermUserRW); err != nil {
			return err
		}
	}

	preview, err := m.PreviewConfig(ctx)
	if err != nil {
		return err
	}

	if strings.TrimSpace(preview) == "" {
		return fmt.Errorf("generated config is empty")
	}

	var backupPath string
	if m.sys.FileExists(configYAML) {
		backupPath = configYAML + ".bak." + timestamp()
		existing, err := m.sys.ReadFile(configYAML)
		if err != nil {
			return err
		}
		if err := m.sys.WriteFile(backupPath, existing, filePermUserRW); err != nil {
			return err
		}
	}

	if err := m.sys.WriteFile(configYAML, []byte(preview), filePermUserRW); err != nil {
		return err
	}

	if m.sys.FileExists(binaryPath) {
		cmd := exec.Command(binaryPath, "-t", "-d", configDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if backupPath != "" {
				if bak, rErr := m.sys.ReadFile(backupPath); rErr == nil {
					m.sys.WriteFile(configYAML, bak, filePermUserRW)
				}
			}
			return fmt.Errorf("config validation failed:\n%s", string(out))
		}
	}

	m.svcMgr.Reload(serviceName)

	return nil
}
