package runner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/autostart"
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
	q := QuoteContainerName(containerName)
	execCmd := DockerExecCommand(containerName, "sh -c "+hostshell.PosixSingleQuote(shellCmd))
	return fmt.Sprintf(`
if command -v docker >/dev/null 2>&1; then
  if ! docker inspect --format='{{.State.Running}}' %s 2>/dev/null | grep -q true; then
    docker start %s >/dev/null 2>&1 || true
  fi
  if docker inspect --format='{{.State.Running}}' %s 2>/dev/null | grep -q true; then
    %s || true
  fi
fi
`, q, q, q, execCmd)
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
	// SplitSeq avoids the upfront []string allocation that strings.Split makes
	// for the full output of the per-host `ls -1 ~/.gh-sr/runners` (or PowerShell
	// Get-ChildItem) command. The returned slice is what callers consume, so the
	// caller-side allocation is unchanged; only the intermediate slice is removed.
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
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

// dirSizesResult bundles the four size buckets dirSizes collects on the remote
// host so it can flow through runOnHostOS's generic dispatch (the helper
// returns a single value, and dirSizes needs to ship four int64s back).
type dirSizesResult struct {
	total, work, temp, dockerData int64
}

func dirSizes(h *host.Host, instance string) (total, work, temp, dockerData int64, err error) {
	res, err := runOnHostOS(h,
		func() (dirSizesResult, error) {
			t, w, te, dk, ierr := dirSizesWindows(h, instance)
			return dirSizesResult{t, w, te, dk}, ierr
		},
		func() (dirSizesResult, error) {
			t, w, te, dk, ierr := dirSizesPOSIX(h, instance)
			return dirSizesResult{t, w, te, dk}, ierr
		},
	)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return res.total, res.work, res.temp, res.dockerData, nil
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

// parseFourInt64s extracts up to four int64 values from the trailing
// whitespace-separated line of `out`. The line shape is the emit produced by
// the `du`-based POSIX script (`echo "$total $work $temp $docker"`) and the
// `Write-Output "$t $w $te $dk"` PowerShell path. The inner scanner is a
// manual ASCII scan over the line — strings.Fields would allocate a
// []string header for every call, and parseFourInt64s runs once per host per
// `gh sr disk` listing refresh, so the saved allocation compounds across
// listings with many hosts.
func parseFourInt64s(out string) (a, b, c, d int64, err error) {
	// Trim leading whitespace and a single trailing newline without
	// strings.TrimSpace's full Unicode pass.
	idx := 0
	for idx < len(out) && (out[idx] == ' ' || out[idx] == '\t' || out[idx] == '\n' || out[idx] == '\r') {
		idx++
	}
	if idx >= len(out) {
		return 0, 0, 0, 0, nil
	}
	end := len(out)
	for end > idx && (out[end-1] == ' ' || out[end-1] == '\t' || out[end-1] == '\n' || out[end-1] == '\r') {
		end--
	}
	line := out[idx:end]
	// If the script emitted multiple lines (a stale diagnostic trailing the
	// sizes), keep the last non-empty line — matches the prior LastIndex('\n')
	// + TrimSpace behavior.
	if nl := strings.LastIndexByte(line, '\n'); nl >= 0 {
		seg := line[nl+1:]
		t := seg
		j := 0
		for j < len(t) && (t[j] == ' ' || t[j] == '\t' || t[j] == '\r') {
			j++
		}
		k := len(t)
		for k > j && (t[k-1] == ' ' || t[k-1] == '\t' || t[k-1] == '\r') {
			k--
		}
		line = t[j:k]
		if line == "" {
			return 0, 0, 0, 0, nil
		}
	}
	// Manual scan: split on whitespace into at most 4 substrings of `line`,
	// then strconv.ParseInt each one. The field slice is a [4]string stack
	// array so no heap allocation is needed for typical 4-number lines.
	var fields [4]string
	nf := 0
	for i := 0; i < len(line); {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		if nf < 4 {
			fields[nf] = line[start:i]
			nf++
		}
	}
	var vals [4]int64
	for i := 0; i < nf; i++ {
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
			if kind, err := autostart.Detect(h, instance); err == nil && kind != autostart.KindNone {
				_ = autostart.Uninstall(h, instance)
			}
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
	_, err := runOnHostOS(h,
		func() (struct{}, error) {
			dirExpr := h.RunnerDirPS(instance)
			ps := fmt.Sprintf(`
foreach ($sub in @('_work','_temp')) {
  $p = Join-Path (%s) $sub
  if (Test-Path -LiteralPath $p) {
    Get-ChildItem -LiteralPath $p -Force -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
  }
}
`, dirExpr)
			_, ierr := h.RunShell(ps)
			return struct{}{}, ierr
		},
		func() (struct{}, error) {
			_, ierr := h.Run(clearWorkTempPOSIX(instance, containerMode))
			return struct{}{}, ierr
		},
	)
	return err
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
	_, err := runOnHostOS(h,
		func() (struct{}, error) {
			dirExpr := h.RunnerDirPS(instance)
			ps := fmt.Sprintf(`if (Test-Path -LiteralPath (%s)) { Remove-Item -LiteralPath (%s) -Recurse -Force }`, dirExpr, dirExpr)
			_, ierr := h.RunShell(ps)
			return struct{}{}, ierr
		},
		func() (struct{}, error) {
			_, ierr := h.Run(removeDirTreePOSIX(instance))
			return struct{}{}, ierr
		},
	)
	return err
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
	check, err := h.Run(DockerExecCommand(containerName, "docker info >/dev/null 2>&1 && echo ok || echo no"))
	if err != nil || strings.TrimSpace(check) != "ok" {
		return fmt.Errorf("inner dockerd not responding in %s; skipped cache prune", containerName)
	}
	_, err = h.Run(DockerExecCommand(containerName, "docker system prune -af --volumes"))
	return err
}

// FormatBytesHuman formats bytes as GiB/MiB/KiB/B for display.
//
// strconv.AppendFloat + stack-allocated byte buffer + inline unit-suffix
// bytes avoids the two allocations the previous `string(AppendFloat(...)) +
// " GiB"` chain dragged in (one for the string coercion, one for the concat).
// Writing the unit suffix directly into the buffer collapses to a single
// allocation on the GiB/MiB/KiB branches.
//
// Called 5× per row by ops.PrintDiskUsage and once per host by doctor
// DiskEntry rendering, so the per-call alloc drop compounds across listings
// with many instances.
//
// The largest realistic output is "9999.9 GiB" (10 chars); [24]byte holds
// AppendFloat's worst case (~24 chars) plus the 4-char unit suffix.
func FormatBytesHuman(b int64) string {
	if b < 0 {
		b = 0
	}
	const gib = 1024 * 1024 * 1024
	const mib = 1024 * 1024
	var buf [24]byte
	switch {
	case b >= gib:
		out := buf[:0]
		out = strconv.AppendFloat(out, float64(b)/float64(gib), 'f', 1, 64)
		out = append(out, ' ', 'G', 'i', 'B')
		return string(out)
	case b >= mib:
		out := buf[:0]
		out = strconv.AppendFloat(out, float64(b)/float64(mib), 'f', 1, 64)
		out = append(out, ' ', 'M', 'i', 'B')
		return string(out)
	case b >= 1024:
		out := buf[:0]
		out = strconv.AppendFloat(out, float64(b)/1024, 'f', 1, 64)
		out = append(out, ' ', 'K', 'i', 'B')
		return string(out)
	default:
		return strconv.FormatInt(b, 10) + " B"
	}
}
