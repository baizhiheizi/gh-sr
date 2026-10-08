package autostart

import (
	"fmt"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// Actions dispatched by dispatchAction. The systemd arms interpolate the
// action directly as the systemctl verb, so these double as the verb string
// for KindSystemdUser / KindSystemdSystem.
const (
	actionStart     = "start"
	actionStop      = "stop"
	actionUninstall = "uninstall"
)

// resolveInstalledTarget is the shared preamble of Start and Stop: detect the
// installed kind, sanitize the instance, and fail when nothing is installed.
// Uninstall does not use it because KindNone is a no-op there rather than an
// error.
func resolveInstalledTarget(h *host.Host, instance string) (Kind, string, string, error) {
	kind, san, base, err := resolveAutostartTarget(h, instance)
	if err != nil {
		return KindNone, "", "", err
	}
	if kind == KindNone {
		return KindNone, "", "", fmt.Errorf("autostart is not installed for %s", instance)
	}
	return kind, san, base, nil
}

// dispatchAction runs the per-kind command for action (start, stop, or
// uninstall) against a detected autostart unit. It is the single dispatch
// point that replaces the copy-pasted `switch kind` blocks previously in
// Uninstall, Start, and Stop — adding a new Kind means editing this function
// (plus Detect/Install), not three near-identical switches.
//
// base is the systemd unit basename (without .service); san is the
// sanitized instance token used to derive launchd labels and Windows task
// names. Callers handle the KindNone precondition themselves (see
// resolveInstalledTarget and Uninstall).
func dispatchAction(h *host.Host, kind Kind, san, base, action string) error {
	switch kind {
	case KindSystemdUser:
		if action == actionUninstall {
			_, err := h.Run(systemdDisableUserScript(base))
			return err
		}
		_, err := h.Run("systemctl --user " + action + " " + base + ".service")
		return err

	case KindSystemdSystem:
		if action == actionUninstall {
			_, err := h.Run(systemdDisableSystemScript(base))
			return err
		}
		script := sudoPrelude() + fmt.Sprintf(`
$SUDO systemctl %s %s.service
`, action, base)
		_, err := h.Run(script)
		return err

	case KindLaunchd:
		return dispatchLaunchdAction(h, san, action)

	case KindWindowsTask:
		return dispatchWindowsTaskAction(h, san, action)

	default:
		return fmt.Errorf("unknown autostart kind %q", kind)
	}
}

// dispatchLaunchdAction handles the three launchd verbs. Start bootstraps
// the LaunchAgent (resolving the remote home first), Stop boots the job out
// of the gui/user domains without touching the plist, and uninstall boots it
// out and removes the plist via LaunchdBootoutScript.
func dispatchLaunchdAction(h *host.Host, san, action string) error {
	label := LaunchdLabel(san)
	switch action {
	case actionStart:
		home, err := remoteHome(h)
		if err != nil {
			return err
		}
		plistPath := home + "/Library/LaunchAgents/" + label + ".plist"
		cmd := launchdActivateScript(hostshell.PosixSingleQuote(label), hostshell.PosixSingleQuote(plistPath), label+".plist", false)
		_, err = h.Run(cmd)
		return err
	case actionStop:
		cmd := launchdStopScript(hostshell.PosixSingleQuote(label))
		_, err := h.Run(cmd)
		return err
	default:
		cmd := LaunchdBootoutScript(hostshell.PosixSingleQuote(label), label+".plist")
		_, err := h.Run(cmd)
		return err
	}
}

// dispatchWindowsTaskAction maps the three actions onto their scheduled-task
// PowerShell cmdlets. Stop and uninstall are best-effort (SilentlyContinue)
// so a missing task does not fail the caller.
func dispatchWindowsTaskAction(h *host.Host, san, action string) error {
	name := WindowsTaskName(san)
	var ps string
	switch action {
	case actionStart:
		ps = fmt.Sprintf(`Start-ScheduledTask -TaskName %s`, hostshell.PowerShellSingleQuote(name))
	case actionStop:
		ps = fmt.Sprintf(`Stop-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue`, hostshell.PowerShellSingleQuote(name))
	default:
		ps = fmt.Sprintf(`Unregister-ScheduledTask -TaskName %s -Confirm:$false -ErrorAction SilentlyContinue`, hostshell.PowerShellSingleQuote(name))
	}
	_, err := h.RunShell(ps)
	return err
}
