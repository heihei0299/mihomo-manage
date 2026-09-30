package manager

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAdoptConfigDeletionsArePreservedAndIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name, subscription, override, current, directive string
		fields                                           []string
	}{
		{"top-level-scalar", "port: 7890\nmode: rule\n", "port: 9999\n", "mode: rule\n", "port: !delete", []string{"port"}},
		{"top-level-map", "mode: rule\ndns:\n  enable: true\n", "dns: !replace\n  enable: false\n", "mode: rule\n", "dns: !delete", []string{"dns"}},
		{"nested-key", "dns:\n  enable: true\n  ipv6: true\n", "dns:\n  ipv6: false\n", "dns:\n  enable: true\n", "dns: !replace", []string{"dns"}},
		{"deep-nested-key", "dns:\n  enable: true\n  nameserver-policy:\n    example.com: 1.1.1.1\n    example.org: 8.8.8.8\n", "", "dns:\n  enable: true\n  nameserver-policy:\n    example.com: 1.1.1.1\n", "dns: !replace", []string{"dns"}},
		{"empty-map", "dns:\n  enable: true\n", "", "dns: {}\n", "dns: !replace", []string{"dns"}},
		{"override-only-field", "mode: rule\n", "external-controller: 127.0.0.1:9090\n", "mode: rule\n", "external-controller: !delete", []string{"external-controller"}},
		{"delete-last-field", "port: 7890\n", "", "{}\n", "port: !delete", []string{"port"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := &fakeFileSystem{written: map[string][]byte{
				subscriptionDataFile:   []byte(tc.subscription),
				subscriptionSourceFile: []byte("local\n"),
				OverrideFilePath:       []byte(tc.override),
				configYAML:             []byte(tc.current),
			}}
			m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
			report, err := m.AdoptConfig(context.Background(), false)
			if err != nil || report.NoChanges || !reflect.DeepEqual(report.Fields, tc.fields) || len(report.ArrayDiff) != 0 {
				t.Fatalf("adopt report=%+v err=%v", report, err)
			}
			if !strings.Contains(string(fs.written[OverrideFilePath]), tc.directive) {
				t.Fatalf("override missing %s: %s", tc.directive, fs.written[OverrideFilePath])
			}
			preview, err := m.PreviewConfig(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var current, generated map[string]any
			if err := yaml.Unmarshal([]byte(tc.current), &current); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal([]byte(preview), &generated); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(current, generated) {
				t.Fatalf("generated=%s, want current=%s", preview, tc.current)
			}
			if strings.Contains(preview, "!delete") || strings.Contains(preview, "!replace") {
				t.Fatal("overlay directive leaked into generated config")
			}
			again, err := m.AdoptConfig(context.Background(), false)
			if err != nil || !again.NoChanges {
				t.Fatalf("second adopt=%+v err=%v", again, err)
			}
			if err := m.UpdateConfig(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(fs.written[configYAML], &generated); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(current, generated) {
				t.Fatal("update resurrected deleted keys")
			}
		})
	}
}

func TestAdoptConfigReportsDeletedArrays(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		subscriptionDataFile:   []byte("mode: rule\nproxies: []\nrules:\n  - MATCH,DIRECT\n"),
		subscriptionSourceFile: []byte("local\n"),
		OverrideFilePath:       []byte("mode: rule\n"),
		configYAML:             []byte("mode: rule\n"),
	}}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
	report, err := m.AdoptConfig(context.Background(), false)
	if err != nil || report.NoChanges || len(report.Fields) != 0 || !reflect.DeepEqual(report.ArrayDiff, []string{"proxies", "rules"}) {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if string(fs.written[OverrideFilePath]) != "mode: rule\n" {
		t.Fatal("array-only deletion changed the overlay")
	}
}

func TestAdoptConfigCanReintroduceDeletedField(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		subscriptionDataFile:   []byte("port: 7890\nmode: rule\n"),
		subscriptionSourceFile: []byte("local\n"),
		OverrideFilePath:       []byte("port: !delete null\n"),
		configYAML:             []byte("port: 9999\nmode: rule\n"),
	}}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
	if _, err := m.AdoptConfig(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	preview, err := m.PreviewConfig(context.Background())
	if err != nil || !strings.Contains(preview, "port: 9999") || strings.Contains(string(fs.written[OverrideFilePath]), "!delete") {
		t.Fatalf("preview=%s err=%v override=%s", preview, err, fs.written[OverrideFilePath])
	}
	again, err := m.AdoptConfig(context.Background(), false)
	if err != nil || !again.NoChanges {
		t.Fatalf("second adopt=%+v err=%v", again, err)
	}
}

func TestAdoptConfigLargeDeletionRequiresForce(t *testing.T) {
	const override = "mode: rule\n# preserve until confirmed\n"
	fs := &fakeFileSystem{written: map[string][]byte{
		subscriptionDataFile:   []byte("port: 7890\nsocks-port: 7891\nmode: rule\nlog-level: info\nallow-lan: false\n"),
		subscriptionSourceFile: []byte("local\n"),
		OverrideFilePath:       []byte(override),
		configYAML:             []byte("{}\n"),
	}}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
	report, err := m.AdoptConfig(context.Background(), false)
	if !errors.Is(err, ErrAdoptNeedsConfirmation) || !report.LargeDiff || len(report.Fields) != 5 || string(fs.written[OverrideFilePath]) != override {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if _, err := m.AdoptConfig(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	again, err := m.AdoptConfig(context.Background(), false)
	if err != nil || !again.NoChanges {
		t.Fatalf("second adopt=%+v err=%v", again, err)
	}
}
