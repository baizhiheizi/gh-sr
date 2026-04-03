package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

// NativeRunnerConfigPresent reports whether the remote instance directory contains
// a configured runner (.runner), matching the check used by setup and doctor.
func NativeRunnerConfigPresent(h *host.Host, instanceName string) (bool, error) {
	dir := h.RunnerDir(instanceName)
	if h.OS == "windows" {
		out, err := h.RunShell(fmt.Sprintf(
			"%s; if (Test-Path (Join-Path $runnerDir '.runner')) { Write-Output 'yes' } else { Write-Output 'no' }",
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		))
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(out) == "yes", nil
	}
	out, err := h.Run(fmt.Sprintf("test -f %s/.runner && echo yes || echo no", dir))
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

func windowsNativeConfigScript(h *host.Host, rc config.RunnerConfig, instanceName, regToken string) string {
	labels := strings.Join(rc.Labels, ",")
	return strings.Join([]string{
		windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		"Set-Location -Path $runnerDir",
		fmt.Sprintf(
			"& .\\config.cmd --unattended --url %s --token %s --name %s --labels %s --work '_work' --replace",
			powerShellSingleQuoted("https://github.com/"+rc.Repo),
			powerShellSingleQuoted(regToken),
			powerShellSingleQuoted(instanceName),
			powerShellSingleQuoted(labels),
		),
	}, "; ")
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
			fmt.Printf("  %s: already installed, skipping\n", name)
			continue
		}

		fmt.Printf("  %s: installing runner v%s...\n", name, version)

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
				fmt.Printf("  %s: warning: failed to ensure curl/tar are installed: %v\n", name, err)
			}
		}

		if h.OS == "windows" {
			if _, err := h.RunShell(windowsNativeInstallScript(h, name, version, url)); err != nil {
				return fmt.Errorf("installing runner on Windows: %w", err)
			}
		} else {
			tarball := fmt.Sprintf("%s/ghr-runner-%s-%s.tar.gz", h.TempDir(), name, version)
			cmds := fmt.Sprintf(
				"mkdir -p %s && cd %s && tarball=%s && rm -f \"$tarball\" && "+
					"curl -fSL --retry 3 --retry-delay 2 -o \"$tarball\" '%s' && "+
					"tar xzf \"$tarball\" && rm -f \"$tarball\"",
				dir, dir, tarball, url,
			)
			if _, err := h.Run(cmds); err != nil {
				return fmt.Errorf("installing runner: %w", err)
			}
		}

		if h.OS == "linux" {
			depsCmd := fmt.Sprintf(
				"cd %s && %s && $SUDO ./bin/installdependencies.sh",
				dir, strings.TrimSpace(linuxElevatePrelude),
			)
			if _, err := h.Run(depsCmd); err != nil {
				fmt.Printf("  %s: warning: failed to install runner dependencies: %v\n", name, err)
			}
		}

		regToken, err := m.GitHub.GetRegistrationToken(rc.Repo)
		if err != nil {
			return err
		}

		labels := strings.Join(rc.Labels, ",")

		if h.OS == "windows" {
			if _, err := h.RunShell(windowsNativeConfigScript(h, rc, name, regToken)); err != nil {
				return fmt.Errorf("configuring runner on Windows: %w", err)
			}
		} else {
			configCmd := fmt.Sprintf(
				"cd %s && ./config.sh --unattended --url 'https://github.com/%s' --token '%s' --name '%s' --labels '%s' --work '_work' --replace",
				dir, rc.Repo, regToken, name, labels,
			)
			if _, err := h.Run(configCmd); err != nil {
				return fmt.Errorf("configuring runner: %w", err)
			}
		}

		fmt.Printf("  %s: configured\n", name)
	}

	return nil
}

func (m *Manager) startNative(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	dir := h.RunnerDir(instanceName)

	ok, err := NativeRunnerConfigPresent(h, instanceName)
	if err != nil {
		return fmt.Errorf("checking runner install at %s: %w", dir, err)
	}
	if !ok {
		return fmt.Errorf("runner not installed on host %s at %s; run: ghr setup %s", h.Name, dir, rc.Name)
	}

	if h.OS == "windows" {
		cmd := fmt.Sprintf(
			"%s; $pidFile = Join-Path $runnerDir '.runner_pid'; "+
				"$logFile = Join-Path $runnerDir 'runner.log'; "+
				"if (Test-Path $pidFile) { $p = Get-Content $pidFile; try { Get-Process -Id $p -EA Stop | Out-Null; Write-Host 'already running'; exit 0 } catch {} }; "+
				"$proc = Start-Process -FilePath (Join-Path $runnerDir 'run.cmd') -WorkingDirectory $runnerDir -PassThru -WindowStyle Hidden "+
				"-RedirectStandardOutput $logFile -RedirectStandardError $logFile; "+
				"$proc.Id | Out-File -FilePath $pidFile -NoNewline; Write-Host \"started PID $($proc.Id)\"",
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		)
		out, err := h.RunShell(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("  %s: %s\n", instanceName, strings.TrimSpace(out))
		return nil
	}

	cmd := fmt.Sprintf(
		`cd %s && pid_file=".runner_pid" && `+
			`if [ -f "$pid_file" ] && kill -0 "$(cat "$pid_file")" 2>/dev/null; then echo "already running"; exit 0; fi && `+
			`nohup ./run.sh > runner.log 2>&1 & echo $! > "$pid_file" && echo "started PID $!"`,
		dir,
	)
	out, err := h.Run(cmd)
	if err != nil {
		return err
	}
	fmt.Printf("  %s: %s\n", instanceName, strings.TrimSpace(out))
	return nil
}

func (m *Manager) stopNative(h *host.Host, instanceName string) error {
	dir := h.RunnerDir(instanceName)

	if h.OS == "windows" {
		cmd := fmt.Sprintf(
			"%s; $pidFile = Join-Path $runnerDir '.runner_pid'; "+
				"if (-not (Test-Path $pidFile)) { Write-Host 'not running'; exit 0 }; "+
				"$p = Get-Content $pidFile; "+
				"try { Stop-Process -Id $p -Force -EA Stop; Write-Host 'stopped' } catch { Write-Host 'not running' }; "+
				"Remove-Item $pidFile -Force -EA SilentlyContinue",
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		)
		out, err := h.RunShell(cmd)
		if err != nil {
			return err
		}
		fmt.Printf("  %s: %s\n", instanceName, strings.TrimSpace(out))
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
	fmt.Printf("  %s: %s\n", instanceName, strings.TrimSpace(out))
	return nil
}

func (m *Manager) removeNative(h *host.Host, rc config.RunnerConfig, instanceName string) error {
	dir := h.RunnerDir(instanceName)

	_ = m.stopNative(h, instanceName)

	removeToken, err := m.GitHub.GetRemovalToken(rc.Repo)
	if err != nil {
		fmt.Printf("  %s: warning: could not get removal token: %v\n", instanceName, err)
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
		fmt.Printf("  %s: deregistered\n", instanceName)
	}

	if h.OS == "windows" {
		h.RunShell(fmt.Sprintf("%s; Remove-Item -Recurse -Force $runnerDir", windowsRunnerDirAssignment(h, "runnerDir", instanceName)))
	} else {
		h.Run(fmt.Sprintf("rm -rf %s", dir))
	}

	fmt.Printf("  %s: removed\n", instanceName)
	return nil
}

func (m *Manager) statusNative(h *host.Host, instanceName string) string {
	dir := h.RunnerDir(instanceName)

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
				"if (-not (Test-Path $logFile)) { Write-Output 'no logs found' } else { Get-Content -Tail 50 -LiteralPath $logFile }",
			windowsRunnerDirAssignment(h, "runnerDir", instanceName),
		)
		return h.RunShell(cmd)
	}

	cmd := fmt.Sprintf("tail -50 %s/runner.log 2>/dev/null || echo 'no logs found'", dir)
	return h.Run(cmd)
}
