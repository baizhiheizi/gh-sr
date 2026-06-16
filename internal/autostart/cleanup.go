package autostart

import (
	"fmt"
	"sort"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

const serviceBasenamePrefix = "ghsr-runner-"

// ListInstalled returns instance names that have gh-sr autostart artifacts installed on the host.
func ListInstalled(h *host.Host) ([]string, error) {
	switch h.OS {
	case "linux":
		return listInstalledLinux(h)
	case "darwin":
		return listInstalledDarwin(h)
	case "windows":
		return listInstalledWindows(h)
	default:
		return nil, fmt.Errorf("unsupported host OS %q", h.OS)
	}
}

func listInstalledLinux(h *host.Host) ([]string, error) {
	cmd := `set -e
for f in "$HOME/.config/systemd/user/ghsr-runner-"*.service; do
  [ -f "$f" ] || continue
  basename "$f" .service
done
for f in /etc/systemd/system/ghsr-runner-*.service; do
  [ -f "$f" ] || continue
  basename "$f" .service
done
`
	out, err := h.Run(cmd)
	if err != nil {
		return nil, err
	}
	return dedupeInstances(parseInstanceLines(out)), nil
}

func listInstalledDarwin(h *host.Host) ([]string, error) {
	cmd := `for f in "$HOME/Library/LaunchAgents/com.github.ghsr.runner."*.plist; do
  [ -f "$f" ] || continue
  basename "$f" .plist
done
`
	out, err := h.Run(cmd)
	if err != nil {
		return nil, err
	}
	var instances []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		const prefix = "com.github.ghsr.runner."
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		instances = append(instances, strings.TrimPrefix(line, prefix))
	}
	return dedupeInstances(instances), nil
}

func listInstalledWindows(h *host.Host) ([]string, error) {
	ps := `Get-ScheduledTask -ErrorAction SilentlyContinue |
  Where-Object { $_.TaskName -like 'ghsr-runner-*' } |
  ForEach-Object { $_.TaskName }`
	out, err := h.RunShell(ps)
	if err != nil {
		return nil, err
	}
	var instances []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if inst, ok := instanceFromServiceBasename(line); ok {
			instances = append(instances, inst)
		}
	}
	return dedupeInstances(instances), nil
}

func parseInstanceLines(out string) []string {
	var instances []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if inst, ok := instanceFromServiceBasename(line); ok {
			instances = append(instances, inst)
		}
	}
	return instances
}

func instanceFromServiceBasename(base string) (string, bool) {
	if !strings.HasPrefix(base, serviceBasenamePrefix) {
		return "", false
	}
	inst := strings.TrimPrefix(base, serviceBasenamePrefix)
	if inst == "" {
		return "", false
	}
	return inst, true
}

func dedupeInstances(instances []string) []string {
	seen := make(map[string]struct{}, len(instances))
	out := make([]string, 0, len(instances))
	for _, inst := range instances {
		if _, ok := seen[inst]; ok {
			continue
		}
		seen[inst] = struct{}{}
		out = append(out, inst)
	}
	sort.Strings(out)
	return out
}

// IsStale reports whether autostart for instance should be removed because the runner directory or launcher is missing.
func IsStale(h *host.Host, instance string) (bool, error) {
	dir := h.RunnerDir(instance)

	if h.OS == "windows" {
		ps := fmt.Sprintf(
			`$d=%s; if ((Test-Path -LiteralPath $d -PathType Container) -and (Test-Path -LiteralPath (Join-Path $d 'run.cmd'))) { 'no' } else { 'yes' }`,
			hostshell.PowerShellSingleQuote(dir),
		)
		out, err := h.RunShell(ps)
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(out) == "yes", nil
	}

	out, err := h.Run(fmt.Sprintf(`test -d %s && test -f %s/run.sh && echo no || echo yes`, dir, dir))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}

// CleanupStale removes autostart for installed instances whose runner directory is missing.
// When dryRun is true, stale instances are reported without uninstalling.
func CleanupStale(h *host.Host, dryRun bool) (removed []string, found int, err error) {
	installed, err := ListInstalled(h)
	if err != nil {
		return nil, 0, err
	}
	for _, inst := range installed {
		stale, serr := IsStale(h, inst)
		if serr != nil {
			return removed, found, fmt.Errorf("%s: checking stale autostart: %w", inst, serr)
		}
		if !stale {
			continue
		}
		found++
		if dryRun {
			removed = append(removed, inst)
			continue
		}
		if uerr := Uninstall(h, inst); uerr != nil {
			return removed, found, fmt.Errorf("%s: removing stale autostart: %w", inst, uerr)
		}
		removed = append(removed, inst)
	}
	return removed, found, nil
}
