package manager

func platformStoragePaths() (string, string, string) {
	return "/opt/mihomo", "/opt/mihomo-manager", "/run/mihomo-manager/instance.lock"
}

func coreExecutableName() string { return "mihomo" }
