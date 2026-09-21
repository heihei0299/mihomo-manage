package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAdoptConfigNoExistingConfig(t *testing.T) {
	fs := &fakeFileSystem{}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	_, err := m.AdoptConfig(context.Background(), false)
	if err == nil {
		t.Error("expected error when config.yaml does not exist")
	}
}

func TestAdoptConfigNoChanges(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:     true,
			subscriptionDataFile: true,
			configYAML:           true,
		},
		written: map[string][]byte{
			OverrideFilePath:     []byte("port: 8888\n"),
			subscriptionDataFile: []byte("port: 7890\nmode: rule\n"),
			configYAML:           []byte("port: 8888\nmode: rule\n"),
		},
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	report, err := m.AdoptConfig(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.NoChanges {
		t.Errorf("expected no changes, got fields=%v arrayDiff=%v", report.Fields, report.ArrayDiff)
	}
}

func TestAdoptConfigWritesScalarDiff(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:     true,
			subscriptionDataFile: true,
			configYAML:           true,
		},
		written: map[string][]byte{
			// override 既有内容（dns 补充字段）
			OverrideFilePath:     []byte("dns:\n  enable: true\n"),
			subscriptionDataFile: []byte("port: 7890\nmode: rule\n"),
			// 用户手动改了 port 与 socks-port（订阅没有 socks-port）
			configYAML: []byte("port: 9999\nmode: rule\nsocks-port: 7891\ndns:\n  enable: true\n"),
		},
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	report, err := m.AdoptConfig(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.NoChanges {
		t.Fatal("expected changes to be adopted")
	}
	if !containsStr(report.Fields, "port") || !containsStr(report.Fields, "socks-port") {
		t.Errorf("expected port and socks-port in fields, got %v", report.Fields)
	}

	// override 保留既有 dns 内容，并写入 port/socks-port
	ovr, err := fs.ReadFile(OverrideFilePath)
	if err != nil {
		t.Fatalf("override file missing: %v", err)
	}
	content := string(ovr)
	if !strings.Contains(content, "dns:") || !strings.Contains(content, "enable: true") {
		t.Errorf("override existing content should be preserved, got: %s", content)
	}
	if !strings.Contains(content, "port: 9999") || !strings.Contains(content, "socks-port: 7891") {
		t.Errorf("override should contain adopted port/socks-port, got: %s", content)
	}
}

func TestAdoptConfigArraysReportedNotWritten(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			subscriptionDataFile: true,
			configYAML:           true,
		},
		written: map[string][]byte{
			subscriptionDataFile: []byte("proxies:\n  - name: node1\n"),
			configYAML:           []byte("proxies:\n  - name: node1\n  - name: node2\n"),
		},
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	report, err := m.AdoptConfig(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(report.ArrayDiff, "proxies") {
		t.Errorf("proxies array diff should be reported, got %v", report.ArrayDiff)
	}
	if len(report.Fields) != 0 {
		t.Errorf("array diff should not be adopted as field, got %v", report.Fields)
	}
	if exists, _ := fs.FileExists(OverrideFilePath); exists {
		t.Errorf("override file should not be created for array-only diff")
	}
}

func TestAdoptConfigIdempotent(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			subscriptionDataFile: true,
			configYAML:           true,
		},
		written: map[string][]byte{
			subscriptionDataFile: []byte("port: 7890\nmode: rule\n"),
			configYAML:           []byte("port: 9999\nmode: rule\n"),
		},
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if _, err := m.AdoptConfig(context.Background(), false); err != nil {
		t.Fatalf("first adopt failed: %v", err)
	}
	report, err := m.AdoptConfig(context.Background(), false)
	if err != nil {
		t.Fatalf("second adopt failed: %v", err)
	}
	if !report.NoChanges {
		t.Errorf("second adopt should report no changes, got fields=%v", report.Fields)
	}
}

func TestAdoptConfigLargeDiffNeedsForce(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			subscriptionDataFile: true,
			configYAML:           true,
		},
		written: map[string][]byte{
			subscriptionDataFile: []byte("port: 7890\nmode: rule\nlog-level: info\nallow-lan: false\nexternal-controller: 127.0.0.1:9090\n"),
			// 五个字段全部不同
			configYAML: []byte("port: 1111\nmode: global\nlog-level: debug\nallow-lan: true\nexternal-controller: 127.0.0.1:9999\n"),
		},
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	report, err := m.AdoptConfig(context.Background(), false)
	if !errors.Is(err, ErrAdoptNeedsConfirmation) {
		t.Fatalf("expected ErrAdoptNeedsConfirmation, got %v", err)
	}
	if !report.LargeDiff || len(report.Fields) != 5 {
		t.Errorf("expected 5 fields and LargeDiff, got %v", report)
	}
	if exists, _ := fs.FileExists(OverrideFilePath); exists {
		t.Error("adopt should not write without force on large diff")
	}

	report, err = m.AdoptConfig(context.Background(), true)
	if err != nil {
		t.Fatalf("force adopt failed: %v", err)
	}
	if len(report.Fields) != 5 {
		t.Errorf("expected 5 adopted fields with force, got %v", report.Fields)
	}
	if _, err := fs.ReadFile(OverrideFilePath); err != nil {
		t.Error("override file should be created after forced adopt")
	}
}

func containsStr(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}
