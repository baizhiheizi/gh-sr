package autostart

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
)

// Kind describes how autostart is installed for an instance.
type Kind string

const (
	KindNone          Kind = ""
	KindSystemdUser   Kind = "systemd-user"
	KindSystemdSystem Kind = "systemd-system"
	KindLaunchd       Kind = "launchd"
	KindWindowsTask   Kind = "windows-task"
)

// ServiceBasename returns the systemd unit basename (without .service) for ghsr-runner-<sanitized>.
func ServiceBasename(instanceSanitized string) string {
	return "ghsr-runner-" + instanceSanitized
}

func remoteHome(h *host.Host) (string, error) {
	if h.OS == "windows" {
		out, err := h.RunShell(`Write-Output $env:USERPROFILE`)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}
	out, err := h.Run(`printf %s "$HOME"`)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func absRunnerDir(h *host.Host, home, instance string) string {
	home = strings.TrimRight(home, `/\`)
	if h.OS == "windows" {
		return home + `\` + `.gh-sr` + `\` + `runners` + `\` + instance
	}
	return home + "/.gh-sr/runners/" + instance
}

// writeRemoteBytes writes data to a remote path using base64 (POSIX) or PowerShell (Windows).
func writeRemoteBytes(h *host.Host, remotePath string, data []byte) error {
	if h.OS == "windows" {
		b64 := base64.StdEncoding.EncodeToString(data)
		ps := fmt.Sprintf(
			"$p = %s; $d = [Convert]::FromBase64String(%s); $dir = Split-Path -Parent $p; New-Item -ItemType Directory -Force -Path $dir | Out-Null; [IO.File]::WriteAllBytes($p, $d)",
			powerShellSingleQuoted(remotePath),
			powerShellSingleQuoted(b64),
		)
		_, err := h.RunShell(ps)
		return err
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	qpath := posixSingleQuote(remotePath)
	cmd := fmt.Sprintf(`set -e; d=$(dirname %s); mkdir -p "$d"; printf '%%s' %s | base64 -d > %s`,
		qpath, posixSingleQuote(b64), qpath)
	_, err := h.Run(cmd)
	return err
}

func powerShellSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Detect reports whether an autostart unit is installed for this instance.
func Detect(h *host.Host, instance string) (Kind, error) {
	san, err := SanitizeInstance(instance)
	if err != nil {
		return KindNone, err
	}
	base := ServiceBasename(san)

	switch h.OS {
	case "linux":
		userUnit := fmt.Sprintf(`test -f "$HOME/.config/systemd/user/%s.service" && echo user || true`, base)
		out, err := h.Run(userUnit)
		if err != nil {
			return KindNone, err
		}
		if strings.TrimSpace(out) == "user" {
			return KindSystemdUser, nil
		}
		sysCheck := fmt.Sprintf(`test -f /etc/systemd/system/%s.service && echo system || true`, base)
		out2, err := h.Run(sysCheck)
		if err != nil {
			return KindNone, err
		}
		if strings.TrimSpace(out2) == "system" {
			return KindSystemdSystem, nil
		}
		return KindNone, nil

	case "darwin":
		label := LaunchdLabel(san)
		check := fmt.Sprintf(`test -f "$HOME/Library/LaunchAgents/%s" && echo yes || true`, label+".plist")
		out, err := h.Run(check)
		if err != nil {
			return KindNone, err
		}
		if strings.TrimSpace(out) == "yes" {
			return KindLaunchd, nil
		}
		return KindNone, nil

	case "windows":
		name := WindowsTaskName(san)
		ps := fmt.Sprintf(`if (Get-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue) { 'yes' } else { 'no' }`, powerShellSingleQuoted(name))
		out, err := h.RunShell(ps)
		if err != nil {
			return KindNone, err
		}
		if strings.TrimSpace(out) == "yes" {
			return KindWindowsTask, nil
		}
		return KindNone, nil

	default:
		return KindNone, fmt.Errorf("unsupported host OS %q", h.OS)
	}
}

// InstallOpts configures Install.
type InstallOpts struct {
	// System (Linux only) install unit under /etc/systemd/system with User=/Group=.
	System bool
}

// Install writes platform autostart definitions and enables them.
func Install(h *host.Host, instance string, opts InstallOpts) error {
	san, err := SanitizeInstance(instance)
	if err != nil {
		return err
	}
	home, err := remoteHome(h)
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	runnerDir := absRunnerDir(h, home, instance)

	switch h.OS {
	case "linux":
		if opts.System {
			return installSystemdSystem(h, instance, san, runnerDir)
		}
		return installSystemdUser(h, instance, san, runnerDir)
	case "darwin":
		return installLaunchd(h, instance, san, runnerDir, home)
	case "windows":
		return installWindowsTask(h, instance, san, runnerDir)
	default:
		return fmt.Errorf("autostart install not supported on OS %q", h.OS)
	}
}

func installSystemdUser(h *host.Host, instance, san, absRunnerDir string) error {
	unit := SystemdUserUnit(san, absRunnerDir)
	base := ServiceBasename(san)
	home, err := remoteHome(h)
	if err != nil {
		return err
	}
	fullUnitPath := home + "/.config/systemd/user/" + base + ".service"

	if err := writeRemoteBytes(h, fullUnitPath, []byte(unit)); err != nil {
		return fmt.Errorf("writing systemd user unit: %w", err)
	}

	cmd := fmt.Sprintf(`set -e
systemctl --user daemon-reload
systemctl --user enable %s.service
systemctl --user restart %s.service || systemctl --user start %s.service
`, base, base, base)
	if _, err := h.Run(cmd); err != nil {
		return fmt.Errorf("enabling systemd user unit: %w", err)
	}
	_ = instance
	return nil
}

func installSystemdSystem(h *host.Host, instance, san, absRunnerDir string) error {
	userOut, err := h.Run(`id -un`)
	if err != nil {
		return fmt.Errorf("id -un: %w", err)
	}
	groupOut, err := h.Run(`id -gn`)
	if err != nil {
		return fmt.Errorf("id -gn: %w", err)
	}
	user := strings.TrimSpace(userOut)
	group := strings.TrimSpace(groupOut)
	unit := SystemdSystemUnit(san, absRunnerDir, user, group)
	base := ServiceBasename(san)
	sysPath := "/etc/systemd/system/" + base + ".service"

	home, err := remoteHome(h)
	if err != nil {
		return err
	}
	tmpPath := home + "/.gh-sr/" + base + ".service.tmp"
	if err := writeRemoteBytes(h, tmpPath, []byte(unit)); err != nil {
		return fmt.Errorf("staging systemd system unit: %w", err)
	}

	script := linuxElevatePrelude + fmt.Sprintf(`
set -e
$SUDO mv %s %s
$SUDO systemctl daemon-reload
$SUDO systemctl enable %s.service
$SUDO systemctl restart %s.service || $SUDO systemctl start %s.service
`,
		posixSingleQuote(tmpPath),
		posixSingleQuote(sysPath),
		base, base, base)

	if _, err := h.Run(script); err != nil {
		return fmt.Errorf("installing system systemd unit: %w", err)
	}
	_ = instance
	return nil
}

func installLaunchd(h *host.Host, instance, san, absRunnerDir, home string) error {
	plist := LaunchdPlist(san, absRunnerDir)
	label := LaunchdLabel(san)
	plistName := label + ".plist"
	plistPath := home + "/Library/LaunchAgents/" + plistName

	if err := writeRemoteBytes(h, plistPath, []byte(plist)); err != nil {
		return fmt.Errorf("writing LaunchAgent plist: %w", err)
	}

	qplist := posixSingleQuote(plistPath)
	qlabel := posixSingleQuote(label)
	cmd := "mkdir -p \"$HOME/Library/LaunchAgents\"\n" +
		launchdActivateScript(qlabel, qplist, plistName, true)

	if _, err := h.Run(cmd); err != nil {
		return fmt.Errorf("loading LaunchAgent: %w", err)
	}
	_ = instance
	return nil
}

func installWindowsTask(h *host.Host, instance, san, absRunnerDir string) error {
	taskName := WindowsTaskName(san)
	// run.cmd in runner dir; WorkingDirectory set on the action.
	ps := fmt.Sprintf(`
$tn = %s
$rd = %s
Unregister-ScheduledTask -TaskName $tn -Confirm:$false -ErrorAction SilentlyContinue
	$act = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-WindowStyle Hidden -Command ".\run.cmd"' -WorkingDirectory $rd
$tr = New-ScheduledTaskTrigger -AtLogOn
$st = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
Register-ScheduledTask -TaskName $tn -Action $act -Trigger $tr -Settings $st -RunLevel Limited -Force | Out-Null
Start-ScheduledTask -TaskName $tn
`,
		powerShellSingleQuoted(taskName),
		powerShellSingleQuoted(absRunnerDir),
	)
	if _, err := h.RunShell(strings.TrimSpace(ps)); err != nil {
		return fmt.Errorf("registering scheduled task: %w", err)
	}
	_ = instance
	_ = san
	return nil
}

// Uninstall removes autostart and stops the supervised process where applicable.
func Uninstall(h *host.Host, instance string) error {
	kind, err := Detect(h, instance)
	if err != nil {
		return err
	}
	if kind == KindNone {
		return nil
	}
	san, err := SanitizeInstance(instance)
	if err != nil {
		return err
	}
	base := ServiceBasename(san)

	switch kind {
	case KindSystemdUser:
		cmd := fmt.Sprintf(`set -e
systemctl --user disable --now %s.service 2>/dev/null || true
rm -f "$HOME/.config/systemd/user/%s.service"
systemctl --user daemon-reload
`, base, base)
		_, err := h.Run(cmd)
		return err

	case KindSystemdSystem:
		script := linuxElevatePrelude + fmt.Sprintf(`
set -e
$SUDO systemctl disable --now %s.service 2>/dev/null || true
$SUDO rm -f /etc/systemd/system/%s.service
$SUDO systemctl daemon-reload
`, base, base)
		_, err := h.Run(script)
		return err

	case KindLaunchd:
		label := LaunchdLabel(san)
		cmd := launchdBootoutScript(posixSingleQuote(label), label+".plist")
		_, err := h.Run(cmd)
		return err

	case KindWindowsTask:
		name := WindowsTaskName(san)
		ps := fmt.Sprintf(
			`Unregister-ScheduledTask -TaskName %s -Confirm:$false -ErrorAction SilentlyContinue`,
			powerShellSingleQuoted(name),
		)
		_, err := h.RunShell(ps)
		return err

	default:
		return nil
	}
}

// Start launches the autostart-backed runner (systemd / launchd / scheduled task).
func Start(h *host.Host, instance string) error {
	kind, err := Detect(h, instance)
	if err != nil {
		return err
	}
	if kind == KindNone {
		return fmt.Errorf("autostart is not installed for %s", instance)
	}
	san, err := SanitizeInstance(instance)
	if err != nil {
		return err
	}
	base := ServiceBasename(san)

	switch kind {
	case KindSystemdUser:
		_, err := h.Run("systemctl --user start " + base + ".service")
		return err
	case KindSystemdSystem:
		script := linuxElevatePrelude + fmt.Sprintf(`
$SUDO systemctl start %s.service
`, base)
		_, err := h.Run(script)
		return err
	case KindLaunchd:
		label := LaunchdLabel(san)
		home, herr := remoteHome(h)
		if herr != nil {
			return herr
		}
		plistPath := home + "/Library/LaunchAgents/" + label + ".plist"
		cmd := launchdActivateScript(posixSingleQuote(label), posixSingleQuote(plistPath), label+".plist", false)
		_, err := h.Run(cmd)
		return err
	case KindWindowsTask:
		name := WindowsTaskName(san)
		ps := fmt.Sprintf(`Start-ScheduledTask -TaskName %s`, powerShellSingleQuoted(name))
		_, err := h.RunShell(ps)
		return err
	default:
		return fmt.Errorf("unknown autostart kind %q", kind)
	}
}

// Stop stops the autostart-backed runner without removing the unit.
func Stop(h *host.Host, instance string) error {
	kind, err := Detect(h, instance)
	if err != nil {
		return err
	}
	if kind == KindNone {
		return fmt.Errorf("autostart is not installed for %s", instance)
	}
	san, err := SanitizeInstance(instance)
	if err != nil {
		return err
	}
	base := ServiceBasename(san)

	switch kind {
	case KindSystemdUser:
		_, err := h.Run("systemctl --user stop " + base + ".service")
		return err
	case KindSystemdSystem:
		script := linuxElevatePrelude + fmt.Sprintf(`
$SUDO systemctl stop %s.service
`, base)
		_, err := h.Run(script)
		return err
	case KindLaunchd:
		label := LaunchdLabel(san)
		cmd := fmt.Sprintf(`UID=$(id -u); LABEL=%s; for _DOMAIN in "gui/$UID" "user/$UID"; do launchctl bootout "$_DOMAIN/$LABEL" 2>/dev/null || true; done`,
			posixSingleQuote(label))
		_, err := h.Run(cmd)
		return err
	case KindWindowsTask:
		name := WindowsTaskName(san)
		ps := fmt.Sprintf(`Stop-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue`, powerShellSingleQuoted(name))
		_, err := h.RunShell(ps)
		return err
	default:
		return fmt.Errorf("unknown autostart kind %q", kind)
	}
}

// StatusRow is one line of `gh sr service status` output.
type StatusRow struct {
	Instance string
	Host     string
	Mode     string // native / docker
	Kind     Kind
	Detail   string // active/inactive, etc.
}

// Status describes autostart and service state for a runner instance (native only).
func Status(h *host.Host, hostName, instance, mode string) (StatusRow, error) {
	row := StatusRow{Instance: instance, Host: hostName, Mode: mode}
	if mode != "native" {
		row.Kind = KindNone
		row.Detail = "docker: containers use --restart unless-stopped; gh sr down stops the container so it will not auto-start on boot until gh sr up"
		return row, nil
	}

	kind, err := Detect(h, instance)
	if err != nil {
		return row, err
	}
	row.Kind = kind
	if kind == KindNone {
		row.Detail = "autostart not installed"
		return row, nil
	}

	san, err := SanitizeInstance(instance)
	if err != nil {
		return row, err
	}
	base := ServiceBasename(san)

	switch kind {
	case KindSystemdUser:
		out, err := h.Run(fmt.Sprintf(`systemctl --user is-active %s.service 2>/dev/null || echo inactive`, base))
		active := strings.TrimSpace(out)
		if err != nil {
			row.Detail = "installed (user): " + active + " (check failed: " + err.Error() + ")"
			return row, nil
		}
		row.Detail = "installed (user): " + active
		return row, nil

	case KindSystemdSystem:
		script := linuxElevatePrelude + fmt.Sprintf(`
$SUDO systemctl is-active %s.service 2>/dev/null || echo inactive
`, base)
		out, err := h.Run(script)
		active := strings.TrimSpace(out)
		if err != nil {
			row.Detail = "installed (system): " + active + " (check failed: " + err.Error() + ")"
			return row, nil
		}
		row.Detail = "installed (system): " + active
		return row, nil

	case KindLaunchd:
		label := LaunchdLabel(san)
		cmd := launchdPrintScript(posixSingleQuote(label)) + " | head -n 5"
		out, err := h.Run(cmd)
		if err != nil {
			row.Detail = "installed (launchd): error " + err.Error()
			return row, nil
		}
		row.Detail = "installed (launchd): " + strings.TrimSpace(strings.ReplaceAll(out, "\n", " "))
		return row, nil

	case KindWindowsTask:
		name := WindowsTaskName(san)
		ps := fmt.Sprintf(`(Get-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue | Select-Object -ExpandProperty State)`, powerShellSingleQuoted(name))
		out, err := h.RunShell(ps)
		if err != nil {
			row.Detail = "installed (task): error " + err.Error()
			return row, nil
		}
		row.Detail = "installed (task): " + strings.TrimSpace(out)
		return row, nil
	}

	return row, nil
}
