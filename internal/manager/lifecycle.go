package manager

import (
	"fmt"
	"os"
	"strings"
)

var defaultReleaseTemplate = "https://github.com/MetaCubeX/mihomo/releases/download/{version}/mihomo-{os}-{arch}-{version}.gz"

func releaseURL(goos, goarch, version string) string {
	tmpl := os.Getenv("MIHOMO_RELEASE_URL")
	if tmpl == "" {
		tmpl = defaultReleaseTemplate
		if goos == "windows" {
			arch := goarch
			if arch == "amd64" {
				arch = "amd64-v1"
			}
			return fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/mihomo-windows-%s-%s.zip", version, arch, version)
		}
	}
	r := strings.NewReplacer(
		"{os}", goos,
		"{arch}", goarch,
		"{version}", version,
	)
	return r.Replace(tmpl)
}

func releaseChecksumURL(version, assetName string) string {
	tmpl := os.Getenv("MIHOMO_RELEASE_CHECKSUM_URL")
	if tmpl == "" {
		return ""
	}
	return strings.NewReplacer(
		"{version}", version,
		"{asset}", assetName,
	).Replace(tmpl)
}
