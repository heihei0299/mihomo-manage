package manager

import (
	"strings"
	"testing"
)

func TestMergeConfigEmptyBase(t *testing.T) {
	result, err := mergeConfig("", `port: 7890`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "port: 7890") {
		t.Errorf("empty base should return overlay, got: %s", result)
	}
}

func TestMergeConfigEmptyOverlay(t *testing.T) {
	result, err := mergeConfig("port: 7890", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "port: 7890") {
		t.Errorf("empty overlay should return base, got: %s", result)
	}
}

func TestMergeConfigAppendFields(t *testing.T) {
	base := `proxies:
  - name: node1
    type: ss
proxy-groups:
  - name: Proxy
    type: select
rules:
  - DOMAIN-SUFFIX,example.com,Proxy
proxy-providers:
  provider1:
    type: http
    url: "https://example.com/provider1"
rule-providers:
  provider1:
    type: http
    behavior: domain
    url: "https://example.com/rules1"`

	overlay := `proxies:
  - name: node2
    type: ss
proxy-groups:
  - name: AdBlock
    type: reject
rules:
  - DOMAIN-SUFFIX,google.com,Direct
proxy-providers:
  provider2:
    type: http
    url: "https://example.com/provider2"
rule-providers:
  provider2:
    type: http
    behavior: domain
    url: "https://example.com/rules2"`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		field   string
		baseVal string
		overVal string
	}{
		{"proxies", "node1", "node2"},
		{"proxy-groups", "Proxy", "AdBlock"},
		{"rules", "example.com", "google.com"},
		{"proxy-providers", "provider1", "provider2"},
		{"rule-providers", "provider1", "provider2"},
	}

	for _, c := range checks {
		if !strings.Contains(result, c.baseVal) {
			t.Errorf("'%s' should contain base value %q after merge, got: %s", c.field, c.baseVal, result)
		}
		if !strings.Contains(result, c.overVal) {
			t.Errorf("'%s' should contain overlay value %q after append merge, got: %s", c.field, c.overVal, result)
		}
	}
}

func TestMergeConfigTopLevelScalarOverrides(t *testing.T) {
	base := `port: 7890
socks-port: 7891
mode: rule
log-level: info
allow-lan: false
external-controller: 127.0.0.1:9090`
	overlay := `port: 8888
mode: global
log-level: debug`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "port: 8888") {
		t.Errorf("overlay port should override base port, got: %s", result)
	}
	if strings.Contains(result, "port: 7890") {
		t.Errorf("base port should be replaced, got: %s", result)
	}
	if !strings.Contains(result, "mode: global") {
		t.Errorf("overlay mode should override base mode, got: %s", result)
	}
	if !strings.Contains(result, "log-level: debug") {
		t.Errorf("overlay log-level should override base log-level, got: %s", result)
	}
}

func TestMergeConfigMixed(t *testing.T) {
	base := `port: 7890
mode: rule
proxies:
  - name: node1
    type: ss
proxy-groups:
  - name: Proxy
    type: select`

	overlay := `socks-port: 7891
proxies:
  - name: node2
    type: ss
rules:
  - MATCH,Proxy
dns:
  enable: true
  nameservers:
    - 8.8.8.8`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "socks-port: 7891") {
		t.Errorf("socks-port should be supplemented, got: %s", result)
	}
	if !strings.Contains(result, "node1") || !strings.Contains(result, "node2") {
		t.Errorf("proxies should contain both node1 and node2, got: %s", result)
	}
	if !strings.Contains(result, "MATCH,Proxy") {
		t.Errorf("rules from overlay should be appended, got: %s", result)
	}
	if !strings.Contains(result, "dns:") || !strings.Contains(result, "enable: true") {
		t.Errorf("dns section should be supplemented from overlay, got: %s", result)
	}
}

func TestMergeConfigListOverlay(t *testing.T) {
	base := `port: 7890`
	overlay := `- item1
- item2`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "item1") {
		t.Errorf("non-map overlay should replace map base, got: %s", result)
	}
}

func TestMergeConfigInvalidBaseYAML(t *testing.T) {
	_, err := mergeConfig("{invalid: yaml: broken", "port: 7890")
	if err == nil {
		t.Error("expected error for invalid base YAML")
	}
}

func TestMergeConfigInvalidOverlayYAML(t *testing.T) {
	_, err := mergeConfig("port: 7890", "{invalid: yaml: broken")
	if err == nil {
		t.Error("expected error for invalid overlay YAML")
	}
}

func TestMergeConfigNonAppendArrayOverrides(t *testing.T) {
	base := `listen:
  - 0.0.0.0:9090
  - 127.0.0.1:9091`
	overlay := `listen:
  - 0.0.0.0:8080`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "8080") {
		t.Errorf("overlay should override 'listen' (not in append list), got: %s", result)
	}
	if strings.Contains(result, "9090") {
		t.Errorf("base 'listen' values should be replaced, got: %s", result)
	}
}

func TestMergeConfigNestedMapOverrides(t *testing.T) {
	base := `dns:
  enable: true
  nameservers:
    - 8.8.8.8
    - 1.1.1.1`
	overlay := `dns:
  enable: false
  nameservers:
    - 9.9.9.9
  ipv6: true`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "enable: false") {
		t.Errorf("overlay should override existing 'dns.enable', got: %s", result)
	}
	if strings.Contains(result, "8.8.8.8") {
		t.Errorf("overlay should replace 'dns.nameservers' (not in append list), got: %s", result)
	}
	if !strings.Contains(result, "9.9.9.9") {
		t.Errorf("overlay nameservers should replace base ones, got: %s", result)
	}
	if !strings.Contains(result, "ipv6: true") {
		t.Errorf("overlay should supplement missing 'dns.ipv6', got: %s", result)
	}
}

func TestMergeConfigScalarOverrideAndSupplement(t *testing.T) {
	base := `port: 7890
mode: rule`
	overlay := `port: 9999
socks-port: 7891`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "port: 9999") {
		t.Errorf("overlay should override existing scalar 'port', got: %s", result)
	}
	if strings.Contains(result, "port: 7890") {
		t.Errorf("base port should be replaced, got: %s", result)
	}
	if !strings.Contains(result, "socks-port: 7891") {
		t.Errorf("overlay should supplement missing 'socks-port', got: %s", result)
	}
}

func TestMergeConfigReplaceTag(t *testing.T) {
	base := `proxies:
  - name: node1
    type: ss
proxy-groups:
  - name: Proxy
    type: select
rules:
  - DOMAIN-SUFFIX,example.com,Proxy`
	overlay := `proxies: !replace
  - name: local-node
    type: ss
rules: !replace
  - MATCH,Direct`

	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "local-node") {
		t.Errorf("replace overlay proxy should be present, got: %s", result)
	}
	if strings.Contains(result, "node1") {
		t.Errorf("base proxies should be fully replaced, got: %s", result)
	}
	if !strings.Contains(result, "Proxy") {
		t.Errorf("non-replace proxy-groups should be kept, got: %s", result)
	}
	if !strings.Contains(result, "MATCH,Direct") {
		t.Errorf("replace overlay rules should be present, got: %s", result)
	}
	if strings.Contains(result, "example.com") {
		t.Errorf("base rules should be fully replaced, got: %s", result)
	}
}

func TestMergeConfigReplaceTagEmptyBase(t *testing.T) {
	result, err := mergeConfig("", "proxies: !replace\n  - name: local-node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "!replace") {
		t.Errorf("!replace tag should be stripped even with empty base, got: %s", result)
	}
	if !strings.Contains(result, "local-node") {
		t.Errorf("overlay content should be kept, got: %s", result)
	}
}
