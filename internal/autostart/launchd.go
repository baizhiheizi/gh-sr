package autostart

import "fmt"

// launchdDomainList is the canonical list of launchctl domains searched in
// order when probing or mutating a LaunchAgent. The order matches the modern
// macOS preference: gui/$UID (when a GUI session is present) precedes
// user/$UID. The returned string is a pre-quoted shell word list, intended
// to be spliced directly into a `for _DOMAIN in ...; do` loop body via
// fmt.Sprintf (no shell-variable assignment in between).
//
// The list was previously duplicated as a literal `for _DOMAIN in
// "gui/$UID" "user/$UID"` block in four places (this file's three helpers
// plus the KindLaunchd arm of autostart.go::Stop). Drift here is a real
// failure mode — see commit 904dd60 where the analogous launchd remoteHome
// list diverged between Start and Stop.
func launchdDomainList() string {
	return `"gui/$UID" "user/$UID"`
}

// launchdActivateScript loads or starts a LaunchAgent in ~/Library/LaunchAgents.
//
// Modern macOS expects LaunchAgents in the gui/$UID domain when a GUI session exists.
// Over SSH with no GUI login, bootstrap may fail in both gui and user domains; callers
// should fall back to a direct run.sh start (see runner.Manager.Start on darwin).
//
// qlabel and qplist must already be posixSingleQuote'd. plistFileName is the basename
// (e.g. com.github.ghsr.runner.foo.plist).
func launchdActivateScript(qlabel, qplist, plistFileName string, bootoutFirst bool) string {
	domains := launchdDomainList()
	bootout := ""
	if bootoutFirst {
		bootout = fmt.Sprintf(`
for _DOMAIN in %s; do
  launchctl bootout "$_DOMAIN/$LABEL" 2>/dev/null || true
done
`, domains)
	}
	return fmt.Sprintf(`set -e
UID=$(id -u)
GUI_DOMAIN="gui/$UID"
USER_DOMAIN="user/$UID"
LABEL=%s
PLIST=%s
%s
_DOMAIN=""
if launchctl print "$GUI_DOMAIN/$LABEL" >/dev/null 2>&1; then
  _DOMAIN="$GUI_DOMAIN"
elif launchctl print "$USER_DOMAIN/$LABEL" >/dev/null 2>&1; then
  _DOMAIN="$USER_DOMAIN"
fi
if [ -n "$_DOMAIN" ]; then
  launchctl kickstart -k "$_DOMAIN/$LABEL"
  exit 0
fi
for _DOMAIN in %s; do
  if launchctl bootstrap "$_DOMAIN" "$PLIST" 2>/dev/null; then
    launchctl enable "$_DOMAIN/$LABEL" 2>/dev/null || true
    launchctl kickstart -k "$_DOMAIN/$LABEL" 2>/dev/null || true
    exit 0
  fi
done
exit 1
`, qlabel, qplist, bootout, domains)
}

// LaunchdBootoutScript unloads a LaunchAgent from both gui and user domains.
// qlabel must already be posixSingleQuote'd.
func LaunchdBootoutScript(qlabel, plistFileName string) string {
	return fmt.Sprintf(`set -e
UID=$(id -u)
LABEL=%s
PLIST="$HOME/Library/LaunchAgents/%s"
for _DOMAIN in %s; do
  launchctl bootout "$_DOMAIN/$LABEL" 2>/dev/null || true
done
launchctl unload -w "$PLIST" 2>/dev/null || true
rm -f "$PLIST"
`, qlabel, plistFileName, launchdDomainList())
}

// launchdPrintScript returns launchctl print output for the first domain that has the job.
func launchdPrintScript(qlabel string) string {
	return fmt.Sprintf(`UID=$(id -u)
LABEL=%s
for _DOMAIN in %s; do
  if launchctl print "$_DOMAIN/$LABEL" 2>/dev/null; then
    exit 0
  fi
done
echo unknown
`, qlabel, launchdDomainList())
}
