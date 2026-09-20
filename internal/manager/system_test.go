package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOSSystemFileExistsReturnsFalseForMissingPath(t *testing.T) {
	exists, err := (OSSystem{}).FileExists(t.TempDir() + "/missing")
	if err != nil {
		t.Fatalf("FileExists returned an error for a missing path: %v", err)
	}
	if exists {
		t.Fatal("FileExists reported a missing path as existing")
	}
}

func TestOSSystemFileExistsReturnsStatError(t *testing.T) {
	exists, err := (OSSystem{}).FileExists("\x00")
	if err == nil {
		t.Fatal("FileExists should return non-NotExist stat errors")
	}
	if exists {
		t.Fatal("FileExists should not report an invalid path as existing")
	}
}

func TestOSSystemRemoveAllReturnsIOError(t *testing.T) {
	if err := (OSSystem{}).RemoveAll("\x00"); err == nil {
		t.Fatal("RemoveAll should return filesystem errors")
	}
}

func TestOSSystemRunCommandHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (OSSystem{}).RunCommand(ctx, "sleep", "10"); err == nil {
		t.Fatal("RunCommand should return an error after cancellation")
	}
}

func TestOSSystemDownloadHonorsCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- (OSSystem{}).Download(ctx, server.URL, t.TempDir()+"/artifact")
	}()
	<-started
	cancel()

	if err := <-result; err == nil {
		t.Fatal("Download should return an error after context cancellation")
	}
}

func TestExpectedChecksumFromCustomMirror(t *testing.T) {
	const expected = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.18.0/mihomo-linux-amd64-v1.18.0.gz.sha256" {
			t.Fatalf("checksum path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(expected + "  mihomo-linux-amd64-v1.18.0.gz\n"))
	}))
	defer server.Close()

	t.Setenv("MIHOMO_RELEASE_URL", server.URL+"/{version}/{asset}")
	t.Setenv("MIHOMO_RELEASE_CHECKSUM_URL", server.URL+"/{version}/{asset}.sha256")

	got, err := (OSSystem{}).ExpectedChecksum(context.Background(), "MetaCubeX", "mihomo", "v1.18.0", "mihomo-linux-amd64-v1.18.0.gz")
	if err != nil {
		t.Fatalf("ExpectedChecksum failed: %v", err)
	}
	if got != expected {
		t.Fatalf("checksum = %q, want %q", got, expected)
	}
}

func TestExpectedChecksumRequiresCustomMetadata(t *testing.T) {
	t.Setenv("MIHOMO_RELEASE_URL", "https://mirror.example/{version}/{asset}")
	t.Setenv("MIHOMO_RELEASE_CHECKSUM_URL", "")

	_, err := (OSSystem{}).ExpectedChecksum(context.Background(), "MetaCubeX", "mihomo", "v1.18.0", "mihomo-linux-amd64-v1.18.0.gz")
	if err == nil || !strings.Contains(err.Error(), "MIHOMO_RELEASE_CHECKSUM_URL") {
		t.Fatalf("ExpectedChecksum error = %v, want custom metadata requirement", err)
	}
}
