package manager

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeConfigDeleteDirective(t *testing.T) {
	base := "port: 7890\ndns:\n  enable: true\nmode: rule\n"
	result, err := mergeConfig(base, "port: !delete null\ndns: !delete ~\nmode: rule\n")
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(result), &fields); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"port", "dns"} {
		if _, exists := fields[k]; exists {
			t.Fatalf("deleted %s remains: %s", k, result)
		}
	}
	if fields["mode"] != "rule" || strings.Contains(result, "!delete") {
		t.Fatalf("result=%s", result)
	}
}

func TestMergeConfigRejectsDeleteDirectiveWithValue(t *testing.T) {
	for _, base := range []string{"mode: rule\n", "", "{}\n"} {
		for _, overlay := range []string{"port: !delete 7890\n", "dns: !delete {enable: true}\n", "rules: !delete [MATCH,DIRECT]\n"} {
			if _, err := mergeConfig(base, overlay); err == nil {
				t.Fatalf("accepted invalid deletion: %s", overlay)
			}
		}
	}
}

func TestMergeConfigDeleteMissingKeyIsAnErrorWithPath(t *testing.T) {
	for _, tc := range []struct {
		name, base, overlay, path string
	}{
		{"empty base", "", "port: !delete null\n", "port"},
		{"empty mapping base", "{}\n", "port: !delete null\n", "port"},
		{"nested key", "dns:\n  enable: true\n", "dns:\n  fallback: !delete null\n", "dns.fallback"},
		{"nested empty base", "dns:\n  enable: true\n", "dns:\n  nameservers: !delete null\n", "dns.nameservers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := mergeConfig(tc.base, tc.overlay)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("error %q does not contain path %q", err, tc.path)
			}
		})
	}
}

func TestMergeConfigNestedDeleteAppliesAtAnyLevel(t *testing.T) {
	base := `dns:
  enable: true
  fallback:
    - 8.8.8.8
  nameserver-policy:
    example.com: 1.1.1.1
    example.org: 8.8.8.8
`
	overlay := `dns:
  fallback: !delete null
  nameserver-policy:
    example.org: !delete null
`
	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal([]byte(result), &got); err != nil {
		t.Fatal(err)
	}
	dns, ok := got["dns"].(map[string]any)
	if !ok {
		t.Fatalf("dns missing: %s", result)
	}
	if _, exists := dns["fallback"]; exists {
		t.Fatalf("nested !delete left fallback behind: %s", result)
	}
	policy, ok := dns["nameserver-policy"].(map[string]any)
	if !ok {
		t.Fatalf("nameserver-policy missing: %s", result)
	}
	if _, exists := policy["example.org"]; exists {
		t.Fatalf("deep !delete left example.org behind: %s", result)
	}
	if policy["example.com"] != "1.1.1.1" || dns["enable"] != true {
		t.Fatalf("unrelated fields changed: %s", result)
	}
	if strings.Contains(result, "!delete") {
		t.Fatalf("directive leaked into output: %s", result)
	}
}

func TestMergeConfigReplaceAppliesAtAnyLevel(t *testing.T) {
	base := `dns:
  enable: true
  ipv6: true
  nameserver-policy:
    example.com: 1.1.1.1
tun:
  enable: false
  stack: system
`
	overlay := `dns:
  ipv6: !replace
    enable: false
`
	result, err := mergeConfig(base, overlay)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := yaml.Unmarshal([]byte(result), &got); err != nil {
		t.Fatal(err)
	}
	dns := got["dns"].(map[string]any)
	ipv6, ok := dns["ipv6"].(map[string]any)
	if !ok || ipv6["enable"] != false {
		t.Fatalf("nested !replace should replace the subtree: %s", result)
	}
	if dns["enable"] != true {
		t.Fatalf("sibling field should be inherited: %s", result)
	}
	if _, exists := dns["nameserver-policy"]; !exists {
		t.Fatalf("unrelated nested field should remain: %s", result)
	}
	if strings.Contains(result, "!replace") {
		t.Fatalf("directive leaked into output: %s", result)
	}
}

func TestMergeConfigRejectsNestedDeleteInsideReplaceWithPath(t *testing.T) {
	for _, tc := range []struct {
		name, overlay, path string
	}{
		{"mapping", "dns: !replace\n  fallback: !delete null\n", "dns.fallback"},
		{"sequence", "rules: !replace\n  - !delete MATCH,DIRECT\n", "rules[0]"},
		{"identity item", "proxies:\n  - name: n1\n    udp: !delete null\n", "proxies.n1.udp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := "dns:\n  fallback: 8.8.8.8\nrules:\n  - MATCH,DIRECT\nproxies:\n  - name: n1\n    udp: true\n"
			_, err := mergeConfig(base, tc.overlay)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("error %q does not contain path %q", err, tc.path)
			}
		})
	}
}
