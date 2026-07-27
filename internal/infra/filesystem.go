package infra

import "os"

// FileSystem provides file operations.
type FileSystem interface {
	FileExists(path string) bool
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm uint32) error
	Remove(path string) error
	Rename(oldPath, newPath string) error
	MkdirAll(path string, perm uint32) error
	Chmod(path string, perm uint32) error
}

// OSFileSystem implements FileSystem using the real OS.
type OSFileSystem struct{}

// NewFileSystem returns a new OSFileSystem.
func NewFileSystem() *OSFileSystem { return &OSFileSystem{} }

func (OSFileSystem) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (OSFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (OSFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	return os.WriteFile(path, data, os.FileMode(perm))
}

func (OSFileSystem) Remove(path string) error {
	return os.RemoveAll(path)
}

func (OSFileSystem) Rename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func (OSFileSystem) MkdirAll(path string, perm uint32) error {
	return os.MkdirAll(path, os.FileMode(perm))
}

func (OSFileSystem) Chmod(path string, perm uint32) error {
	return os.Chmod(path, os.FileMode(perm))
}
