package manager

import (
	"os"
	"runtime"
	"strings"
)

func serviceUnitPath() string {
	if runtime.GOOS == "darwin" {
		return "/Library/LaunchAgents/mihomo.plist"
	}
	return "/etc/systemd/system/mihomo.service"
}

func serviceUnitContent(autoStart bool) []byte {
	if runtime.GOOS == "darwin" {
		if autoStart {
			return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>mihomo</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/mihomo/bin/mihomo</string>
    <string>-d</string>
    <string>/opt/mihomo/etc</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
</dict>
</plist>
`)
		}
		return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>mihomo</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/mihomo/bin/mihomo</string>
    <string>-d</string>
    <string>/opt/mihomo/etc</string>
  </array>
</dict>
</plist>
`)
	}
	return []byte(`[Unit]
Description=mihomo (Clash Meta) proxy
After=network.target

[Service]
Type=simple
ExecStart=/opt/mihomo/bin/mihomo -d /opt/mihomo/etc
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`)
}

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
