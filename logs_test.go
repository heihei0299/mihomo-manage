package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLogFileTailPreservesPartialLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mihomo.log")
	if err := os.WriteFile(path, []byte("first\nsecond\npartial"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := streamLogFile(context.Background(), path, &out, 2, false); err != nil {
		t.Fatal(err)
	}
	if out.String() != "second\npartial" {
		t.Fatalf("logs = %q", out.String())
	}
}

func TestLogFileFollowStopsOnCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mihomo.log")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := streamLogFile(ctx, path, &bytes.Buffer{}, 0, true); err != nil {
		t.Fatal(err)
	}
}
