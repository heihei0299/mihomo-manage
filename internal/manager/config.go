package manager

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func renderConfig(template, subscription, routingRules string) (string, error) {
	result := strings.ReplaceAll(template, "{{subscription}}", subscription)
	result = strings.ReplaceAll(result, "{{routing_rules}}", routingRules)
	return result, nil
}

func (m *manager) SetSubscriptionSource(ctx context.Context, url string) error {
	if looksLikeURL(url) {
		return m.sys.WriteFile(subscriptionURLFile, []byte(url), filePermUserRW)
	}
	return m.sys.WriteFile(subscriptionDataFile, []byte(url), filePermUserRW)
}

func (m *manager) SetRoutingRules(ctx context.Context, rules string) error {
	return m.sys.WriteFile(RoutingRulesPath, []byte(rules), filePermUserRW)
}

func (m *manager) PreviewConfig(ctx context.Context) (string, error) {
	tmpl, err := m.sys.ReadFile(ConfigTemplatePath)
	if err != nil {
		return "", err
	}

	subData, err := m.sys.ReadFile(subscriptionDataFile)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	rulesData, err := m.sys.ReadFile(RoutingRulesPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	return renderConfig(string(tmpl), string(subData), string(rulesData))
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
			m.sys.WriteFile(subscriptionDataFile, fetched, filePermUserRW)
			m.sys.Remove(tmpPath)
		}
	}

	preview, err := m.PreviewConfig(ctx)
	if err != nil {
		return err
	}

	if m.sys.FileExists(configYAML) {
		backupPath := configYAML + ".bak." + timestamp()
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

	m.svcMgr.Reload(serviceName)

	return nil
}
