package manager

import (
	"context"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func previewWithFiles(t *testing.T, subscription, override string) (string, error) {
	t.Helper()
	fs := &fakeFileSystem{written: map[string][]byte{
		subscriptionSourceFile: []byte("local\n"),
		subscriptionDataFile:   []byte(subscription),
		OverrideFilePath:       []byte(override),
	}}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
	return m.PreviewConfig(context.Background())
}

func mustPreviewYAML(t *testing.T, subscription, override string) map[string]any {
	t.Helper()
	preview, err := previewWithFiles(t, subscription, override)
	if err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}
	if strings.Contains(preview, "!delete") || strings.Contains(preview, "!replace") {
		t.Fatalf("directive leaked into preview:\n%s", preview)
	}
	var got map[string]any
	if err := yaml.Unmarshal([]byte(preview), &got); err != nil {
		t.Fatalf("parsing preview: %v\n%s", err, preview)
	}
	return got
}

func previewError(t *testing.T, subscription, override string) string {
	t.Helper()
	_, err := previewWithFiles(t, subscription, override)
	if err == nil {
		t.Fatal("expected PreviewConfig to fail")
	}
	return err.Error()
}

func proxyNames(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected an array, got %#v", value)
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected a mapping entry, got %#v", item)
		}
		names = append(names, entry["name"].(string))
	}
	return names
}

func ruleTexts(t *testing.T, value any) []string {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected an array, got %#v", value)
	}
	rules := make([]string, 0, len(items))
	for _, item := range items {
		rules = append(rules, item.(string))
	}
	return rules
}

func TestPreviewNestedDeleteAnyLevel(t *testing.T) {
	got := mustPreviewYAML(t, `port: 7890
dns:
  enable: true
  fallback:
    - 8.8.8.8
  nameserver-policy:
    example.com: 1.1.1.1
    example.org: 8.8.8.8
`, `dns:
  fallback: !delete null
  nameserver-policy:
    example.org: !delete null
`)

	dns := got["dns"].(map[string]any)
	if _, exists := dns["fallback"]; exists {
		t.Fatalf("nested !delete did not remove dns.fallback: %#v", dns)
	}
	policy := dns["nameserver-policy"].(map[string]any)
	if _, exists := policy["example.org"]; exists {
		t.Fatalf("nested !delete did not remove nameserver-policy.example.org: %#v", policy)
	}
	if dns["enable"] != true || policy["example.com"] != "1.1.1.1" {
		t.Fatalf("unrelated fields changed: %#v", dns)
	}
}

func TestPreviewNestedReplaceAnyLevel(t *testing.T) {
	got := mustPreviewYAML(t, `dns:
  enable: true
  ipv6:
    enable: true
    stack: system
tun:
  enable: false
`, `dns:
  ipv6: !replace
    enable: false
`)
	dns := got["dns"].(map[string]any)
	ipv6 := dns["ipv6"].(map[string]any)
	if ipv6["enable"] != false {
		t.Fatalf("!replace did not set enable: %#v", ipv6)
	}
	if _, exists := ipv6["stack"]; exists {
		t.Fatalf("!replace inherited a subscription field: %#v", ipv6)
	}
	if dns["enable"] != true {
		t.Fatalf("sibling field should be inherited: %#v", dns)
	}
	if tun := got["tun"].(map[string]any); tun["enable"] != false {
		t.Fatalf("unrelated field changed: %#v", got["tun"])
	}
}

func TestPreviewIdentityArraysReplaceByNameInPlace(t *testing.T) {
	got := mustPreviewYAML(t, `proxies:
  - name: 香港01
    type: ss
    server: hk.example.com
    port: 443
    udp: true
  - name: 日本01
    type: ss
    server: jp.example.com
    port: 443
proxy-groups:
  - name: Proxy
    type: select
    proxies:
      - 香港01
  - name: Auto
    type: url-test
    proxies:
      - 日本01
`, `proxies:
  - name: 香港01
    type: ss
    server: hk.local
    port: 8443
proxy-groups:
  - name: Proxy
    type: select
    proxies:
      - 香港01
      - 日本01
`)

	names := proxyNames(t, got["proxies"])
	if len(names) != 2 || names[0] != "香港01" || names[1] != "日本01" {
		t.Fatalf("same-name replacement must keep position, got %v", names)
	}
	first := got["proxies"].([]any)[0].(map[string]any)
	if first["server"] != "hk.local" || first["port"] != 8443 {
		t.Fatalf("same-name proxy was not replaced whole: %#v", first)
	}
	if _, inherited := first["udp"]; inherited {
		t.Fatalf("same-name proxy inherited a subscription field: %#v", first)
	}
	second := got["proxies"].([]any)[1].(map[string]any)
	if second["server"] != "jp.example.com" {
		t.Fatalf("untouched proxy changed: %#v", second)
	}

	groupNames := proxyNames(t, got["proxy-groups"])
	if len(groupNames) != 2 || groupNames[0] != "Proxy" || groupNames[1] != "Auto" {
		t.Fatalf("same-name group replacement must keep position, got %v", groupNames)
	}
	proxyGroup := got["proxy-groups"].([]any)[0].(map[string]any)
	if len(proxyGroup["proxies"].([]any)) != 2 {
		t.Fatalf("same-name group should be replaced whole, not deep-merged: %#v", proxyGroup)
	}
}

func TestPreviewIdentityArraysAppendNewNames(t *testing.T) {
	got := mustPreviewYAML(t, `proxies:
  - name: node1
    type: ss
proxy-providers:
  remote:
    type: http
    url: "https://example.com/sub"
rule-providers:
  ads:
    type: http
    behavior: domain
    url: "https://example.com/ads"
`, `proxies:
  - name: node2
    type: ss
proxy-providers:
  local:
    type: file
    path: ./providers/local.yaml
rule-providers:
  tracking:
    type: http
    behavior: domain
    url: "https://example.com/tracking"
`)

	if names := proxyNames(t, got["proxies"]); len(names) != 2 || names[0] != "node1" || names[1] != "node2" {
		t.Fatalf("new proxy should append, got %v", names)
	}
	providers := got["proxy-providers"].(map[string]any)
	if providers["remote"] == nil || providers["local"] == nil {
		t.Fatalf("providers should contain both entries: %#v", providers)
	}
	rules := got["rule-providers"].(map[string]any)
	if rules["ads"] == nil || rules["tracking"] == nil {
		t.Fatalf("rule providers should contain both entries: %#v", rules)
	}
}

func TestPreviewProvidersReplaceWholeValueByName(t *testing.T) {
	got := mustPreviewYAML(t, `proxy-providers:
  remote:
    type: http
    url: "https://example.com/old"
    interval: 300
rule-providers:
  ads:
    type: http
    behavior: domain
    url: "https://example.com/old"
    interval: 300
`, `proxy-providers:
  remote:
    type: http
    url: "https://example.com/new"
rule-providers:
  ads:
    type: http
    behavior: domain
    url: "https://example.com/new"
`)
	provider := got["proxy-providers"].(map[string]any)["remote"].(map[string]any)
	if provider["url"] != "https://example.com/new" {
		t.Fatalf("same-name provider not replaced: %#v", provider)
	}
	if _, exists := provider["interval"]; exists {
		t.Fatalf("same-name provider inherited subscription fields: %#v", provider)
	}
	ruleProvider := got["rule-providers"].(map[string]any)["ads"].(map[string]any)
	if _, exists := ruleProvider["interval"]; exists {
		t.Fatalf("same-name rule provider inherited subscription fields: %#v", ruleProvider)
	}
}

func TestPreviewDeletesIdentityItemsAndRules(t *testing.T) {
	got := mustPreviewYAML(t, `proxies:
  - name: node1
    type: ss
  - name: node2
    type: ss
  - name: node3
    type: ss
proxy-providers:
  remote:
    type: http
    url: "https://example.com/sub"
rules:
  - DOMAIN-SUFFIX,sub.example.com,Proxy
  - GEOIP,CN,DIRECT
  - MATCH,DIRECT
`, `proxies:
  - !delete node2
proxy-providers:
  remote: !delete null
rules:
  - DOMAIN-SUFFIX,local.example.com,DIRECT
  - !delete DOMAIN-SUFFIX,sub.example.com,Proxy
  - !delete GEOIP,CN,DIRECT
`)

	if names := proxyNames(t, got["proxies"]); len(names) != 2 || names[0] != "node1" || names[1] != "node3" {
		t.Fatalf("!delete should remove the named proxy only, got %v", names)
	}
	if _, exists := got["proxy-providers"]; exists {
		providers := got["proxy-providers"].(map[string]any)
		if _, hasRemote := providers["remote"]; hasRemote {
			t.Fatalf("provider deletion did not apply: %#v", providers)
		}
	}
	rules := ruleTexts(t, got["rules"])
	want := []string{"DOMAIN-SUFFIX,local.example.com,DIRECT", "MATCH,DIRECT"}
	if len(rules) != len(want) || rules[0] != want[0] || rules[1] != want[1] {
		t.Fatalf("rules = %v, want %v", rules, want)
	}
}

func TestPreviewUserRulesInsertBeforeFirstMatch(t *testing.T) {
	got := mustPreviewYAML(t, `rules:
  - DOMAIN-SUFFIX,sub.example.com,Proxy
  - MATCH,DIRECT
`, `rules:
  - DOMAIN-SUFFIX,local.example.com,DIRECT
`)
	rules := ruleTexts(t, got["rules"])
	want := []string{"DOMAIN-SUFFIX,sub.example.com,Proxy", "DOMAIN-SUFFIX,local.example.com,DIRECT", "MATCH,DIRECT"}
	if len(rules) != len(want) {
		t.Fatalf("rules = %v, want %v", rules, want)
	}
	for i := range want {
		if rules[i] != want[i] {
			t.Fatalf("rules = %v, want %v", rules, want)
		}
	}
}

func TestPreviewUserRulesAppendWhenSubscriptionHasNoMatch(t *testing.T) {
	got := mustPreviewYAML(t, `rules:
  - GEOIP,CN,DIRECT
`, `rules:
  - MATCH,DIRECT
`)
	rules := ruleTexts(t, got["rules"])
	if len(rules) != 2 || rules[0] != "GEOIP,CN,DIRECT" || rules[1] != "MATCH,DIRECT" {
		t.Fatalf("rules = %v, want subscription rules then user rules", rules)
	}
}

func TestPreviewDirectiveErrorsCarryPaths(t *testing.T) {
	subscription := `dns:
  enable: true
proxies:
  - name: node1
    type: ss
proxy-providers:
  remote:
    type: http
    url: "https://example.com/sub"
rules:
  - MATCH,DIRECT
`
	for _, tc := range []struct {
		name, override, path string
	}{
		{"missing key", "dns:\n  fallback: !delete null\n", "dns.fallback"},
		{"missing name", "proxies:\n  - !delete node2\n", "proxies.node2"},
		{"missing provider", "proxy-providers:\n  missing: !delete null\n", "proxy-providers.missing"},
		{"provider delete and add", "proxy-providers:\n  remote: !delete null\n  remote:\n    type: http\n    url: new\n", "proxy-providers.remote"},
		{"missing rule", "rules:\n  - !delete GEOIP,CN,DIRECT\n", "rules.GEOIP,CN,DIRECT"},
		{"delete and add same name", "proxies:\n  - !delete node1\n  - name: node1\n    type: ss\n", "proxies.node1"},
		{"delete and add same rule", "rules:\n  - !delete MATCH,DIRECT\n  - MATCH,DIRECT\n", "rules.MATCH,DIRECT"},
		{"nested delete in replace", "dns: !replace\n  fallback: !delete null\n", "dns.fallback"},
		{"nested delete in replaced array", "listen:\n  - 0.0.0.0: !delete null\n", "listen[0].0.0.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := previewError(t, subscription, tc.override)
			if !strings.Contains(message, tc.path) {
				t.Fatalf("error %q does not contain path %q", message, tc.path)
			}
		})
	}
}

func TestApplyRejectsInvalidOverrideBeforeWritingConfig(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		subscriptionSourceFile: []byte("local\n"),
		subscriptionDataFile:   []byte("dns:\n  enable: true\n"),
		OverrideFilePath:       []byte("dns:\n  fallback: !delete null\n"),
		configYAML:             []byte("old: config\n"),
	}}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	err := m.ApplyConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dns.fallback") {
		t.Fatalf("ApplyConfig error = %v, want path dns.fallback", err)
	}
	if got := string(fs.written[configYAML]); got != "old: config\n" {
		t.Fatalf("config.yaml must stay untouched on merge error, got %q", got)
	}
}

func TestMergeConfigSyntaxTable(t *testing.T) {
	for _, tc := range []struct {
		name, base, overlay string
		check               func(*testing.T, map[string]any)
	}{
		{
			name:    "empty mapping base accepts overlay values",
			base:    "{}\n",
			overlay: "port: 7890\n",
			check: func(t *testing.T, got map[string]any) {
				if got["port"] != 7890 {
					t.Fatalf("port = %v", got["port"])
				}
			},
		},
		{
			name:    "empty mapping overlay keeps base values",
			base:    "port: 7890\n",
			overlay: "{}\n",
			check: func(t *testing.T, got map[string]any) {
				if got["port"] != 7890 {
					t.Fatalf("port = %v", got["port"])
				}
			},
		},
		{
			name:    "empty overlay keeps base text",
			base:    "port: 7890\n",
			overlay: "",
			check: func(t *testing.T, got map[string]any) {
				if got["port"] != 7890 {
					t.Fatalf("port = %v", got["port"])
				}
			},
		},
		{
			name:    "empty base applies and strips directives",
			base:    "",
			overlay: "proxies: !replace\n  - name: local\n    type: ss\n",
			check: func(t *testing.T, got map[string]any) {
				if names := proxyNames(t, got["proxies"]); len(names) != 1 || names[0] != "local" {
					t.Fatalf("proxies = %v", names)
				}
			},
		},
		{
			name:    "type mismatch replaces map with scalar",
			base:    "dns:\n  enable: true\n",
			overlay: "dns: false\n",
			check: func(t *testing.T, got map[string]any) {
				if got["dns"] != false {
					t.Fatalf("dns = %#v", got["dns"])
				}
			},
		},
		{
			name:    "type mismatch replaces scalar with map",
			base:    "dns: false\n",
			overlay: "dns:\n  enable: true\n",
			check: func(t *testing.T, got map[string]any) {
				dns, ok := got["dns"].(map[string]any)
				if !ok || dns["enable"] != true {
					t.Fatalf("dns = %#v", got["dns"])
				}
			},
		},
		{
			name:    "non identity array is replaced",
			base:    "listen:\n  - 0.0.0.0:9090\n",
			overlay: "listen:\n  - 0.0.0.0:8080\n",
			check: func(t *testing.T, got map[string]any) {
				listen := got["listen"].([]any)
				if len(listen) != 1 || listen[0] != "0.0.0.0:8080" {
					t.Fatalf("listen = %v", listen)
				}
			},
		},
		{
			name:    "identity array key is case sensitive",
			base:    "proxies:\n  - name: node1\n    type: ss\n",
			overlay: "Proxies:\n  - name: node2\n    type: ss\n",
			check: func(t *testing.T, got map[string]any) {
				if names := proxyNames(t, got["proxies"]); len(names) != 1 || names[0] != "node1" {
					t.Fatalf("proxies = %v", names)
				}
				if names := proxyNames(t, got["Proxies"]); len(names) != 1 || names[0] != "node2" {
					t.Fatalf("Proxies = %v", names)
				}
			},
		},
		{
			name:    "duplicate overlay names keep the last value",
			base:    "proxies:\n  - name: node1\n    type: ss\n",
			overlay: "proxies:\n  - name: node1\n    type: ss\n    port: 1\n  - name: node1\n    type: ss\n    port: 2\n",
			check: func(t *testing.T, got map[string]any) {
				items := got["proxies"].([]any)
				if len(items) != 1 {
					t.Fatalf("duplicate overlay names must not duplicate output: %v", items)
				}
				if items[0].(map[string]any)["port"] != 2 {
					t.Fatalf("last duplicate should win: %#v", items[0])
				}
			},
		},
		{
			name:    "same name replacement collapses duplicate base identities",
			base:    "proxies:\n  - name: node1\n    type: ss\n    port: 1\n  - name: node1\n    type: ss\n    port: 2\n",
			overlay: "proxies:\n  - name: node1\n    type: ss\n    port: 3\n",
			check: func(t *testing.T, got map[string]any) {
				items := got["proxies"].([]any)
				if len(items) != 1 || items[0].(map[string]any)["port"] != 3 {
					t.Fatalf("proxies = %v", items)
				}
			},
		},
		{
			name:    "delete removes every duplicate identity",
			base:    "proxies:\n  - name: node1\n    type: ss\n  - name: node1\n    type: ss\nrules:\n  - GEOIP,CN,DIRECT\n  - GEOIP,CN,DIRECT\n",
			overlay: "proxies:\n  - !delete node1\nrules:\n  - !delete GEOIP,CN,DIRECT\n",
			check: func(t *testing.T, got map[string]any) {
				if items := got["proxies"].([]any); len(items) != 0 {
					t.Fatalf("proxies = %v", items)
				}
				if items := got["rules"].([]any); len(items) != 0 {
					t.Fatalf("rules = %v", items)
				}
			},
		},
		{
			name:    "mapping deep merges while scalars override",
			base:    "dns:\n  enable: true\n  ipv6: true\nport: 7890\n",
			overlay: "dns:\n  enable: false\nport: 9999\n",
			check: func(t *testing.T, got map[string]any) {
				dns := got["dns"].(map[string]any)
				if dns["enable"] != false || dns["ipv6"] != true || got["port"] != 9999 {
					t.Fatalf("result = %#v", got)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := mergeConfig(tc.base, tc.overlay)
			if err != nil {
				t.Fatalf("mergeConfig failed: %v", err)
			}
			if strings.Contains(result, "!delete") || strings.Contains(result, "!replace") {
				t.Fatalf("directive leaked into output:\n%s", result)
			}
			var got map[string]any
			if err := yaml.Unmarshal([]byte(result), &got); err != nil {
				t.Fatalf("parsing result: %v\n%s", err, result)
			}
			tc.check(t, got)
		})
	}
}

func TestMergeConfigEmptyBaseRejectsMissingDeleteWithPath(t *testing.T) {
	_, err := mergeConfig("", "dns:\n  fallback: !delete null\n")
	if err == nil || !strings.Contains(err.Error(), "dns.fallback") {
		t.Fatalf("error = %v, want path dns.fallback", err)
	}
}

func TestMergeConfigDeleteAndAddSameNameIsRejectedInEitherOrder(t *testing.T) {
	base := "proxies:\n  - name: node1\n    type: ss\n"
	for _, overlay := range []string{
		"proxies:\n  - !delete node1\n  - name: node1\n    type: ss\n",
		"proxies:\n  - name: node1\n    type: ss\n  - !delete node1\n",
	} {
		_, err := mergeConfig(base, overlay)
		if err == nil || !strings.Contains(err.Error(), "proxies.node1") {
			t.Fatalf("overlay %q error = %v, want path proxies.node1", overlay, err)
		}
	}
}

func TestUpdateConfigRejectsInvalidOverrideWithPath(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		subscriptionSourceFile: []byte("local\n"),
		subscriptionDataFile:   []byte("dns:\n  enable: true\n"),
		OverrideFilePath:       []byte("dns:\n  fallback: !delete null\n"),
	}}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	err := m.UpdateConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "dns.fallback") {
		t.Fatalf("UpdateConfig error = %v, want path dns.fallback", err)
	}
}
