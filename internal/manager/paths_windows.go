package manager

import (
	"os"
	"path/filepath"
)

func platformStoragePaths() (string, string, string) {
	root := os.Getenv("ProgramData")
	if root == "" {
		root = `C:\ProgramData`
	}
	return filepath.Join(root, "mihomo"), filepath.Join(root, "mihomo-manager"), filepath.Join(root, "mihomo-manager-locks", "instance.lock")
}

func coreExecutableName() string { return "mihomo.exe" }
