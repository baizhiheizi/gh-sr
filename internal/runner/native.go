package runner

import (
	"fmt"
	"io"
	"strings"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
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
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
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

func powerShellSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func windowsRunnerDirAssignment(h *host.Host, varName, instanceName string) string {
	return fmt.Sprintf("$%s = %s", varName, h.RunnerDirPS(instanceName))
}

func windowsNativeInstallScript(h *host.Host, instanceName, version, url string) string {
	zipName := fmt.Sprintf("actions-runner-%s.zip", version)
	return strings.Join([]string{
		windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		"New-Item -ItemType Directory -Force -Path $runnerDir | Out-Null",
		fmt.Sprintf("$zip = Join-Path %s %s", h.TempDirPS(), powerShellSingleQuoted(zipName)),
		fmt.Sprintf("if (-not (Test-Path $zip)) { Invoke-WebRequest -Uri %s -OutFile $zip }", powerShellSingleQuoted(url)),
		"Expand-Archive -Path $zip -DestinationPath $runnerDir -Force",
	}, "; ")
}

// nativeConfigURL returns the --url value for config.sh / config.cmd.
func nativeConfigURL(rc config.RunnerConfig) string {
	if rc.Org != "" {
		return "https://github.com/" + rc.Org
	}
	return "https://github.com/" + rc.Repo
}

func windowsNativeConfigScript(h *host.Host, rc config.RunnerConfig, instanceName, regToken string) string {
	labels := strings.Join(rc.EffectiveLabels(h.OS, h.Arch), ",")
	cmd := fmt.Sprintf(
		"& .\\config.cmd --unattended --url %s --token %s --name %s --labels %s --work '_work' --replace",
		powerShellSingleQuoted(nativeConfigURL(rc)),
		powerShellSingleQuoted(regToken),
		powerShellSingleQuoted(instanceName),
		powerShellSingleQuoted(labels),
	)
	if rc.Group != "" {
		cmd += fmt.Sprintf(" --runnergroup %s", powerShellSingleQuoted(rc.Group))
	}
	if rc.Ephemeral {
		cmd += " --ephemeral"
	}
	return strings.Join([]string{
		windowsRunnerDirAssignment(h, "runnerDir", instanceName),
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
	parts := []string{
		windowsRunnerDirAssignment(h, "runnerDir", instanceName) + "; ",
		`$pidFile = Join-Path $runnerDir '.runner_pid'; `,
		`$logFile = Join-Path $runnerDir 'runner.log'; `,
		`if (Test-Path $pidFile) { $existingPid = Get-Content $pidFile; try { Get-Process -Id $existingPid -EA Stop | Out-Null; Write-Host 'already running'; exit 0 } catch {} }; `,
		`$cmdArg = 'cd /d "' + $runnerDir + '" && run.cmd > "' + $logFile + '" 2>&1'; `,
		`$fullLine = 'cmd.exe /c ' + $cmdArg; `,
		`$cim = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = $fullLine; CurrentDirectory = $runnerDir }; `,
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
		windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		powerShellSingleQuoted(staleRegistrationMsg),
	)
}

// windowsDeleteRunnerConfig removes the .runner file so setupNative will re-configure.
func windowsDeleteRunnerConfig(h *host.Host, instanceName string) string {
	return fmt.Sprintf(
		"%s; Remove-Item -Force (Join-Path $runnerDir '.runner') -EA SilentlyContinue; "+
			"Remove-Item -Force (Join-Path $runnerDir '.credentials') -EA SilentlyContinue; "+
			"Remove-Item -Force (Join-Path $runnerDir '.credentials_rsaparams') -EA SilentlyContinue",
		windowsRunnerDirAssignment(h, "runnerDir", instanceName),
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

	for _, name := range rc.InstanceNames() {
		dir := h.RunnerDir(name)

		installed, _ := NativeRunnerConfigPresent(h, name)
		if installed {
			fmt.Fprintf(m.out(), "  %s: already installed, skipping\n", name)
			continue
		}

		fmt.Fprintf(m.out(), "  %s: installing runner v%s...\n", name, version)

		if h.OS == "linux" {
			installDepsCmd := linuxElevatePrelude + `
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

		if h.OS == "linux" {
			fmt.Fprintf(m.out(), "  %s: installing runner dependencies...\n", name)
			depsCmd := fmt.Sprintf(
				"cd %s && %s && $SUDO ./bin/installdependencies.sh",
				dir, strings.TrimSpace(linuxElevatePrelude),
			)
			if _, err := h.Run(depsCmd); err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to install runner dependencies: %v\n", name, err)
			}
		}

		regToken, err := m.GitHub.GetRegistrationTokenScoped(rc.Scope(), rc.ScopeTarget())
		if err != nil {
			return err
		}

		labels := strings.Join(rc.EffectiveLabels(h.OS, h.Arch), ",")

		fmt.Fprintf(m.out(), "  %s: registering runner with GitHub...\n", name)
		if h.OS == "windows" {
			if _, err := h.RunShell(windowsNativeConfigScript(h, rc, name, regToken)); err != nil {
				return fmt.Errorf("configuring runner on Windows: %w", err)
			}
		} else {
			configURL := nativeConfigURL(rc)
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
	}

	// For agentic runners, install gh-aw CLI on the host.
	// gh-aw (GitHub Agentic Workflows) uses Docker on the host for AWF sandbox containers.
	if rc.IsAgentic() && h.OS == "linux" {
		// Pre-flight check: warn if the runner user lacks passwordless sudo.
		// gh-aw requires sudo to manage iptables (DOCKER-USER chain) and awf-net bridge.
		warnAgenticSudoPrereqs(h, m.out(), rc.Name)

		// Clean up zombie Docker resources from previously crashed gh-aw jobs.
		// If a job crashes, orphaned gh-aw containers and networks block the next job.
		fmt.Fprintf(m.out(), "  %s: cleaning up zombie gh-aw Docker resources...\n", rc.Name)
		cleanupOut, cleanupErr := h.Run(`
docker ps -a --filter "name=gh-aw" --format '{{.ID}}' 2>/dev/null | xargs -r docker rm -f 2>/dev/null
docker network ls --filter "name=gh-aw" --format '{{.ID}}' 2>/dev/null | xargs -r docker network rm 2>/dev/null
docker network prune -f 2>/dev/null
echo "cleanup done"
`)
		if cleanupErr != nil {
			fmt.Fprintf(m.out(), "  %s: warning: Docker cleanup failed: %v\n", rc.Name, cleanupErr)
		} else if strings.TrimSpace(cleanupOut) != "" {
			fmt.Fprintf(m.out(), "  %s: Docker cleanup: %s\n", rc.Name, strings.TrimSpace(cleanupOut))
		}

		fmt.Fprintf(m.out(), "  %s: installing gh-aw CLI for agentic profile...\n", rc.Name)
		installGHAWCmd := `if [ -d ~/.local/share/gh/extensions/gh-aw ]; then
			echo "gh-aw already installed"
		else
			curl -sL https://raw.githubusercontent.com/github/gh-aw/main/install-gh-aw.sh | bash
		fi`
		out, err := h.Run(installGHAWCmd)
		if err != nil {
			fmt.Fprintf(m.out(), "  %s: warning: failed to install gh-aw CLI: %v\n", rc.Name, err)
		} else {
			fmt.Fprintf(m.out(), "  %s: gh-aw CLI installed\n", rc.Name)
			if out != "" {
				fmt.Fprintf(m.out(), "  %s: gh-aw: %s\n", rc.Name, strings.TrimSpace(out))
			}
		}

		// Set up /opt/hostedtoolcache for gh-aw agent containers.
		// gh-aw searches for tools (e.g., claude) in /opt/hostedtoolcache, which exists on
		// GitHub Hosted Runners but not on self-hosted runners. We create a bind mount so
		// agent containers can find npm-installed tools.
		fmt.Fprintf(m.out(), "  %s: setting up /opt/hostedtoolcache for agentic workflows...\n", rc.Name)
		npmPrefixCmd := `npm config get prefix 2>/dev/null || echo "/usr/local"`
		npmPrefix, err := h.Run(npmPrefixCmd)
		if err != nil {
			fmt.Fprintf(m.out(), "  %s: warning: failed to detect npm prefix: %v\n", rc.Name, err)
		} else {
			npmPrefix = strings.TrimSpace(npmPrefix)
			hostedtoolcacheSetup := fmt.Sprintf(`%s
			if [ -d /opt/hostedtoolcache ]; then
				if [ -L /opt/hostedtoolcache ]; then
					if [ "$(readlink -f /opt/hostedtoolcache)" != "%s" ]; then
						echo "Updating /opt/hostedtoolcache symlink to %s"
						$SUDO rm -f /opt/hostedtoolcache
						$SUDO mkdir -p /opt/hostedtoolcache
						$SUDO mount --bind %s /opt/hostedtoolcache
					else
						echo "/opt/hostedtoolcache already correctly configured"
					fi
				else
					echo "/opt/hostedtoolcache already exists as a directory"
				fi
			else
				echo "Creating /opt/hostedtoolcache -> %s"
				$SUDO mkdir -p /opt/hostedtoolcache
				$SUDO mount --bind %s /opt/hostedtoolcache
				if ! grep -q "^%s" /etc/fstab 2>/dev/null; then
					echo "%s /opt/hostedtoolcache none defaults,bind 0 0" | $SUDO tee -a /etc/fstab
				fi
			fi`, linuxElevatePrelude, npmPrefix, npmPrefix, npmPrefix, npmPrefix, npmPrefix, npmPrefix, npmPrefix)
			out, err := h.Run(hostedtoolcacheSetup)
			if err != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to set up /opt/hostedtoolcache: %v\n", rc.Name, err)
			} else {
				fmt.Fprintf(m.out(), "  %s: /opt/hostedtoolcache configured\n", rc.Name)
				if out != "" {
					fmt.Fprintf(m.out(), "  %s: hostedtoolcache: %s\n", rc.Name, strings.TrimSpace(out))
				}
			}
		}

		// Set up Docker DNS for agentic workflows.
		// Agent containers (gh-aw) use host.docker.internal to reach the MCP Gateway running
		// on the host network. Linux Docker does not resolve this by default; we configure
		// dnsmasq as a local DNS resolver that answers host.docker.internal from the docker0
		// bridge IP and forwards everything else upstream. This also ensures external DNS
		// (model provider APIs, GitHub, etc.) works from inside agent containers.
		fmt.Fprintf(m.out(), "  %s: setting up Docker DNS for agentic workflows...\n", rc.Name)
		if err := m.setupAgenticDNS(h, rc.Name); err != nil {
			fmt.Fprintf(m.out(), "  %s: warning: Docker DNS setup failed: %v\n", rc.Name, err)
		}

		// Pre-pull critical gh-aw images to reduce first-job startup latency.
		// These images are required by every agentic workflow run and can be several hundred MB.
		fmt.Fprintf(m.out(), "  %s: pre-pulling gh-aw images (this may take a few minutes)...\n", rc.Name)
		for _, img := range []string{
			"ghcr.io/github/gh-aw-firewall/agent:latest",
			"ghcr.io/github/gh-aw-mcpg:latest",
		} {
			pullOut, pullErr := h.Run(fmt.Sprintf("docker pull %s 2>&1", img))
			pullOut = strings.TrimSpace(pullOut)
			if pullErr != nil {
				fmt.Fprintf(m.out(), "  %s: warning: failed to pull %s: %v\n", rc.Name, img, pullErr)
				if pullOut != "" {
					fmt.Fprintf(m.out(), "  %s:   %s\n", rc.Name, pullOut)
				}
			} else {
				// Show the final status line (e.g. "Status: Image is up to date for ...")
				lines := strings.Split(pullOut, "\n")
				statusLine := pullOut
				for i := len(lines) - 1; i >= 0; i-- {
					if l := strings.TrimSpace(lines[i]); l != "" {
						statusLine = l
						break
					}
				}
				fmt.Fprintf(m.out(), "  %s: pulled %s: %s\n", rc.Name, img, statusLine)
			}
		}
	}

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
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
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

	_ = m.stopNative(h, instanceName)

	removeToken, err := m.GitHub.GetRemovalTokenScoped(rc.Scope(), rc.ScopeTarget())
	if err != nil {
		fmt.Fprintf(m.out(), "  %s: warning: could not get removal token: %v\n", instanceName, err)
	} else {
		if h.OS == "windows" {
			cmd := fmt.Sprintf(
				"%s; Set-Location -Path $runnerDir; & .\\config.cmd remove --token %s",
				windowsRunnerDirAssignment(h, "runnerDir", instanceName),
				powerShellSingleQuoted(removeToken),
			)
			h.RunShell(cmd)
		} else {
			cmd := fmt.Sprintf("cd %s && ./config.sh remove --token '%s'", dir, removeToken)
			h.Run(cmd)
		}
		fmt.Fprintf(m.out(), "  %s: deregistered\n", instanceName)
	}

	if h.OS == "windows" {
		h.RunShell(fmt.Sprintf("%s; Remove-Item -Recurse -Force $runnerDir", windowsRunnerDirAssignment(h, "runnerDir", instanceName)))
	} else {
		h.Run(fmt.Sprintf("rm -rf %s", dir))
	}

	fmt.Fprintf(m.out(), "  %s: removed\n", instanceName)
	return nil
}

func (m *Manager) statusNative(h *host.Host, instanceName string) string {
	dir := h.RunnerDir(instanceName)

	if kind, err := autostart.Detect(h, instanceName); err == nil && kind != autostart.KindNone {
		active, err := autostart.IsServiceActive(h, instanceName, kind)
		if err != nil {
			return "unknown"
		}
		if active {
			return "running"
		}
		return "stopped"
	}

	if h.OS == "windows" {
		cmd := fmt.Sprintf(
			"%s; if (-not (Test-Path (Join-Path $runnerDir '.runner'))) { Write-Host 'not installed'; exit 0 }; "+
				"$pidFile = Join-Path $runnerDir '.runner_pid'; "+
				"if (-not (Test-Path $pidFile)) { Write-Host 'stopped'; exit 0 }; "+
				"$p = Get-Content $pidFile; "+
				"try { Get-Process -Id $p -EA Stop | Out-Null; Write-Host 'running' } catch { Write-Host 'stopped' }",
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
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
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		)
		return h.RunShell(cmd)
	}

	cmd := fmt.Sprintf("tail -50 %s/runner.log 2>/dev/null || echo 'no logs found'", dir)
	return h.Run(cmd)
}

// setupAgenticDNS configures Docker DNS on a Linux host so that agent containers
// (gh-aw) can resolve host.docker.internal to the docker0 bridge IP and also reach
// external domains (model providers, GitHub, etc.). It is idempotent: safe to re-run.
func (m *Manager) setupAgenticDNS(h *host.Host, runnerName string) error {
	// Step 1: Check if Docker is available at all.
	dockerCheck, err := h.Run(`docker info >/dev/null 2>&1 && echo ok || echo missing`)
	if err != nil || strings.TrimSpace(dockerCheck) != "ok" {
		return fmt.Errorf("docker daemon not available on host; skipping DNS setup")
	}

	// Step 2: Detect docker0 bridge IP. Fall back to 172.17.0.1 if detection fails.
	bridgeIP, err := h.Run(`ip -4 addr show docker0 2>/dev/null | grep -oP 'inet \K[\d.]+'`)
	if err != nil || strings.TrimSpace(bridgeIP) == "" {
		bridgeIP = "172.17.0.1"
	} else {
		bridgeIP = strings.TrimSpace(bridgeIP)
	}

	// Step 3: Detect if host.docker.internal already resolves inside containers.
	// If it resolves to a non-loopback IP, skip DNS setup (user may have their own solution).
	checkCmd := `docker run --rm alpine sh -c "getent hosts host.docker.internal 2>/dev/null" 2>/dev/null`
	out, err := h.Run(checkCmd)
	if err == nil && strings.TrimSpace(out) != "" {
		fields := strings.Fields(strings.TrimSpace(out))
		if len(fields) > 0 && fields[0] != "127.0.0.1" && fields[0] != "::1" {
			// Already configured; nothing to do.
			return nil
		}
	}

	// Step 4: Detect the package manager.
	detectPM := `if command -v apt-get >/dev/null 2>&1; then echo apt
elif command -v dnf >/dev/null 2>&1; then echo dnf
elif command -v yum >/dev/null 2>&1; then echo yum
elif command -v apk >/dev/null 2>&1; then echo apk
else echo unknown; fi`
	pmOut, err := h.Run(detectPM)
	pm := strings.TrimSpace(pmOut)
	if pm == "unknown" {
		return fmt.Errorf("could not detect package manager; skipping DNS setup")
	}

	// Step 5: Install dnsmasq if missing.
	dnsmasqInstalled, err := h.Run(`command -v dnsmasq >/dev/null 2>&1 && echo yes || echo no`)
	dnsmasqInstalled = strings.TrimSpace(dnsmasqInstalled)
	if dnsmasqInstalled != "yes" {
		var installCmd string
		switch pm {
		case "apt":
			installCmd = fmt.Sprintf(`%s && $SUDO apt-get update && $SUDO apt-get install -y dnsmasq`, linuxElevatePrelude)
		case "dnf", "yum":
			installCmd = fmt.Sprintf(`%s && $SUDO %s install -y dnsmasq`, linuxElevatePrelude, pm)
		case "apk":
			installCmd = fmt.Sprintf(`%s && $SUDO apk add dnsmasq`, linuxElevatePrelude)
		}
		if _, err := h.Run(installCmd); err != nil {
			return fmt.Errorf("installing dnsmasq: %w", err)
		}
	}

	// Step 6: Write dnsmasq config for gh-sr.
	// Resolves host.docker.internal to the docker0 bridge IP and forwards everything else
	// to systemd-resolved (127.0.0.53) and 8.8.8.8. The config file is prefixed "gh-sr-"
	// so we can detect and manage it separately from any user-provided config.
	dnsmasqConf := fmt.Sprintf(`address=/host.docker.internal/%s
listen-address=%s
bind-interfaces
server=127.0.0.53
server=8.8.8.8
`, bridgeIP, bridgeIP)
	// Use linuxElevatePrelude once at the top to set $SUDO, then use $SUDO throughout.
	confWrite := linuxElevatePrelude + fmt.Sprintf(`
CONF=/etc/dnsmasq.d/gh-sr-docker.conf
TMPCONF=$(mktemp)
cat > "$TMPCONF" << 'GHSREOF'
%sGHSREOF
if ! cmp -s "$TMPCONF" "$CONF" 2>/dev/null; then
    $SUDO cp "$TMPCONF" "$CONF"
    $SUDO systemctl restart dnsmasq
    echo "dnsmasq configured"
else
    echo "dnsmasq config unchanged"
fi
rm -f "$TMPCONF"`, dnsmasqConf)
	out, err = h.Run(confWrite)
	if err != nil {
		return fmt.Errorf("writing dnsmasq config: %w", err)
	}
	if out != "" {
		fmt.Fprintf(m.out(), "  %s: dnsmasq: %s\n", runnerName, strings.TrimSpace(out))
	}

	// Step 7: Configure Docker daemon DNS if not already set to use our dnsmasq.
	// linuxElevatePrelude is prepended once to set $SUDO; all elevated ops use $SUDO thereafter.
	daemonDNSConfigured, err := h.Run(linuxElevatePrelude + fmt.Sprintf(`
DOCKER_CONF=/etc/docker/daemon.json
BRIDGE_IP='%s'
if [ ! -f "$DOCKER_CONF" ]; then
    printf '{"dns":["%s","8.8.8.8"]}\n' "$BRIDGE_IP" | $SUDO tee "$DOCKER_CONF" > /dev/null
    $SUDO systemctl restart docker
    echo "daemon.json created with DNS"
else
    # Check if our dnsmasq IP is already in the dns list.
    if grep -q '"'"'"$BRIDGE_IP"'"'"' "$DOCKER_CONF" 2>/dev/null; then
        echo "daemon.json DNS already configured"
    else
        # Merge: add our dnsmasq IP at the front of the existing dns array, preserve other keys.
        # Try python3 first, then fall back to a shell-based approach.
        if command -v python3 >/dev/null 2>&1; then
            python3 -c "
import json, sys
path = '$DOCKER_CONF'
try:
    with open(path) as f:
        data = json.load(f)
except:
    data = {}
dns = data.get('dns', [])
if '$BRIDGE_IP' not in dns:
    dns.insert(0, '$BRIDGE_IP')
    dns = [d for d in dns if d]
data['dns'] = dns
with open(path, 'w') as f:
    json.dump(data, f, indent=2)
print('daemon.json DNS merged')
" 2>/dev/null && $SUDO systemctl restart docker && echo "daemon.json DNS updated"
        else
            # Fallback: use jq if available, otherwise just overwrite dns array safely.
            if command -v jq >/dev/null 2>&1; then
                $SUDO jq '.dns = ["'"'"'$BRIDGE_IP'"'"'", (.dns // [])[0:5]] | .dns += ["8.8.8.8"] | .dns = (.dns | unique)' "$DOCKER_CONF" > "${DOCKER_CONF}.new" && $SUDO mv "${DOCKER_CONF}.new" "$DOCKER_CONF" && $SUDO systemctl restart docker && echo "daemon.json DNS updated via jq"
            else
                # Last resort: read existing content and rebuild the dns array.
                # This is fragile but works on minimal systems without python3 or jq.
                $SUDO sh -c 'grep -v dns "$1" > "${1}.new"' -- "$DOCKER_CONF" && echo "  \"dns\": [\"$BRIDGE_IP\",\"8.8.8.8\"]" | $SUDO tee -a "${DOCKER_CONF}.new" > /dev/null && echo "}" | $SUDO tee -a "${DOCKER_CONF}.new" > /dev/null && $SUDO mv "${DOCKER_CONF}.new" "$DOCKER_CONF" && $SUDO systemctl restart docker && echo "daemon.json DNS updated via shell"
            fi
        fi
    fi
fi`, bridgeIP, bridgeIP))
	if err != nil && !strings.Contains(daemonDNSConfigured, "already configured") && !strings.Contains(daemonDNSConfigured, "configured") && !strings.Contains(daemonDNSConfigured, "updated") {
		// Non-fatal: docker restart may fail if there are running containers.
		fmt.Fprintf(m.out(), "  %s: warning: Docker daemon DNS merge failed: %v\n", runnerName, err)
	} else if daemonDNSConfigured != "" {
		fmt.Fprintf(m.out(), "  %s: docker: %s\n", runnerName, strings.TrimSpace(daemonDNSConfigured))
	}

	// Step 8: Verify DNS resolution from inside a container.
	verifyCmd := `docker run --rm alpine sh -c "getent hosts host.docker.internal 2>/dev/null && nslookup github.com >/dev/null 2>&1 && echo ok" 2>/dev/null`
	verifyOut, err := h.Run(verifyCmd)
	if err != nil || strings.TrimSpace(verifyOut) != "ok" {
		fmt.Fprintf(m.out(), "  %s: warning: container DNS verification failed; agentic workflows may not work. Run 'gh sr doctor' for diagnostics.\n", runnerName)
	} else {
		fmt.Fprintf(m.out(), "  %s: Docker DNS verified inside containers\n", runnerName)
	}

	return nil
}

// warnAgenticSudoPrereqs checks whether the runner user has passwordless sudo and, if not,
// prints a warning with remediation instructions. gh-aw requires sudo to manage iptables
// (DOCKER-USER chain) and the awf-net Docker bridge. This is a non-blocking warning: setup
// continues regardless so that the user can fix sudo and re-run without restarting from scratch.
func warnAgenticSudoPrereqs(h *host.Host, w io.Writer, runnerName string) {
	uid, err := h.Run(`id -u`)
	if err != nil {
		return // cannot determine, skip silently
	}
	if strings.TrimSpace(uid) == "0" {
		return // running as root, no sudo needed
	}
	out, err := h.Run(`sudo -n true 2>/dev/null && echo ok || echo no`)
	if err != nil || strings.TrimSpace(out) != "ok" {
		userName, _ := h.Run(`id -un`)
		userName = strings.TrimSpace(userName)
		fmt.Fprintf(w, "  %s: WARNING: passwordless sudo not available for user %q\n", runnerName, userName)
		fmt.Fprintf(w, "  %s:   gh-aw requires passwordless sudo to manage iptables and Docker bridge networks.\n", runnerName)
		fmt.Fprintf(w, "  %s:   To fix, run as root on the host:\n", runnerName)
		fmt.Fprintf(w, "  %s:     echo \"%s ALL=(ALL) NOPASSWD:ALL\" > /etc/sudoers.d/gh-sr-%s\n", runnerName, userName, userName)
		fmt.Fprintf(w, "  %s:     chmod 0440 /etc/sudoers.d/gh-sr-%s\n", runnerName, userName)
	}
}
