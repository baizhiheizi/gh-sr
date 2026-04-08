package autostart

import (
	"fmt"
	"strings"
)

// SystemdUserUnit returns a systemd user unit for the GitHub Actions runner listener.
func SystemdUserUnit(instanceSanitized, absRunnerDir string) string {
	execPath := absRunnerDir + "/run.sh"
	return fmt.Sprintf(`[Unit]
Description=GitHub Actions runner (gh sr) %s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=10

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

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`, instanceSanitized, user, group, absRunnerDir, execPath)
}

// LaunchdLabel returns the launchd job label for an instance.
func LaunchdLabel(instanceSanitized string) string {
	return "com.github.ghsr.runner." + instanceSanitized
}

func xmlEscapePlist(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	s = strings.ReplaceAll(s, `<`, `&lt;`)
	s = strings.ReplaceAll(s, `>`, `&gt;`)
	return s
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
`, xmlEscapePlist(label), xmlEscapePlist(script), xmlEscapePlist(absRunnerDir))
}

// WindowsTaskName returns the scheduled task name for an instance.
func WindowsTaskName(instanceSanitized string) string {
	return "ghsr-runner-" + instanceSanitized
}
