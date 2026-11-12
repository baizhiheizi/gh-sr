package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/probe"
)

// Manager.Status batches every per-instance probe on a host into ONE SSH
// round-trip via the scripts in this file. Before the batch, the native Linux
// path cost 2-3 round trips per instance (the linuxInstanceProbe marker
// probe, then the svc.sh systemctl state probe, or the autostart state probe
// twice — ServiceActiveState is called both directly and through
// IsServiceActive), and the container path cost one round trip per instance.
// A 16-instance host paid 32 round trips per Status tick on the TUI refresh
// path; the batch answers all of them in one.
//
// Both scripts render the state machine ONCE and drive it from a remote loop
// over a heredoc of instance records, following dirSizesBatchPOSIX. That
// keeps the script length proportional to the instance-name list rather than
// to the number of instances times the size of the state machine; a bounded
// chunker (statusBatchScriptMaxBytes) splits the list across more than one
// exec only when even the names alone would exceed an SSH channel packet.
//
// Each loop iteration emits one tagged marker (internal/probe wire format):
//
//	@@ns:<i>|<local-state>|<runner-version>   (native Linux)
//	@@cs:<i>|<status>|<configImage>|<digest>|<imageRev>   (container)
//
// so the Go-side parse feeds the same per-instance values the per-instance
// probes produced. A batch failure (SSH error, malformed or truncated output)
// returns ok=false and Status falls back to the per-instance probes verbatim,
// which preserves the pre-batch behaviour on transient failures — a degraded
// tick still fills every row instead of blanking the dashboard.

// statusBatchScriptHeader opens both batch scripts; tests and mocks use it to
// recognise a batch script on the wire.
const statusBatchScriptHeader = "# ghsr-batch-status-v1"

const (
	nativeStatusBatchTag    = "@@ns:"
	containerStatusBatchTag = "@@cs:"
)

// statusBatchRowsDelim terminates the heredoc that carries the instance list.
// It is quoted on the shell side (<<'...') so instance names are never
// expanded by the remote shell.
const statusBatchRowsDelim = "GHSR_STATUS_INSTANCES"

// statusBatchScriptMaxBytes caps the length of one batch script, i.e. one
// SSH exec channel request. SSH's channel-request payload is bounded by the
// peer's max packet size (classically 32768 bytes; older OpenSSH builds,
// non-OpenSSH servers, and jump hosts may advertise less). The original
// per-instance expansion wrote ~1.3 KB of shell per instance, so a 24-runner
// host already exceeded 32 KB and would silently fall back to 2N+1 round
// trips — worse than the 2N it replaced. Rendering the state machine once
// inside a remote loop fixes the growth; this cap is the belt-and-braces
// guarantee, and statusBatchChunkScript slices the list into bounded execs
// instead of ever emitting an oversized request. 16 KB keeps a full margin
// under the 32 KB classic limit.
const statusBatchScriptMaxBytes = 16 * 1024

// statusBatchResult carries the per-instance answers one batch script
// produced. Native probes fill local+version; container probes fill
// local+image+imageRev.
type statusBatchResult struct {
	local    string
	version  string
	image    string
	imageRev string
}

// statusNativeBatch answers Manager.Status's native Linux probes for every
// instance in one round trip (more only if the name list alone exceeds
// statusBatchScriptMaxBytes). ok=false means the caller must fall back to the
// per-instance probes.
func statusNativeBatch(h *host.Host, instances []string) ([]statusBatchResult, bool) {
	rows, ok := nativeStatusBatchRows(instances)
	if !ok {
		return nil, false
	}
	prefix := nativeStatusBatchPrefix(h)
	return runStatusBatch(h, rows, nativeStatusBatchTag, parseNativeStatusTail,
		func(chunk []string) string { return renderStatusBatchScript(prefix, chunk) })
}

// containerStatusBatch answers Manager.Status's container probes for every
// instance in one round trip. ok=false means fall back to the per-instance
// containerLocalStatusOneShot probes.
func containerStatusBatch(h *host.Host, instances []string) ([]statusBatchResult, bool) {
	rows, ok := containerStatusBatchRows(instances)
	if !ok {
		return nil, false
	}
	prefix := containerStatusBatchPrefix(h)
	return runStatusBatch(h, rows, containerStatusBatchTag, parseContainerStatusTail,
		func(chunk []string) string { return renderStatusBatchScript(prefix, chunk) })
}

// nativeStatusBatchRows renders one heredoc row per instance: the instance
// name and its sanitized systemd unit basename. Sanitization (and the shell
// row check) can fail; as with the original script the caller then falls back
// to the per-instance probes, which own that error path.
func nativeStatusBatchRows(instances []string) ([]string, bool) {
	rows := make([]string, len(instances))
	for i, inst := range instances {
		if !statusBatchRowSafe(inst) {
			return nil, false
		}
		san, err := autostart.SanitizeInstance(inst)
		if err != nil {
			return nil, false
		}
		rows[i] = inst + "|" + autostart.ServiceBasename(san)
	}
	return rows, true
}

// containerStatusBatchRows renders one heredoc row per instance. The
// container name is derived from $inst inside the remote loop.
func containerStatusBatchRows(instances []string) ([]string, bool) {
	rows := make([]string, len(instances))
	for i, inst := range instances {
		if !statusBatchRowSafe(inst) {
			return nil, false
		}
		rows[i] = inst
	}
	return rows, true
}

// statusBatchRowSafe rejects names that would break the heredoc framing.
// The quoted delimiter keeps every other shell metacharacter literal.
func statusBatchRowSafe(inst string) bool {
	return inst != "" && !strings.ContainsAny(inst, "|\r\n")
}

// renderStatusBatchScript appends the instance rows and the closing heredoc
// delimiter to a loop prefix. prefix ends with "done <<'DELIM'\n".
func renderStatusBatchScript(prefix string, rows []string) string {
	var b strings.Builder
	b.Grow(len(prefix) + 64)
	b.WriteString(prefix)
	for _, row := range rows {
		b.WriteString(row)
		b.WriteByte('\n')
	}
	b.WriteString(statusBatchRowsDelim + "\n")
	return b.String()
}

// statusBatchChunkScript picks the largest prefix of rows whose rendered
// script stays within statusBatchScriptMaxBytes. A single row that cannot fit
// returns count=0 so the caller falls back rather than sending an oversized
// exec. The fixed cost is measured by rendering with no rows.
func statusBatchChunkScript(rows []string, build func([]string) string) (string, int) {
	budget := statusBatchScriptMaxBytes - len(build(nil))
	if budget <= 0 {
		return "", 0
	}
	n := 0
	for n < len(rows) && len(rows[n])+1 <= budget {
		budget -= len(rows[n]) + 1
		n++
	}
	if n == 0 {
		return "", 0
	}
	return build(rows[:n]), n
}

// runStatusBatch executes the bounded chunks and stitches their per-chunk
// answers back into one result slice in instance order. Markers are numbered
// 1..count within each chunk (the remote loop increments $n), so the same
// parser validates every chunk.
func runStatusBatch(
	h *host.Host,
	rows []string,
	tag string,
	parseTail func(string, *statusBatchResult),
	build func([]string) string,
) ([]statusBatchResult, bool) {
	results := make([]statusBatchResult, len(rows))
	for start := 0; start < len(rows); {
		script, count := statusBatchChunkScript(rows[start:], build)
		if count == 0 {
			return nil, false
		}
		out, err := h.Run(script)
		if err != nil {
			return nil, false
		}
		part, ok := parseStatusBatch(out, tag, count, parseTail)
		if !ok {
			return nil, false
		}
		copy(results[start:start+count], part)
		start += count
	}
	return results, true
}

// parseStatusBatch decodes one batch stream into n per-instance results.
// Lines with a foreign tag are ignored (stray remote output must not corrupt
// a row). Any malformed index — out of range or a duplicate — or a missing
// instance index fails the whole batch, so Status falls back to the
// per-instance probes instead of trusting a corrupted stream. This is the
// contract stated in the PR: "missing/duplicate/out-of-range indices fail the
// whole batch". A duplicate in particular means the remote emitted two
// different answers for one instance; taking the first silently would be a
// guess, and a fallback costs one extra round trip but shows the truth.
func parseStatusBatch(out, tag string, n int, parseTail func(tail string, r *statusBatchResult)) ([]statusBatchResult, bool) {
	results := make([]statusBatchResult, n)
	seen := make([]bool, n)
	for d := range probe.Lines(out) {
		if !d.OK || d.Tag != tag {
			continue
		}
		if d.Index < 1 || d.Index > n || seen[d.Index-1] {
			return nil, false
		}
		parseTail(d.Tail, &results[d.Index-1])
		seen[d.Index-1] = true
	}
	for _, ok := range seen {
		if !ok {
			return nil, false
		}
	}
	return results, true
}

func parseNativeStatusTail(tail string, r *statusBatchResult) {
	r.local, r.version, _ = strings.Cut(tail, "|")
}

func parseContainerStatusTail(tail string, r *statusBatchResult) {
	r.local, r.image, r.imageRev = parseContainerStatusInspectOutput(tail)
}

// nativeStatusBatchPrefix renders the one-time elevation gate and the single
// copy of the native Linux status state machine, driven by a remote loop over
// the instance rows. It mirrors linuxInstanceProbe + statusNativeState:
//
//  1. Version: `cat $dir/.runner-version`, whitespace collapsed to single
//     words (mirroring the space-join the V-marker parser applies to
//     multi-line versions; runner versions are single-word in practice).
//  2. svc.sh branch: when $dir/svc.sh and $dir/.service exist, the systemctl
//     state probe answers verbatim — active → running, any other non-empty
//     output → stopped, empty → fall through. This mirrors
//     statusNativeState's hasSvc branch (its round trip #2).
//  3. Autostart branch: user unit → plain `systemctl --user is-active`,
//     system unit → `$SUDO systemctl is-active` behind the same elevation
//     gate LinuxElevatePrelude enforces. The old per-instance probe ran the
//     hard prelude (exiting 1 without root/passwordless sudo, which made
//     statusNativeState fall through to the PID-file check); here the gate is
//     evaluated once per script and a failed gate skips to the same fallback.
//     active → running; failed/activating → service error; anything else →
//     fall through. IsServiceActive's second identical probe is folded away:
//     it only re-read the state the first probe had already reported
//     inactive, and one script reads one snapshot.
//  4. PID-file fallback (statusNativePIDFile): no .runner → not installed,
//     no .runner_pid → stopped, unreadable pid file → unknown, readable pid
//     that is alive → running, everything else (dead PID, or a present but
//     EMPTY pid file) → stopped. The empty-file case deliberately matches
//     statusNativePIDFile: `pid=$(cat …)` succeeds with pid="", then
//     `kill -0 ""` fails → stopped. It must not become "unknown", which
//     would make the TUI report an unknown state for a plainly stopped
//     runner.
//
// The marker only ever carries a mapped state (running/stopped/service
// error/not installed/unknown — no pipes or newlines), so multi-line raw
// systemctl output like "inactive\ninactive" cannot split the marker line:
// it maps to stopped before the emit, exactly as the Go TrimSpace+compare
// does.
func nativeStatusBatchPrefix(h *host.Host) string {
	var b strings.Builder
	b.WriteString(statusBatchScriptHeader + "\n")
	// One elevation check per script instead of one hard prelude per probe:
	// SUDO/ce reproduce what runActiveCheck's KindSystemdSystem case observed
	// (root or passwordless sudo → probe; otherwise fall through to the
	// PID-file check).
	b.WriteString("SUDO=''\n")
	b.WriteString("ce=1\n")
	b.WriteString("if [ \"$(id -u)\" -ne 0 ]; then\n")
	b.WriteString("  if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then SUDO='sudo -n'; else ce=0; fi\n")
	b.WriteString("fi\n")
	// noglob: the version collapse below word-splits $v unquoted, and a
	// version containing glob metacharacters must not expand against cwd.
	b.WriteString("set -f\n")
	b.WriteString("n=0\n")
	b.WriteString("while IFS='|' read -r inst base; do\n")
	b.WriteString("  [ -n \"$inst\" ] || continue\n")
	b.WriteString("  n=$((n+1))\n")
	fmt.Fprintf(&b, "  dir=\"%s/$inst\"\n", h.RunnerBaseDir())
	b.WriteString("  upath=\"$HOME/.config/systemd/user/$base.service\"\n")
	b.WriteString("  spath=\"/etc/systemd/system/$base.service\"\n")
	b.WriteString("  st=''; src=''\n")
	b.WriteString("  v=$(cat \"$dir/.runner-version\" 2>/dev/null || true)\n")
	b.WriteString("  v=$(printf '%s ' $v); v=${v%% }\n")
	b.WriteString("  if [ -f \"$dir/svc.sh\" ] && svc_file=\"$dir/.service\" && [ -f \"$svc_file\" ]; then\n")
	b.WriteString("    st=$(systemctl is-active \"$(cat \"$svc_file\")\" 2>/dev/null || echo inactive); src=svc\n")
	b.WriteString("  fi\n")
	b.WriteString("  if [ -z \"$st\" ] && [ -f \"$upath\" ]; then\n")
	b.WriteString("    st=$(systemctl --user is-active \"$base.service\" 2>/dev/null || echo inactive); src=kind\n")
	b.WriteString("  elif [ -z \"$st\" ] && [ -f \"$spath\" ] && [ \"$ce\" = 1 ]; then\n")
	b.WriteString("    st=$($SUDO systemctl is-active \"$base.service\" 2>/dev/null || echo inactive); src=kind\n")
	b.WriteString("  fi\n")
	b.WriteString("  rstate=''\n")
	// The two branches map raw states differently, mirroring
	// statusNativeState: the svc.sh branch turns any non-active state
	// into stopped, while the autostart branch only recognises
	// active/failed/activating and leaves everything else (inactive, …)
	// empty so the PID-file fallback decides.
	b.WriteString("  if [ -n \"$st\" ]; then\n")
	b.WriteString("    if [ \"$st\" = active ]; then rstate=running\n")
	b.WriteString("    elif [ \"$src\" = svc ]; then rstate=stopped\n")
	b.WriteString("    elif [ \"$st\" = failed ] || [ \"$st\" = activating ]; then rstate='service error'\n")
	b.WriteString("    fi\n")
	b.WriteString("  fi\n")
	b.WriteString("  if [ -z \"$rstate\" ]; then\n")
	b.WriteString("    if [ ! -f \"$dir/.runner\" ]; then rstate='not installed'\n")
	b.WriteString("    elif [ ! -f \"$dir/.runner_pid\" ]; then rstate=stopped\n")
	b.WriteString("    elif pid=$(cat \"$dir/.runner_pid\" 2>/dev/null); then\n")
	b.WriteString("      if [ -n \"$pid\" ] && kill -0 \"$pid\" 2>/dev/null; then rstate=running; else rstate=stopped; fi\n")
	b.WriteString("    else rstate=unknown; fi\n")
	b.WriteString("  fi\n")
	probe.NewWriter(&b).Echo(nativeStatusBatchTag, "$n", "$rstate|$v")
	b.WriteString("done <<'" + statusBatchRowsDelim + "'\n")
	return b.String()
}

// buildNativeStatusBatchScript renders one bounded chunk of the native batch
// script. It returns the number of rows the chunk carries (the marker indices
// the script emits are 1..count). count=0 means the rows cannot fit and the
// caller must fall back.
func buildNativeStatusBatchScript(h *host.Host, rows []string) (string, int) {
	prefix := nativeStatusBatchPrefix(h)
	return statusBatchChunkScript(rows, func(chunk []string) string {
		return renderStatusBatchScript(prefix, chunk)
	})
}

// containerStatusBatchPrefix renders the single copy of the container status
// state machine, driven by a remote loop over the instance rows. The body is
// containerStatusProbeBody — the exact same fragment the one-shot probe runs
// — so a payload or bootstrap-failed change cannot drift between the two
// paths. Only the emit differs: one-shot prints `st`, the batch tags it.
func containerStatusBatchPrefix(h *host.Host) string {
	var b strings.Builder
	b.WriteString(statusBatchScriptHeader + "\n")
	b.WriteString("n=0\n")
	b.WriteString("while IFS= read -r inst; do\n")
	b.WriteString("  [ -n \"$inst\" ] || continue\n")
	b.WriteString("  n=$((n+1))\n")
	fmt.Fprintf(&b, "  sd=\"%s/$inst\"\n", h.RunnerBaseDir())
	b.WriteString("  cid=\"gh-sr-$inst\"\n")
	b.WriteString(containerStatusProbeBody())
	probe.NewWriter(&b).Echo(containerStatusBatchTag, "$n", "$st")
	b.WriteString("done <<'" + statusBatchRowsDelim + "'\n")
	return b.String()
}

// buildContainerStatusBatchScript renders one bounded chunk of the container
// batch script; see buildNativeStatusBatchScript for the chunk contract.
func buildContainerStatusBatchScript(h *host.Host, rows []string) (string, int) {
	prefix := containerStatusBatchPrefix(h)
	return statusBatchChunkScript(rows, func(chunk []string) string {
		return renderStatusBatchScript(prefix, chunk)
	})
}
