package autostart

import (
	"fmt"

	"github.com/an-lee/gh-sr/internal/hostshell"
)

// SystemdUserUnit returns a systemd user unit for the GitHub Actions runner listener.
func SystemdUserUnit(instanceSanitized, absRunnerDir string) string {
	execPath := absRunnerDir + "/run.sh"
	return fmt.Sprintf(`[Unit]
Description=GitHub Actions runner (gh sr) %s
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=10
RestartPreventExitStatus=203

[Install]
WantedBy=default.target
`, instanceSanitized, absRunnerDir, execPath)
}

// SystemdSystemUnit returns a systemd system unit running the listener as a specific user.
func SystemdSystemUnit(instanceSanitized, absRunnerDir, user, group string) string {
	execPath := absRunnerDir + "/run.sh"
	return fmt.Sprintf(`[Unit]
Description=GitHub Actions runner (gh sr) %s
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=10
RestartPreventExitStatus=203

[Install]
WantedBy=multi-user.target
`, instanceSanitized, user, group, absRunnerDir, execPath)
}

// LaunchdLabel returns the launchd job label for an instance.
func LaunchdLabel(instanceSanitized string) string {
	return "com.github.ghsr.runner." + instanceSanitized
}

// LaunchdPlist returns a LaunchAgent plist that runs run.sh in the runner directory.
func LaunchdPlist(instanceSanitized, absRunnerDir string) string {
	label := LaunchdLabel(instanceSanitized)
	script := absRunnerDir + "/run.sh"
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>WorkingDirectory</key>
	<string>%s</string>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`, hostshell.PlistEscape(label), hostshell.PlistEscape(script), hostshell.PlistEscape(absRunnerDir))
}

// WindowsTaskName returns the scheduled task name for an instance.
func WindowsTaskName(instanceSanitized string) string {
	return "ghsr-runner-" + instanceSanitized
}
