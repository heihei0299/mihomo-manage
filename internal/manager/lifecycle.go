package manager

import (
	"os"
	"strings"
)

var defaultReleaseTemplate = "https://github.com/MetaCubeX/mihomo/releases/download/{version}/mihomo-{os}-{arch}-{version}.gz"

func releaseURL(goos, goarch, version string) string {
	tmpl := os.Getenv("MIHOMO_RELEASE_URL")
	if tmpl == "" {
		tmpl = defaultReleaseTemplate
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
