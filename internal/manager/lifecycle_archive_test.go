package manager

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"
)

func windowsReleaseZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	for name, content := range files {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestWindowsReleaseURLsUseZIPAndBaselineAMD64(t *testing.T) {
	t.Setenv("MIHOMO_RELEASE_URL", "")
	for arch, suffix := range map[string]string{"amd64": "mihomo-windows-amd64-v1-v1.19.32.zip", "arm64": "mihomo-windows-arm64-v1.19.32.zip"} {
		if got := releaseURL("windows", arch, "v1.19.32"); !strings.HasSuffix(got, suffix) {
			t.Fatalf("release URL = %s", got)
		}
	}
}

func TestReleaseZIPExtractsOnlyCoreToManagedPath(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{"core.zip": windowsReleaseZIP(t, map[string]string{"folder/mihomo.exe": "core", "../../unmanaged.txt": "ignored"})}}
	m := &lifecycleManager{fs: fs}
	if err := m.decompressRelease("core.zip", "staged"); err != nil {
		t.Fatal(err)
	}
	if string(fs.written["staged"]) != "core" || len(fs.written) != 2 {
		t.Fatalf("files = %v", fs.written)
	}
}

func TestReleaseZIPRejectsMissingOrAmbiguousCore(t *testing.T) {
	for _, files := range []map[string]string{{"readme.txt": "no core"}, {"mihomo.exe": "first", "other.exe": "second"}} {
		fs := &fakeFileSystem{written: map[string][]byte{"core.zip": windowsReleaseZIP(t, files)}}
		if err := (&lifecycleManager{fs: fs}).decompressRelease("core.zip", "staged"); err == nil {
			t.Fatal("accepted invalid archive")
		}
		if _, ok := fs.written["staged"]; ok {
			t.Fatal("deployed ambiguous archive")
		}
	}
}

func TestLocalZIPInstallPreservesArchive(t *testing.T) {
	archive := windowsReleaseZIP(t, map[string]string{"mihomo.exe": "core"})
	fs := &fakeFileSystem{fileExists: map[string]bool{"local.zip": true}, written: map[string][]byte{"local.zip": archive}}
	m := newTestLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{}, noopScheduleManager{})
	if err := m.InstallFromLocal(context.Background(), "local.zip", false, noopProgress); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fs.written["local.zip"], archive) || string(fs.written[binaryPath]) != "core" {
		t.Fatalf("archive or deployed core changed")
	}
}
