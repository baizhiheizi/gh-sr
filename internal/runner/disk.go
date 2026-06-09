package runner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// DiskWarnThresholdGiB is the doctor warning threshold for runner state directories.
const DiskWarnThresholdGiB = 50

// DiskUsageEntry reports disk consumption for one runner instance directory.
type DiskUsageEntry struct {
	Instance        string
	Host            string
	Path            string
	Orphan          bool
	Mode            string // "native" or "container"
	Busy            bool
	Remote          string // GitHub status when known
	TotalBytes      int64
	WorkBytes       int64
	TempBytes       int64
	DockerDataBytes int64
	OtherBytes      int64
	Err             error
}

// TotalGiB returns total size in gibibytes.
func (e DiskUsageEntry) TotalGiB() float64 {
	return float64(e.TotalBytes) / (1024 * 1024 * 1024)
}

// PruneOptions configures PruneInstance.
type PruneOptions struct {
	DryRun         bool
	PruneCache     bool // also prune inner Docker cache (docker-data); default keeps cache
	IncludeOrphans bool
	Force          bool // prune when GitHub status unknown
}

// PruneResult summarizes one prune attempt.
type PruneResult struct {
	Instance string
	Host     string
	Skipped  bool
	Reason   string
	Actions  []string
	Err      error
}

// DiskWarnThresholdBytes is DiskWarnThresholdGiB as bytes.
func DiskWarnThresholdBytes() int64 {
	return int64(DiskWarnThresholdGiB) * 1024 * 1024 * 1024
}

func posixRunnerDirVar(instance string) string {
	inst := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(instance)
	return fmt.Sprintf(`dir="$HOME/.gh-sr/runners/%s"`, inst)
}

// ListRunnerInstanceDirs returns subdirectory names under ~/.gh-sr/runners on the host.
func ListRunnerInstanceDirs(h *host.Host) ([]string, error) {
	switch h.OS {
	case "windows":
		ps := `$base = Join-Path $env:USERPROFILE '.gh-sr\runners'; if (Test-Path $base) { Get-ChildItem -Path $base -Directory -ErrorAction SilentlyContinue | ForEach-Object { $_.Name } }`
		out, err := h.RunShell(ps)
		if err != nil {
			return nil, err
		}
		return splitNonEmptyLines(out), nil
	default:
		out, err := h.Run(`ls -1 "$HOME/.gh-sr/runners" 2>/dev/null || true`)
		if err != nil {
			return nil, err
		}
		return splitNonEmptyLines(out), nil
	}
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// MeasureDiskUsage measures disk usage for one runner instance directory.
func MeasureDiskUsage(h *host.Host, hostName, instance string, rc *config.RunnerConfig) DiskUsageEntry {
	entry := DiskUsageEntry{
		Instance: instance,
		Host:     hostName,
		Path:     h.RunnerDir(instance),
	}

	if rc != nil {
		if rc.IsContainerMode() {
			entry.Mode = "container"
		} else {
			entry.Mode = "native"
		}
	} else {
		entry.Orphan = true
		entry.Mode = "unknown"
	}

	total, work, temp, dockerData, err := dirSizes(h, instance)
	if err != nil {
		entry.Err = err
		return entry
	}
	entry.TotalBytes = total
	entry.WorkBytes = work
	entry.TempBytes = temp
	entry.DockerDataBytes = dockerData
	other := total - work - temp - dockerData
	if other < 0 {
		other = 0
	}
	entry.OtherBytes = other
	return entry
}

func dirSizes(h *host.Host, instance string) (total, work, temp, dockerData int64, err error) {
	switch h.OS {
	case "windows":
		return dirSizesWindows(h, instance)
	default:
		return dirSizesPOSIX(h, instance)
	}
}

func dirSizesPOSIX(h *host.Host, instance string) (total, work, temp, dockerData int64, err error) {
	script := fmt.Sprintf(`
set -e
%s
if [ ! -d "$dir" ]; then echo "0 0 0 0"; exit 0; fi
total=$(du -sk "$dir" 2>/dev/null | awk '{print $1*1024}')
work=0; temp=0; docker=0
if [ -d "$dir/_work" ]; then work=$(du -sk "$dir/_work" 2>/dev/null | awk '{print $1*1024}'); fi
if [ -d "$dir/_temp" ]; then temp=$(du -sk "$dir/_temp" 2>/dev/null | awk '{print $1*1024}'); fi
if [ -d "$dir/docker-data" ]; then docker=$(du -sk "$dir/docker-data" 2>/dev/null | awk '{print $1*1024}'); fi
echo "$total $work $temp $docker"
`, posixRunnerDirVar(instance))
	out, err := h.Run(script)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return parseFourInt64s(out)
}

func dirSizesWindows(h *host.Host, instance string) (total, work, temp, dockerData int64, err error) {
	dirExpr := h.RunnerDirPS(instance)
	ps := fmt.Sprintf(`
function Ghsr-DirSize([string]$p) {
  if (-not (Test-Path -LiteralPath $p)) { return 0 }
  $sum = (Get-ChildItem -LiteralPath $p -Recurse -Force -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
  if ($null -eq $sum) { return 0 }
  return [int64]$sum
}
$d = %s
$t = Ghsr-DirSize $d
$w = Ghsr-DirSize (Join-Path $d '_work')
$te = Ghsr-DirSize (Join-Path $d '_temp')
$dk = Ghsr-DirSize (Join-Path $d 'docker-data')
Write-Output "$t $w $te $dk"
`, dirExpr)
	out, err := h.RunShell(ps)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return parseFourInt64s(out)
}

func parseFourInt64s(out string) (a, b, c, d int64, err error) {
	line := strings.TrimSpace(out)
	if line == "" {
		return 0, 0, 0, 0, nil
	}
	// Use last line in case of noise.
	if idx := strings.LastIndex(line, "\n"); idx >= 0 {
		line = strings.TrimSpace(line[idx+1:])
	}
	fields := strings.Fields(line)
	vals := make([]int64, 4)
	for i := 0; i < 4 && i < len(fields); i++ {
		vals[i], err = strconv.ParseInt(fields[i], 10, 64)
		if err != nil {
			return 0, 0, 0, 0, fmt.Errorf("parsing size line %q: %w", line, err)
		}
	}
	return vals[0], vals[1], vals[2], vals[3], nil
}

// PruneInstance reclaims disk for one runner instance when idle.
func (m *Manager) PruneInstance(h *host.Host, hostName, instance string, rc *config.RunnerConfig, busy bool, opts PruneOptions) PruneResult {
	res := PruneResult{Instance: instance, Host: hostName}
	if busy {
		res.Skipped = true
		res.Reason = "busy"
		return res
	}

	dir := h.RunnerDir(instance)

	isOrphan := rc == nil
	if isOrphan && !opts.IncludeOrphans {
		res.Skipped = true
		res.Reason = "orphan (use --include-orphans)"
		return res
	}

	if isOrphan && opts.IncludeOrphans {
		action := fmt.Sprintf("remove orphan directory %s", dir)
		res.Actions = append(res.Actions, action)
		if !opts.DryRun {
			if err := removeDirTree(h, instance); err != nil {
				res.Err = err
			}
		}
		return res
	}

	// Work and temp — safe on host bind mount.
	workAction := fmt.Sprintf("clear %s/_work and %s/_temp", dir, dir)
	res.Actions = append(res.Actions, workAction)
	if !opts.DryRun {
		if err := clearWorkTemp(h, instance); err != nil {
			res.Err = err
			return res
		}
	}

	if rc != nil && rc.IsContainerMode() && opts.PruneCache {
		cname := ContainerDockerName(instance)
		cacheAction := fmt.Sprintf("inner docker cache prune in %s", cname)
		res.Actions = append(res.Actions, cacheAction)
		if !opts.DryRun {
			if err := pruneInnerDockerCache(h, cname); err != nil {
				res.Actions = append(res.Actions, "warning: "+err.Error())
			}
		}
	}

	return res
}

func clearWorkTemp(h *host.Host, instance string) error {
	switch h.OS {
	case "windows":
		dirExpr := h.RunnerDirPS(instance)
		ps := fmt.Sprintf(`
foreach ($sub in @('_work','_temp')) {
  $p = Join-Path (%s) $sub
  if (Test-Path -LiteralPath $p) {
    Get-ChildItem -LiteralPath $p -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
  }
}
`, dirExpr)
		_, err := h.RunShell(ps)
		return err
	default:
		script := fmt.Sprintf(`
set -e
%s
for sub in _work _temp; do
  p="$dir/$sub"
  if [ -d "$p" ]; then
    find "$p" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  fi
done
`, posixRunnerDirVar(instance))
		_, err := h.Run(script)
		return err
	}
}

func removeDirTree(h *host.Host, instance string) error {
	switch h.OS {
	case "windows":
		dirExpr := h.RunnerDirPS(instance)
		ps := fmt.Sprintf(`if (Test-Path -LiteralPath (%s)) { Remove-Item -LiteralPath (%s) -Recurse -Force }`, dirExpr, dirExpr)
		_, err := h.RunShell(ps)
		return err
	default:
		script := fmt.Sprintf(`
set -e
%s
rm -rf "$dir"
`, posixRunnerDirVar(instance))
		_, err := h.Run(script)
		return err
	}
}

func pruneInnerDockerCache(h *host.Host, containerName string) error {
	q := hostshell.PosixSingleQuote(containerName)
	check, err := h.Run(fmt.Sprintf("docker exec %s docker info >/dev/null 2>&1 && echo ok || echo no", q))
	if err != nil || strings.TrimSpace(check) != "ok" {
		return fmt.Errorf("inner dockerd not responding in %s; skipped cache prune", containerName)
	}
	_, err = h.Run(fmt.Sprintf("docker exec %s docker system prune -af --volumes", q))
	return err
}

// FormatBytesHuman formats bytes as GiB/MiB for display.
func FormatBytesHuman(b int64) string {
	if b < 0 {
		b = 0
	}
	const gib = 1024 * 1024 * 1024
	const mib = 1024 * 1024
	if b >= gib {
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(gib))
	}
	if b >= mib {
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(mib))
	}
	if b >= 1024 {
		return fmt.Sprintf("%.1f KiB", float64(b)/1024)
	}
	return fmt.Sprintf("%d B", b)
}
