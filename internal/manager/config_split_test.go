package manager

import (
	"context"
	"strings"
	"testing"
)

func TestFetchSubscriptionUpdatesDataWithoutTouchingConfig(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{},
		written: map[string][]byte{
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte("https://example.com/sub"),
			configYAML:             []byte("port: 7890\n"),
		},
	}
	source := &fakeReleaseSource{downloadData: []byte("proxies:\n  - name: fresh\n")}
	linkStorage(fs, source)
	m := newTestConfigManager(fs, source, &passValidator{}, noopReload)

	fetched, err := m.FetchSubscription(context.Background())
	if err != nil || !fetched {
		t.Fatalf("FetchSubscription = %v, %v; want fetched", fetched, err)
	}
	if got := string(fs.written[subscriptionDataFile]); got != "proxies:\n  - name: fresh\n" {
		t.Fatalf("subscription data = %q, want fetched content", got)
	}
	if got := string(fs.written[configYAML]); got != "port: 7890\n" {
		t.Fatalf("config.yaml = %q, fetch must not touch config", got)
	}
	if _, exists := fs.written[configApplyStatusFile]; exists {
		t.Fatal("fetch must not record a config apply status")
	}
}

func TestFetchSubscriptionLocalSourceIsNoop(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{},
		written: map[string][]byte{
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("port: 7890\n"),
			configYAML:             []byte("port: 7890\n"),
		},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	m := newTestConfigManager(fs, source, &passValidator{}, noopReload)

	fetched, err := m.FetchSubscription(context.Background())
	if err != nil || fetched {
		t.Fatalf("FetchSubscription = %v, %v; want no fetch for a local source", fetched, err)
	}
	if source.downloadCalled {
		t.Fatal("local source must not download")
	}
	if got := string(fs.written[configYAML]); got != "port: 7890\n" {
		t.Fatalf("config.yaml = %q, want unchanged", got)
	}
}

func TestApplyConfigUsesStoredDataWithoutDownloading(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{},
		written: map[string][]byte{
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte("https://example.com/sub"),
			subscriptionDataFile:   []byte("port: 7890\nmode: rule\n"),
			OverrideFilePath:       []byte("mode: global\n"),
		},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	m := newTestConfigManager(fs, source, &passValidator{}, noopReload)

	if err := m.ApplyConfig(context.Background()); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}
	if source.downloadCalled {
		t.Fatal("apply must not download subscription data")
	}
	config := string(fs.written[configYAML])
	if !strings.Contains(config, "mode: global") || !strings.Contains(config, "port: 7890") {
		t.Fatalf("config.yaml = %q, want merged stored data + override", config)
	}
}

func TestApplyConfigRejectsUnconfiguredSource(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}, written: map[string][]byte{}}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.ApplyConfig(context.Background()); err == nil {
		t.Fatal("ApplyConfig without a subscription source should fail")
	}
	if _, exists := fs.written[configYAML]; exists {
		t.Fatal("rejected apply must not write config.yaml")
	}
}

func TestLastConfigApplyReportsStaleSubscription(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{},
		written: map[string][]byte{
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("port: 7890\n"),
		},
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.ApplyConfig(context.Background()); err != nil {
		t.Fatalf("ApplyConfig failed: %v", err)
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != ConfigApplied || status.SubscriptionStale {
		t.Fatalf("status = %+v, want applied and fresh", status)
	}

	// A fetch-only update replaces stored data without re-rendering config.
	fs.written[subscriptionDataFile] = []byte("port: 9999\n")
	status, err = m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.SubscriptionStale {
		t.Fatalf("status = %+v, want stale subscription", status)
	}
}
