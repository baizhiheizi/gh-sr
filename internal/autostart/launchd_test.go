//go:build !integration

package autostart

import (
	"fmt"
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

// TestLaunchdDomainList_IsSingleSourceOfTruth verifies the four call sites
// that previously inlined `for _DOMAIN in "gui/$UID" "user/$UID"; do` all
// emit the canonical word list produced by launchdDomainList(). Catches
// drift if a future change adds (or drops) a domain in one helper but not
// the others.
func TestLaunchdDomainList_IsSingleSourceOfTruth(t *testing.T) {
	domains := launchdDomainList()
	wantLoop := fmt.Sprintf("for _DOMAIN in %s; do", domains)

	activate := launchdActivateScript("'label'", "'plist'", "name.plist", false)
	activateBootout := launchdActivateScript("'label'", "'plist'", "name.plist", true)
	bootout := LaunchdBootoutScript("'label'", "name.plist")
	printScript := launchdPrintScript("'label'")

	cases := []struct {
		name   string
		script string
		want   int // expected occurrences of the canonical for loop
	}{
		{"activate", activate, 1},               // the bootstrap loop
		{"activateBootout", activateBootout, 2}, // bootoutFirst + bootstrap loop
		{"bootout", bootout, 1},                 // the single bootout loop
		{"print", printScript, 1},               // the single print loop
	}
	for _, tc := range cases {
		got := strings.Count(tc.script, wantLoop)
		if got != tc.want {
			t.Errorf("%s: expected %d occurrences of %q, got %d\nscript:\n%s",
				tc.name, tc.want, wantLoop, got, tc.script)
		}
	}
}

// TestLaunchdDomainList_ReturnsPreQuotedTokens documents and locks the
// helper's contract: the returned string must be a valid shell word list
// where each domain is its own pre-quoted token, so splicing it into
// `for _DOMAIN in ...; do` yields exactly two iterations.
func TestLaunchdDomainList_ReturnsPreQuotedTokens(t *testing.T) {
	got := launchdDomainList()
	want := `"gui/$UID" "user/$UID"`
	if got != want {
		t.Errorf("launchdDomainList() = %q, want %q", got, want)
	}
	// Sanity: the canonical snippet is well-formed shell.
	if strings.Count(got, `"`) != 4 {
		t.Errorf("expected 4 double quotes (2 per domain), got %d in %q",
			strings.Count(got, `"`), got)
	}
}
