package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// NativeRunnerConfigPresent reports whether the remote instance directory contains
// a fully-configured runner: the directory exists, run.sh/run.cmd is present, and .runner exists.
// This prevents EnsureSetup from skipping setup when only the .runner file exists but
// the runner binaries (run.sh) or directory are missing.
func NativeRunnerConfigPresent(h *host.Host, instanceName string) (bool, error) {
	dir := h.RunnerDir(instanceName)
	if h.OS == "windows" {
		out, err := h.RunShell(fmt.Sprintf(
			"%s; if ((Test-Path $runnerDir -PathType Container) -and (Test-Path (Join-Path $runnerDir 'run.cmd')) -and (Test-Path (Join-Path $runnerDir '.runner'))) { Write-Output 'yes' } else { Write-Output 'no' }",
			windowsRunnerDirAssignment(h, instanceName),
		))
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(out) == "yes", nil
	}
	out, err := h.Run(fmt.Sprintf("test -d %s && test -f %s/run.sh && test -f %s/.runner && echo yes || echo no", dir, dir, dir))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}

// svcShPresent reports whether svc.sh is deployed in the runner directory.
func svcShPresent(h *host.Host, instanceName string) bool {
	dir := h.RunnerDir(instanceName)
	if h.OS == "windows" {
		return false
	}
	out, _ := h.Run(fmt.Sprintf("test -f %s/svc.sh && echo yes || echo no", dir))
	return strings.TrimSpace(out) == "yes"
}

// posixSingleQuote and writeRemoteBytes were moved to internal/hostshell.
// Call sites use hostshell.PosixSingleQuote and hostshell.WriteRemoteBytes directly.

func runnerTarballURL(version, osName, arch string) string {
	switch osName {
	case "darwin":
		return fmt.Sprintf(
			"https://github.com/actions/runner/releases/download/v%s/actions-runner-osx-%s-%s.tar.gz",
			version, arch, version,
		)
	case "linux":
		return fmt.Sprintf(
			"https://github.com/actions/runner/releases/download/v%s/actions-runner-linux-%s-%s.tar.gz",
			version, arch, version,
		)
	case "windows":
		a := arch
		if a == "amd64" {
			a = "x64"
		}
		return fmt.Sprintf(
			"https://github.com/actions/runner/releases/download/v%s/actions-runner-win-%s-%s.zip",
			version, a, version,
		)
	}
	return ""
}

func archForGitHub(arch string) string {
	switch arch {
	case "amd64":
		return "x64"
	case "arm64":
		return "arm64"
	}
	return arch
}

// powerShellSingleQuoted was moved to internal/hostshell.
// Call sites use hostshell.PowerShellSingleQuote directly.

// windowsRunnerDirAssignment returns the PowerShell snippet that assigns
// $runnerDir to the per-instance runner directory on Windows.
// The variable name is fixed: every Windows script in this package expects
// $runnerDir, so the helper has no second-variable form.
func windowsRunnerDirAssignment(h *host.Host, instanceName string) string {
	return "$runnerDir = " + h.RunnerDirPS(instanceName)
}

func windowsNativeInstallScript(h *host.Host, instanceName, version, url string) string {
	zipName := fmt.Sprintf("actions-runner-%s.zip", version)
	return strings.Join([]string{
		windowsRunnerDirAssignment(h, instanceName),
		"New-Item -ItemType Directory -Force -Path $runnerDir | Out-Null",
		fmt.Sprintf("$zip = Join-Path %s %s", h.TempDirPS(), hostshell.PowerShellSingleQuote(zipName)),
		"[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12",
		fmt.Sprintf("if (-not (Test-Path $zip)) { Invoke-WebRequest -Uri %s -OutFile $zip }", hostshell.PowerShellSingleQuote(url)),
		fmt.Sprintf("try { Expand-Archive -Path $zip -DestinationPath $runnerDir -Force } catch { Remove-Item -Force $zip -EA SilentlyContinue; Invoke-WebRequest -Uri %s -OutFile $zip; Expand-Archive -Path $zip -DestinationPath $runnerDir -Force }", hostshell.PowerShellSingleQuote(url)),
	}, "; ")
}

func windowsNativeConfigScript(h *host.Host, rc config.RunnerConfig, instanceName, regToken string, instanceIndex int) string {
	labels := strings.Join(rc.EffectiveLabelsForInstance(h.OS, h.Arch, instanceIndex), ",")
	cmd := fmt.Sprintf(
		"& .\\config.cmd --unattended --url %s --token %s --name %s --labels %s --work '_work' --replace",
		hostshell.PowerShellSingleQuote(rc.GitHubRegistrationURL()),
		hostshell.PowerShellSingleQuote(regToken),
		hostshell.PowerShellSingleQuote(instanceName),
		hostshell.PowerShellSingleQuote(labels),
	)
	if rc.Group != "" {
		cmd += fmt.Sprintf(" --runnergroup %s", hostshell.PowerShellSingleQuote(rc.Group))
	}
	if rc.Ephemeral {
		cmd += " --ephemeral"
	}
	return strings.Join([]string{
		windowsRunnerDirAssignment(h, instanceName),
		"Set-Location -Path $runnerDir",
		cmd,
	}, "; ")
}

// staleRegistrationMsg is the substring the Actions runner writes to its log when the
// server-side registration has been deleted (auto-pruned after inactivity).
const staleRegistrationMsg = "runner registration has been deleted from the server"

func windowsNativeStartScript(h *host.Host, instanceName string) string {
	// Win32-OpenSSH tears down the session job on disconnect, killing Start-Process children.
	// Win32_Process.Create starts outside that job so the listener survives after gh sr closes SSH.
	// CREATE_NO_WINDOW (0x08000000) prevents the console subsystem from creating a conhost window;
	// ShowWindow=0 is kept as belt-and-suspenders but CREATE_NO_WINDOW is the flag that actually
	// suppresses the visible cmd.exe console on modern Windows.
	parts := []string{
		windowsRunnerDirAssignment(h, instanceName) + "; ",
		`$pidFile = Join-Path $runnerDir '.runner_pid'; `,
		`$logFile = Join-Path $runnerDir 'runner.log'; `,
		`if (Test-Path $pidFile) { $existingPid = Get-Content $pidFile; try { Get-Process -Id $existingPid -EA Stop | Out-Null; Write-Host 'already running'; exit 0 } catch {} }; `,
		`$cmdArg = 'cd /d "' + $runnerDir + '" && run.cmd > "' + $logFile + '" 2>&1'; `,
		`$fullLine = 'cmd.exe /c ' + $cmdArg; `,
		`$si = New-CimInstance -CimClass (Get-CimClass -ClassName Win32_ProcessStartup) -Property @{CreateFlags=0x08000000; ShowWindow=0} -ClientOnly; `,
		`$cim = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $fullLine; CurrentDirectory = $runnerDir; ProcessStartupInformation = $si }; `,
		`if ($cim.ReturnValue -ne 0) { Write-Host ('Win32_Process.Create failed: ' + $cim.ReturnValue); exit 1 }; `,
		`$cim.ProcessId | Out-File -FilePath $pidFile -NoNewline; `,
		`Write-Host ('started PID ' + $cim.ProcessId)`,
	}
	return strings.Join(parts, "")
}

// windowsCheckStaleRegistration returns a PowerShell snippet that waits briefly for the
// runner process to either stay alive (healthy) or exit, then checks runner.log for the
// stale-registration message. It writes "stale" to stdout if detected, "ok" otherwise.
func windowsCheckStaleRegistration(h *host.Host, instanceName string) string {
	return fmt.Sprintf(
		"%s; $pidFile = Join-Path $runnerDir '.runner_pid'; "+
			"$logFile = Join-Path $runnerDir 'runner.log'; "+
			"Start-Sleep -Seconds 5; "+
			"$pid = Get-Content $pidFile -EA SilentlyContinue; "+
			"$alive = $false; if ($pid) { try { Get-Process -Id $pid -EA Stop | Out-Null; $alive = $true } catch {} }; "+
			"if ($alive) { Write-Host 'ok' } "+
			"elseif ((Test-Path $logFile) -and (Select-String -Path $logFile -Pattern %s -Quiet)) { Write-Host 'stale' } "+
			"else { Write-Host 'ok' }",
		windowsRunnerDirAssignment(h, instanceName),
		hostshell.PowerShellSingleQuote(staleRegistrationMsg),
	)
}

// windowsDeleteRunnerConfig removes the .runner file so setupNative will re-configure.
func windowsDeleteRunnerConfig(h *host.Host, instanceName string) string {
	return fmt.Sprintf(
		"%s; Remove-Item -Force (Join-Path $runnerDir '.runner') -EA SilentlyContinue; "+
			"Remove-Item -Force (Join-Path $runnerDir '.credentials') -EA SilentlyContinue; "+
			"Remove-Item -Force (Join-Path $runnerDir '.credentials_rsaparams') -EA SilentlyContinue",
		windowsRunnerDirAssignment(h, instanceName),
	)
}

func (m *Manager) setupNative(h *host.Host, rc config.RunnerConfig) error {
	version, err := m.GitHub.GetLatestRunnerVersion()
	if err != nil {
		return err
	}

	ghArch := archForGitHub(h.Arch)
	url := runnerTarballURL(version, h.OS, ghArch)
	if url == "" {
		return fmt.Errorf("unsupported OS/arch: %s/%s", h.OS, h.Arch)
	}

	for i, name := range rc.InstanceNames() {
		dir := h.RunnerDir(name)

		installed, _ := NativeRunnerConfigPresent(h, name)
		if installed {
			fmt.Fprintf(m.out(), "  %s: already installed, skipping\n", name)
			continue
		}

		fmt.Fprintf(m.out(), "  %s: installing runner v%s...\n", name, version)

		if h.OS == "linux" {
			installDepsCmd := sudoPrelude() + `
				if ! command -v curl >/dev/null 2>&1 || ! command -v tar >/dev/null 2>&1; then
					if command -v apt-get >/dev/null 2>&1; then $SUDO apt-get update && $SUDO apt-get install -y curl tar;
					elif command -v yum >/dev/null 2>&1; then $SUDO yum install -y curl tar;
					elif command -v apk >/dev/null 2>&1; then $SUDO apk add curl tar;
					fi
				fi
			`
			if _, err := h.Run(installDepsCmd); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to ensure curl/tar are installed: %v\n", name, err)
			}
		}

		if h.OS == "windows" {
			fmt.Fprintf(m.out(), "  %s: downloading runner package...\n", name)
			if _, err := h.RunShell(windowsNativeInstallScript(h, name, version, url)); err != nil {
				return fmt.Errorf("installing runner on Windows: %w", err)
			}
		} else {
			tarball := fmt.Sprintf("%s/ghsr-runner-%s-%s.tar.gz", h.TempDir(), name, version)
			fmt.Fprintf(m.out(), "  %s: downloading runner v%s...\n", name, version)
			if _, err := h.Run(fmt.Sprintf("mkdir -p %s", dir)); err != nil {
				return fmt.Errorf("creating runner directory: %w", err)
			}
			cmds := fmt.Sprintf(
				"cd %s && tarball=%s && rm -f \"$tarball\" && "+
					"curl -fSL --retry 3 --retry-delay 2 -o \"$tarball\" '%s'",
				dir, tarball, url,
			)
			if _, err := h.Run(cmds); err != nil {
				return fmt.Errorf("downloading runner: %w", err)
			}
			fmt.Fprintf(m.out(), "  %s: extracting runner...\n", name)
			cmds = fmt.Sprintf("cd %s && tarball=%s && tar xzf \"$tarball\" && rm -f \"$tarball\"", dir, tarball)
			if _, err := h.Run(cmds); err != nil {
				return fmt.Errorf("extracting runner: %w", err)
			}
		}

		hostshell.WriteRemoteBytes(h, dir+"/.runner-version", []byte(version))

		if h.OS == "linux" {
			fmt.Fprintf(m.out(), "  %s: installing runner dependencies...\n", name)
			depsCmd := fmt.Sprintf(
				"cd %s && %s && $SUDO ./bin/installdependencies.sh",
				dir, strings.TrimSpace(sudoPrelude()),
			)
			if _, err := h.Run(depsCmd); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to install runner dependencies: %v\n", name, err)
			}
		}

		regToken, err := m.GitHub.GetRegistrationTokenScoped(rc.Scope(), rc.ScopeTarget())
		if err != nil {
			return err
		}

		labels := strings.Join(rc.EffectiveLabelsForInstance(h.OS, h.Arch, i), ",")

		fmt.Fprintf(m.out(), "  %s: registering runner with GitHub...\n", name)
		if h.OS == "windows" {
			if _, err := h.RunShell(windowsNativeConfigScript(h, rc, name, regToken, i)); err != nil {
				return fmt.Errorf("configuring runner on Windows: %w", err)
			}
		} else {
			configURL := rc.GitHubRegistrationURL()
			configCmd := fmt.Sprintf(
				"cd %s && ./config.sh --unattended --url '%s' --token '%s' --name '%s' --labels '%s' --work '_work' --replace",
				dir, configURL, regToken, name, labels,
			)
			if rc.Group != "" {
				configCmd += fmt.Sprintf(" --runnergroup '%s'", rc.Group)
			}
			if rc.Ephemeral {
				configCmd += " --ephemeral"
			}
			if _, err := h.Run(configCmd); err != nil {
				return fmt.Errorf("configuring runner: %w", err)
			}
		}

		fmt.Fprintf(m.out(), "  %s: configured\n", name)

		// Deploy svc.sh and runsvc.sh for systemd-based service management on Linux.
		if h.OS == "linux" {
			// Deploy svc.sh to the runner directory
			if err := hostshell.WriteRemoteBytes(h, dir+"/svc.sh", []byte(SVCShContent)); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to deploy svc.sh: %v\n", name, err)
			} else {
				h.Run(fmt.Sprintf("chmod +x %s/svc.sh", dir))
			}

			// Deploy the systemd service template
			if err := hostshell.WriteRemoteBytes(h, dir+"/bin/actions.runner.service.template", []byte(ServiceTemplateContent)); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to deploy service template: %v\n", name, err)
			}

			// Copy runsvc.sh from the runner package to the runner root (runsvc.sh handles signals gracefully)
			if _, err := h.Run(fmt.Sprintf("cp %s/bin/runsvc.sh %s/runsvc.sh && chmod +x %s/runsvc.sh", dir, dir, dir)); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to copy runsvc.sh: %v\n", name, err)
			}

			// Install svc.sh as a systemd service (requires root)
			installSvcCmd := fmt.Sprintf("cd %s && %s\n$SUDO ./svc.sh install", dir, strings.TrimSpace(sudoPrelude()))
			if _, err := h.Run(installSvcCmd); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to install svc.sh service: %v\n", name, err)
			}
		}
	}

	// Note: profile: agentic always runs in container mode (see EffectiveRunnerMode),
	// so setupNative is only reached for non-agentic native runners. The former
	// native-agentic host mutations (dnsmasq, sudoers, /opt/hostedtoolcache bind mount,
	// gh-aw install, image pre-pull) lived inside the runner image instead and have
	// been removed from the host path.
	return nil
}

func (m *Manager) startNative(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	return m.startNativeOnce(h, rc, instanceName, true)
}

// startNativeOnce starts the runner and optionally detects a stale registration.
// When retryOnStale is true and the runner exits immediately with the stale-registration
// message, it clears local credentials, re-runs setupNative, and retries once.
func (m *Manager) startNativeOnce(h *host.Host, rc config.RunnerConfig, instanceName string, retryOnStale bool) error {
	dir := h.RunnerDir(instanceName)

	ok, err := NativeRunnerConfigPresent(h, instanceName)
	if err != nil {
		return fmt.Errorf("checking runner install at %s: %w", dir, err)
	}
	if !ok {
		fmt.Fprintf(m.out(), "  %s: not installed, running setup (this takes a few minutes)...\n", instanceName)
		if setupErr := m.setupNative(h, rc); setupErr != nil {
			return fmt.Errorf("auto-setup for %s: %w", instanceName, setupErr)
		}
	}

	if h.OS == "windows" {
		out, err := h.RunShell(windowsNativeStartScript(h, instanceName))
		if err != nil {
			return err
		}
		fmt.Fprintf(m.out(), "  %s: %s\n", instanceName, strings.TrimSpace(out))

		if retryOnStale {
			fmt.Fprintf(m.out(), "  %s: waiting for runner to initialize...\n", instanceName)
			checkOut, _ := h.RunShell(windowsCheckStaleRegistration(h, instanceName))
			if strings.TrimSpace(checkOut) == "stale" {
				return m.handleStaleRegistration(h, rc, instanceName)
			}
		}
		return nil
	}

	// Unix (Linux / macOS)
	cmd := fmt.Sprintf(
		`cd %s; pid_file=".runner_pid"; `+
			`if [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then echo "already running"; exit 0; fi; `+
			`nohup ./run.sh > runner.log 2>&1 & { echo $!; } > "$pid_file"; echo "started PID $!"`,
		dir,
	)
	out, err := h.Run(cmd)
	if err != nil {
		return err
	}
	fmt.Fprintf(m.out(), "  %s: %s\n", instanceName, strings.TrimSpace(out))

	if retryOnStale {
		fmt.Fprintf(m.out(), "  %s: waiting for runner to initialize...\n", instanceName)
		checkCmd := fmt.Sprintf(
			`sleep 5 && cd %s && pid=$(cat .runner_pid 2>/dev/null) && `+
				`if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then echo ok; `+
				`elif grep -q %q runner.log 2>/dev/null; then echo stale; `+
				`else echo ok; fi`,
			dir, staleRegistrationMsg,
		)
		checkOut, _ := h.Run(checkCmd)
		if strings.TrimSpace(checkOut) == "stale" {
			return m.handleStaleRegistration(h, rc, instanceName)
		}
	}
	return nil
}

func (m *Manager) handleStaleRegistration(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	fmt.Fprintf(m.out(), "  %s: registration expired on GitHub, re-configuring...\n", instanceName)
	dir := h.RunnerDir(instanceName)

	if h.OS == "windows" {
		h.RunShell(windowsDeleteRunnerConfig(h, instanceName))
	} else {
		h.Run(fmt.Sprintf("rm -f %s/.runner %s/.credentials %s/.credentials_rsaparams", dir, dir, dir))
	}

	if err := m.setupNative(h, rc); err != nil {
		return fmt.Errorf("re-configuring after stale registration: %w", err)
	}

	return m.startNativeOnce(h, rc, instanceName, false)
}

func (m *Manager) stopNative(h *host.Host, instanceName string) error {
	dir := h.RunnerDir(instanceName)

	if h.OS == "windows" {
		cmd := fmt.Sprintf(
			"%s; $pidFile = Join-Path $runnerDir '.runner_pid'; "+
				"if (-not (Test-Path $pidFile)) { Write-Host 'not running'; exit 0 }; "+
				"$p = (Get-Content $pidFile | Select-Object -First 1).Trim(); "+
				"$null = & taskkill.exe /PID $p /T /F 2>&1; "+
				"if ($LASTEXITCODE -eq 0) { Write-Host 'stopped' } else { Write-Host 'not running' }; "+
				"Remove-Item $pidFile -Force -EA SilentlyContinue",
			windowsRunnerDirAssignment(h, instanceName),
		)
		out, err := h.RunShell(cmd)
		if err != nil {
			return err
		}
		fmt.Fprintf(m.out(), "  %s: %s\n", instanceName, strings.TrimSpace(out))
		return nil
	}

	cmd := fmt.Sprintf(
		`cd %s && pid_file=".runner_pid" && `+
			`if [ ! -f "$pid_file" ]; then echo "not running"; exit 0; fi && `+
			`pid=$(cat "$pid_file") && `+
			`if ! kill -0 "$pid" 2>/dev/null; then echo "not running"; rm -f "$pid_file"; exit 0; fi && `+
			`kill "$pid" && `+
			`for i in $(seq 1 10); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done && `+
			`kill -0 "$pid" 2>/dev/null && kill -9 "$pid" 2>/dev/null; `+
			`rm -f "$pid_file" && echo "stopped"`,
		dir,
	)
	out, err := h.Run(cmd)
	if err != nil {
		return err
	}
	fmt.Fprintf(m.out(), "  %s: %s\n", instanceName, strings.TrimSpace(out))
	return nil
}

func (m *Manager) removeNative(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	dir := h.RunnerDir(instanceName)

	// Uninstall svc.sh service if present (before removing files)
	if h.OS == "linux" && svcShPresent(h, instanceName) {
		uninstallSvcCmd := fmt.Sprintf("cd %s && %s\n$SUDO ./svc.sh uninstall 2>/dev/null || true", dir, strings.TrimSpace(sudoPrelude()))
		h.Run(uninstallSvcCmd) // Ignore errors - we're removing anyway
	}

	_ = m.stopNative(h, instanceName)

	removeToken, err := m.GitHub.GetRemovalTokenScoped(rc.Scope(), rc.ScopeTarget())
	if err != nil {
		fmt.Fprintf(m.out(), "  %s: warning: could not get removal token: %v\n", instanceName, err)
	} else {
		if h.OS == "windows" {
			cmd := fmt.Sprintf(
				"%s; Set-Location -Path $runnerDir; & .\\config.cmd remove --token %s",
				windowsRunnerDirAssignment(h, instanceName),
				hostshell.PowerShellSingleQuote(removeToken),
			)
			h.RunShell(cmd)
		} else {
			cmd := fmt.Sprintf("cd %s && ./config.sh remove --token '%s'", dir, removeToken)
			h.Run(cmd)
		}
		fmt.Fprintf(m.out(), "  %s: deregistered\n", instanceName)
	}

	if h.OS == "windows" {
		h.RunShell(fmt.Sprintf("%s; Remove-Item -Recurse -Force $runnerDir", windowsRunnerDirAssignment(h, instanceName)))
	} else {
		h.Run(fmt.Sprintf("rm -rf %s", dir))
	}

	fmt.Fprintf(m.out(), "  %s: removed\n", instanceName)
	return nil
}

func (m *Manager) statusNative(h *host.Host, instanceName string) string {
	dir := h.RunnerDir(instanceName)

	// For svc.sh-managed runners on Linux: read the service name from the .service
	// marker file written by "svc.sh install" and query systemctl directly.
	if h.OS == "linux" && svcShPresent(h, instanceName) {
		out, err := h.Run(fmt.Sprintf(
			`svc_file="%s/.service"; `+
				`if [ -f "$svc_file" ]; then `+
				`svc=$(cat "$svc_file"); `+
				`systemctl is-active "$svc" 2>/dev/null || echo inactive; `+
				`fi`,
			dir,
		))
		if err == nil {
			state := strings.TrimSpace(out)
			if state == "active" {
				return "running"
			}
			if state != "" {
				return "stopped"
			}
		}
	}

	if kind, err := autostart.Detect(h, instanceName); err == nil && kind != autostart.KindNone {
		active, err := autostart.IsServiceActive(h, instanceName, kind)
		if err == nil && active {
			return "running"
		}
		// Autostart may be installed but the runner was started directly (e.g. macOS
		// with no GUI session for launchd). Fall through to the PID file check.
	}

	if h.OS == "windows" {
		cmd := fmt.Sprintf(
			"%s; if (-not (Test-Path (Join-Path $runnerDir '.runner'))) { Write-Host 'not installed'; exit 0 }; "+
				"$pidFile = Join-Path $runnerDir '.runner_pid'; "+
				"if (-not (Test-Path $pidFile)) { Write-Host 'stopped'; exit 0 }; "+
				"$p = Get-Content $pidFile; "+
				"try { Get-Process -Id $p -EA Stop | Out-Null; Write-Host 'running' } catch { Write-Host 'stopped' }",
			windowsRunnerDirAssignment(h, instanceName),
		)
		out, _ := h.RunShell(cmd)
		return strings.TrimSpace(out)
	}

	cmd := fmt.Sprintf(
		`d="%s" && `+
			`if [ ! -f "$d/.runner" ]; then echo "not installed"; exit 0; fi && `+
			`pid_file="$d/.runner_pid" && `+
			`if [ ! -f "$pid_file" ]; then echo "stopped"; exit 0; fi && `+
			`pid=$(cat "$pid_file") && `+
			`if kill -0 "$pid" 2>/dev/null; then echo "running"; else echo "stopped"; fi`,
		dir,
	)
	out, err := h.Run(cmd)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

func nativeRunnerVersion(h *host.Host, instanceName string) string {
	dir := h.RunnerDir(instanceName)
	if h.OS == "windows" {
		ps := fmt.Sprintf(`if (Test-Path (Join-Path %s '.runner-version')) { Get-Content (Join-Path %s '.runner-version') -EA Stop }`, h.RunnerDirPS(instanceName), h.RunnerDirPS(instanceName))
		out, err := h.RunShell(ps)
		if err != nil {
			return "-"
		}
		v := strings.TrimSpace(out)
		if v == "" {
			return "-"
		}
		return v
	}
	out, err := h.Run(fmt.Sprintf("cat %s/.runner-version 2>/dev/null", dir))
	if err != nil || strings.TrimSpace(out) == "" {
		return "-"
	}
	return strings.TrimSpace(out)
}

func (m *Manager) logsNative(h *host.Host, instanceName string) (string, error) {
	dir := h.RunnerDir(instanceName)

	if h.OS == "windows" {
		cmd := fmt.Sprintf(
			"%s; $logFile = Join-Path $runnerDir 'runner.log'; "+
				"$diagDir = Join-Path $runnerDir '_diag'; "+
				"$tailMain = { if (Test-Path $logFile) { $lines = @(Get-Content -LiteralPath $logFile -Tail 50 -EA SilentlyContinue); "+
				"$j = [string]::Join([Environment]::NewLine, $lines); "+
				"if ($lines.Count -gt 0 -and ($j -match '\\S')) { $j } else { $null } } else { $null } }; "+
				"$main = & $tailMain; "+
				"if ($null -ne $main) { Write-Output $main } "+
				"elseif (Test-Path $diagDir) { "+
				"$latest = Get-ChildItem -Path $diagDir -Filter *.log -File -EA SilentlyContinue | Sort-Object LastWriteTime -Descending | Select-Object -First 1; "+
				"if ($latest) { Write-Output ('--- _diag/' + $latest.Name + ' (last 50 lines) ---'); Get-Content -LiteralPath $latest.FullName -Tail 50 } "+
				"else { Write-Output 'no logs found' } } "+
				"else { Write-Output 'no logs found' }",
			windowsRunnerDirAssignment(h, instanceName),
		)
		return h.RunShell(cmd)
	}

	cmd := fmt.Sprintf("tail -50 %s/runner.log 2>/dev/null || echo 'no logs found'", dir)
	return h.Run(cmd)
}
