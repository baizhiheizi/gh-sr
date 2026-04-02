package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-runners/internal/config"
	"github.com/an-lee/gh-runners/internal/host"
)

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

		installed, _ := h.Run(fmt.Sprintf("test -f %s/.runner && echo yes || echo no", dir))
		if strings.TrimSpace(installed) == "yes" {
			fmt.Printf("  %s: already installed, skipping\n", name)
			continue
		}

		fmt.Printf("  %s: installing runner v%s...\n", name, version)

		if h.OS == "linux" {
			installDepsCmd := `
				SUDO=''; if command -v sudo >/dev/null 2>&1 && [ "$(id -u)" -ne 0 ]; then SUDO=sudo; fi;
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
			cmds := []string{
				fmt.Sprintf("New-Item -ItemType Directory -Force -Path '%s' | Out-Null", dir),
				fmt.Sprintf("$zip = '%s\\actions-runner-%s.zip'", h.TempDir(), version),
				fmt.Sprintf("if (-not (Test-Path $zip)) { Invoke-WebRequest -Uri '%s' -OutFile $zip }", url),
				fmt.Sprintf("Expand-Archive -Path $zip -DestinationPath '%s' -Force", dir),
			}
			if _, err := h.RunShell(strings.Join(cmds, "; ")); err != nil {
				return fmt.Errorf("installing runner on Windows: %w", err)
			}
		} else {
			cmds := fmt.Sprintf(
				"mkdir -p %s && cd %s && curl -sL '%s' | tar xz",
				dir, dir, url,
			)
			if _, err := h.Run(cmds); err != nil {
				return fmt.Errorf("installing runner: %w", err)
			}
		}

		if h.OS == "linux" {
			depsCmd := fmt.Sprintf(
				"cd %s && SUDO=''; if command -v sudo >/dev/null 2>&1 && [ \"$(id -u)\" -ne 0 ]; then SUDO=sudo; fi; $SUDO ./bin/installdependencies.sh",
				dir,
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
			configCmd := fmt.Sprintf(
				"cd '%s'; .\\config.cmd --unattended --url 'https://github.com/%s' --token '%s' --name '%s' --labels '%s' --work '_work' --replace",
				dir, rc.Repo, regToken, name, labels,
			)
			if _, err := h.RunShell(configCmd); err != nil {
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

func (m *Manager) startNative(h *host.Host, instanceName string) error {
	dir := h.RunnerDir(instanceName)

	if h.OS == "windows" {
		cmd := fmt.Sprintf(
			"$d = '%s'; $pidFile = Join-Path $d '.runner_pid'; "+
				"if (Test-Path $pidFile) { $p = Get-Content $pidFile; try { Get-Process -Id $p -EA Stop | Out-Null; Write-Host 'already running'; exit 0 } catch {} }; "+
				"$proc = Start-Process -FilePath (Join-Path $d 'run.cmd') -WorkingDirectory $d -PassThru -WindowStyle Hidden; "+
				"$proc.Id | Out-File -FilePath $pidFile -NoNewline; Write-Host \"started PID $($proc.Id)\"",
			dir,
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
			"$d = '%s'; $pidFile = Join-Path $d '.runner_pid'; "+
				"if (-not (Test-Path $pidFile)) { Write-Host 'not running'; exit 0 }; "+
				"$p = Get-Content $pidFile; "+
				"try { Stop-Process -Id $p -Force -EA Stop; Write-Host 'stopped' } catch { Write-Host 'not running' }; "+
				"Remove-Item $pidFile -Force -EA SilentlyContinue",
			dir,
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
			cmd := fmt.Sprintf("cd '%s'; .\\config.cmd remove --token '%s'", dir, removeToken)
			h.RunShell(cmd)
		} else {
			cmd := fmt.Sprintf("cd %s && ./config.sh remove --token '%s'", dir, removeToken)
			h.Run(cmd)
		}
		fmt.Printf("  %s: deregistered\n", instanceName)
	}

	if h.OS == "windows" {
		h.RunShell(fmt.Sprintf("Remove-Item -Recurse -Force '%s'", dir))
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
			"$d = '%s'; if (-not (Test-Path (Join-Path $d '.runner'))) { Write-Host 'not installed'; exit 0 }; "+
				"$pidFile = Join-Path $d '.runner_pid'; "+
				"if (-not (Test-Path $pidFile)) { Write-Host 'stopped'; exit 0 }; "+
				"$p = Get-Content $pidFile; "+
				"try { Get-Process -Id $p -EA Stop | Out-Null; Write-Host 'running' } catch { Write-Host 'stopped' }",
			dir,
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
		cmd := fmt.Sprintf("Get-Content -Tail 50 '%s\\runner.log'", dir)
		return h.RunShell(cmd)
	}

	cmd := fmt.Sprintf("tail -50 %s/runner.log 2>/dev/null || echo 'no logs found'", dir)
	return h.Run(cmd)
}
