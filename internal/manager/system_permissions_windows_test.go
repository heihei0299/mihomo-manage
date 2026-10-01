package manager

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsPrivateStorageRemovesInheritedUserAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	fs := OSSystem{}
	if err := fs.MkdirAll(path, dirPermPrivate); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, "subscription.txt")
	if err := fs.WriteFile(file, []byte("synthetic"), filePermPrivateRW); err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{path, file} {
		descriptor, err := windows.GetNamedSecurityInfo(item, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		sddl := descriptor.String()
		if !strings.Contains(sddl, "D:P") || !strings.Contains(sddl, ";;;SY)") || !strings.Contains(sddl, ";;;BA)") || strings.Contains(sddl, ";;;BU)") || strings.Contains(sddl, ";;;WD)") {
			t.Fatalf("private ACL = %s", sddl)
		}
	}
	if data, err := fs.ReadFile(file); err != nil || string(data) != "synthetic" {
		t.Fatalf("private file = %q, %v", data, err)
	}
}
