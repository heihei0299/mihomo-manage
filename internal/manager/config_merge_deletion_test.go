package manager

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMergeConfigDeleteDirective(t *testing.T) {
	for _, base := range []string{"port: 7890\ndns:\n  enable: true\nmode: rule\n", "", "{}\n"} {
		result, err := mergeConfig(base, "port: !delete null\ndns: !delete ~\nmissing: !delete\nmode: rule\n")
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := yaml.Unmarshal([]byte(result), &fields); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"port", "dns", "missing"} {
			if _, exists := fields[k]; exists {
				t.Fatalf("deleted %s remains: %s", k, result)
			}
		}
		if fields["mode"] != "rule" || strings.Contains(result, "!delete") {
			t.Fatalf("result=%s", result)
		}
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
