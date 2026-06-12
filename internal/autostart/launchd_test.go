//go:build !integration

package autostart

import (
	"strings"
	"testing"
)

func TestLaunchdActivateScript(t *testing.T) {
	label := "'com.github.ghsr.runner.test'"
	plist := "'/Users/u/Library/LaunchAgents/com.github.ghsr.runner.test.plist'"
	script := launchdActivateScript(label, plist, "com.github.ghsr.runner.test.plist", false)

	for _, want := range []string{
		"GUI_DOMAIN",
		"USER_DOMAIN",
		"launchctl bootstrap",
		"launchctl kickstart",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("activate script missing %q", want)
		}
	}
	if strings.Contains(script, "launchctl load") {
		t.Error("activate script must not use deprecated launchctl load")
	}
}

func TestLaunchdActivateScriptBootoutFirst(t *testing.T) {
	label := "'com.github.ghsr.runner.test'"
	plist := "'/Users/u/Library/LaunchAgents/com.github.ghsr.runner.test.plist'"
	script := launchdActivateScript(label, plist, "com.github.ghsr.runner.test.plist", true)

	if !strings.Contains(script, "launchctl bootout") {
		t.Error("install script should bootout existing jobs first")
	}
}

func TestLaunchdBootoutScript(t *testing.T) {
	label := "'com.github.ghsr.runner.test'"
	script := LaunchdBootoutScript(label, "com.github.ghsr.runner.test.plist")

	for _, want := range []string{
		`"gui/$UID"`,
		`"user/$UID"`,
		"launchctl bootout",
		"rm -f",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("bootout script missing %q", want)
		}
	}
}
