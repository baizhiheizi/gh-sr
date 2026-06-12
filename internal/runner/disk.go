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

// PruneOptions configures PruneInstance.
type PruneOptions struct {
	DryRun         bool
	PruneCache     bool // also prune inner Docker cache (docker-data); default keeps cache
	IncludeOrphans bool
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

// SafeRunnerInstanceName reports whether name is safe to embed in remote shell paths.
func SafeRunnerInstanceName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("invalid instance name %q", name)
	}
	if strings.ContainsAny(name, "/\\\x00\n\r") {
		return fmt.Errorf("invalid instance name %q", name)
	}
	if strings.ContainsAny(name, `;"|&<>$`+"`") {
		return fmt.Errorf("invalid instance name %q", name)
	}
	return nil
}

func posixRunnerDirVar(instance string) string {
	inst := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(instance)
	return fmt.Sprintf(`dir="$HOME/.gh-sr/runners/%s"`, inst)
}

func posixScriptHeader(instance string) string {
	return "set -e\n" + posixRunnerDirVar(instance) + "\n"
}

// containerEscalation returns the "docker inspect → start if down → exec if up"
// shell snippet. shellCmd is run via `sh -c` inside the container, so callers
// can pass compound commands like `for sub in ...; do ...; done` without
// re-quoting. When the docker CLI is unavailable or the container cannot be
// started, the snippet is a no-op; the surrounding script must still degrade
// gracefully when the inner command never runs.
func containerEscalation(containerName, shellCmd string) string {
	q := hostshell.PosixSingleQuote(containerName)
	return fmt.Sprintf(`
if command -v docker >/dev/null 2>&1; then
  if ! docker inspect --format='{{.State.Running}}' %s 2>/dev/null | grep -q true; then
    docker start %s >/dev/null 2>&1 || true
  fi
  if docker inspect --format='{{.State.Running}}' %s 2>/dev/null | grep -q true; then
    docker exec %s sh -c %s || true
  fi
fi
`, q, q, q, q, hostshell.PosixSingleQuote(shellCmd))
}

// passwordlessSudo returns the hostshell.LinuxElevatePreludeSoft fragment used
// by disk-prune scripts that need non-interactive root or passwordless sudo
// over SSH. The soft variant is required because these scripts run several
// elevated commands sequentially (clearWorkTempPOSIX, removeDirTreePOSIX) and
// must keep going when one fails so the user sees each per-command outcome.
// Callers gate usage of "$SUDO" with `if [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ]`
// (or similar) so the empty-string case falls through to a non-elevated attempt
// that surfaces the real permission error.
//
// This thin wrapper exists for symmetry with internal/runner/sudo.go and so the
// test can call a package-local name.
func passwordlessSudo() string {
	return hostshell.LinuxElevatePreludeSoft()
}

// ListRunnerInstanceDirs returns subdirectory names under ~/.gh-sr/runners on the host.
// Names that fail SafeRunnerInstanceName are omitted.
func ListRunnerInstanceDirs(h *host.Host) ([]string, error) {
	raw, err := runOnHostOS(h,
		func() ([]string, error) {
			ps := `$base = Join-Path $env:USERPROFILE '.gh-sr\runners'; if (Test-Path $base) { Get-ChildItem -Path $base -Directory -ErrorAction SilentlyContinue | ForEach-Object { $_.Name } }`
			out, err := h.RunShell(ps)
			if err != nil {
				return nil, err
			}
			return splitNonEmptyLines(out), nil
		},
		func() ([]string, error) {
			out, err := h.Run(`ls -1 "$HOME/.gh-sr/runners" 2>/dev/null || true`)
			if err != nil {
				return nil, err
			}
			return splitNonEmptyLines(out), nil
		},
	)
	if err != nil {
		return nil, err
	}
	var safe []string
	for _, name := range raw {
		if SafeRunnerInstanceName(name) == nil {
			safe = append(safe, name)
		}
	}
	return safe, nil
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

	if err := SafeRunnerInstanceName(instance); err != nil {
		entry.Err = err
		return entry
	}

	if rc != nil {
		entry.Mode = rc.EffectiveRunnerMode()
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
	case "linux", "darwin":
		return dirSizesPOSIX(h, instance)
	default:
		return 0, 0, 0, 0, fmt.Errorf("unsupported host OS %q", h.OS)
	}
}

// buildDirSizesPOSIXScript returns the shell script that `dirSizesPOSIX`
// runs on the remote host. Exposed (1) so the structural test in
// `disk_test.go` can assert the single-walk invariant against the real
// production string instead of a frozen copy, and (2) so a future
// refactor that re-introduces multiple `du` walks fires the test.
func buildDirSizesPOSIXScript(instance string) string {
	// Single `du` walk with depth 1 reports the total and the size of each
	// first-level subdirectory in one pass. Replaces four separate `du` calls
	// (one for $dir, one each for _work/_temp/docker-data) which re-walked
	// overlapping subtrees. On remote hosts the savings compound because each
	// `h.Run` is a separate SSH round trip.
	//
	// `du` flag differs by platform: GNU coreutils uses --max-depth=N, BSD/macOS
	// uses -d N. Probe with --max-depth=0 and fall back to -d 0.
	return fmt.Sprintf(`
%sif [ ! -d "$dir" ]; then echo "0 0 0 0"; exit 0; fi
if du --max-depth=0 "$dir" >/dev/null 2>&1; then
  out=$(du --max-depth=1 -k "$dir" 2>/dev/null)
else
  out=$(du -d 1 -k "$dir" 2>/dev/null)
fi
if [ -z "$out" ]; then echo "0 0 0 0"; exit 0; fi
total=0; work=0; temp=0; docker=0
total_name=$(basename "$dir")
# Default IFS splits the "size<TAB>path" lines. GNU du emits tab-separated;
# BSD du emits space-separated. Both work with the default.
while read -r size path; do
  [ -z "$path" ] && continue
  case "$(basename "$path")" in
    "$total_name") total=$((size * 1024)) ;;
    _work)         work=$((size * 1024)) ;;
    _temp)         temp=$((size * 1024)) ;;
    docker-data)   docker=$((size * 1024)) ;;
  esac
done <<< "$out"
echo "$total $work $temp $docker"
`, posixScriptHeader(instance))
}

func dirSizesPOSIX(h *host.Host, instance string) (total, work, temp, dockerData int64, err error) {
	out, err := h.Run(buildDirSizesPOSIXScript(instance))
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
function Ghsr-OtherDirSize([string]$root) {
  if (-not (Test-Path -LiteralPath $root)) { return 0 }
  $skip = @('_work','_temp','docker-data')
  $sum = [int64]0
  Get-ChildItem -LiteralPath $root -Force -ErrorAction SilentlyContinue | ForEach-Object {
    if ($skip -contains $_.Name) { return }
    if ($_.PSIsContainer) { $sum += Ghsr-DirSize $_.FullName } else { $sum += [int64]$_.Length }
  }
  return $sum
}
$d = %s
$w = Ghsr-DirSize (Join-Path $d '_work')
$te = Ghsr-DirSize (Join-Path $d '_temp')
$dk = Ghsr-DirSize (Join-Path $d 'docker-data')
$other = Ghsr-OtherDirSize $d
$t = $w + $te + $dk + $other
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
	if err := SafeRunnerInstanceName(instance); err != nil {
		res.Err = err
		return res
	}
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

	workAction := fmt.Sprintf("clear %s/_work and %s/_temp", dir, dir)
	res.Actions = append(res.Actions, workAction)
	if !opts.DryRun {
		if err := clearWorkTemp(h, instance, containerPruneMode(rc)); err != nil {
			res.Err = err
			return res
		}
	}

	if containerPruneMode(rc) && opts.PruneCache {
		cname := ContainerDockerName(instance)
		cacheAction := fmt.Sprintf("inner docker cache prune in %s", cname)
		res.Actions = append(res.Actions, cacheAction)
		if !opts.DryRun {
			if err := pruneInnerDockerCache(h, cname); err != nil {
				res.Err = err
			}
		}
	}

	return res
}

// containerPruneMode reports whether disk prune should use container escalation paths.
func containerPruneMode(rc *config.RunnerConfig) bool {
	return rc != nil && rc.IsContainerMode()
}

func clearWorkTemp(h *host.Host, instance string, containerMode bool) error {
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
	case "linux", "darwin":
		_, err := h.Run(clearWorkTempPOSIX(instance, containerMode))
		return err
	default:
		return fmt.Errorf("unsupported host OS %q", h.OS)
	}
}

// clearWorkTempPOSIX removes job scratch under _work and _temp. CI jobs often leave
// root-owned files on the host bind mount; we escalate via docker exec (container
// runners) or passwordless host sudo when a plain rm is not enough.
func clearWorkTempPOSIX(instance string, containerMode bool) string {
	var containerBlock string
	if containerMode {
		containerBlock = containerEscalation(
			ContainerDockerName(instance),
			`for sub in _work _temp; do p="/runner-state/$sub"; if [ -d "$p" ]; then find "$p" -mindepth 1 -maxdepth 1 -exec rm -rf {} +; fi; done`,
		)
	}
	return fmt.Sprintf(`
%s
clear_one() {
  p="$1"
  if [ ! -d "$p" ]; then return 0; fi
  find "$p" -mindepth 1 -maxdepth 1 -exec rm -rf {} + 2>/dev/null || true
  if [ -n "$(ls -A "$p" 2>/dev/null)" ]; then return 1; fi
  return 0
}
need_elev=0
for sub in _work _temp; do
  clear_one "$dir/$sub" || need_elev=1
done
if [ "$need_elev" -eq 0 ]; then exit 0; fi
%s
%s
if [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ]; then
  for sub in _work _temp; do
    p="$dir/$sub"
    if [ -d "$p" ] && [ -n "$(ls -A "$p" 2>/dev/null)" ]; then
      find "$p" -mindepth 1 -maxdepth 1 -exec $SUDO rm -rf {} + 2>/dev/null || true
    fi
  done
fi
for sub in _work _temp; do
  p="$dir/$sub"
  if [ -d "$p" ] && [ -n "$(ls -A "$p" 2>/dev/null)" ]; then
    echo "disk prune: cannot remove files in $p (permission denied); for container runners ensure the container is running or use passwordless sudo on the host" >&2
    exit 1
  fi
done
`, posixScriptHeader(instance), containerBlock, passwordlessSudo())
}

func removeDirTree(h *host.Host, instance string) error {
	switch h.OS {
	case "windows":
		dirExpr := h.RunnerDirPS(instance)
		ps := fmt.Sprintf(`if (Test-Path -LiteralPath (%s)) { Remove-Item -LiteralPath (%s) -Recurse -Force }`, dirExpr, dirExpr)
		_, err := h.RunShell(ps)
		return err
	case "linux", "darwin":
		_, err := h.Run(removeDirTreePOSIX(instance))
		return err
	default:
		return fmt.Errorf("unsupported host OS %q", h.OS)
	}
}

func removeDirTreePOSIX(instance string) string {
	containerBlock := containerEscalation(
		ContainerDockerName(instance),
		`rm -rf /runner-state`,
	)
	return fmt.Sprintf(`
%s
if [ -d "$dir" ]; then
  rm -rf "$dir" 2>/dev/null || true
fi
if [ -d "$dir" ]; then
  %s
  %s
  if [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ]; then
    $SUDO rm -rf "$dir" 2>/dev/null || true
  fi
fi
if [ -d "$dir" ]; then
  echo "disk prune: cannot remove orphan directory $dir (permission denied); ensure the runner container is running or use passwordless sudo on the host" >&2
  exit 1
fi
`, posixScriptHeader(instance), containerBlock, passwordlessSudo())
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
