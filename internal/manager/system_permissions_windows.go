package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func protectPathPermissions(path string, perm uint32) error {
	// Managed roots contain SYSTEM-executed binaries as well as credentials.
	// Protect their parents so another user cannot replace these files.
	for _, root := range []string{installRoot, managerRoot} {
		cleanPath, cleanRoot := strings.ToLower(filepath.Clean(path)), strings.ToLower(filepath.Clean(root))
		if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
			if _, err := os.Stat(root); err == nil {
				if err := setPrivateWindowsACL(root); err != nil {
					return err
				}
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}
	if perm != filePermPrivateRW && perm != dirPermPrivate {
		return nil
	}
	return setPrivateWindowsACL(path)
}

func setPrivateWindowsACL(path string) error {
	current, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := current.Owner()
	if err != nil {
		return err
	}
	// Match owner-only Unix storage: the owner, Administrators, and SYSTEM
	// retain access; inherited access for other users is removed.
	sddl := "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;FA;;;" + owner.String() + ")"
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		return fmt.Errorf("setting private Windows permissions for %s: %w", path, err)
	}
	return nil
}
