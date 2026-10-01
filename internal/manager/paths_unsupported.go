//go:build !linux && !windows

package manager

func platformStoragePaths() (string, string, string) { return "", "", "" }
func coreExecutableName() string                     { return "mihomo" }
