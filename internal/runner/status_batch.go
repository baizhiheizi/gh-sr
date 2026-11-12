package runner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
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
// Both scripts run the full per-instance state machine in the remote shell and
// emit one tagged marker per instance (internal/probe wire format):
//
//	@@ns:<i>|<local-state>|<runner-version>   (native Linux)
//	@@cs:<i>|<status>|<configImage>|<digest>|<imageRev>   (container)
//
// so the Go-side parse feeds the same per-instance values the per-instance
// probes produced. A batch failure (SSH error, truncated output) returns
// ok=false and Status falls back to the per-instance probes verbatim, which
// preserves the pre-batch behaviour on transient failures — a degraded tick
// still fills every row instead of blanking the dashboard.

// statusBatchScriptHeader opens both batch scripts; tests and mocks use it to
// recognise a batch script on the wire.
const statusBatchScriptHeader = "# ghsr-batch-status-v1"

const (
	nativeStatusBatchTag    = "@@ns:"
	containerStatusBatchTag = "@@cs:"
)

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
// instance in one round trip. ok=false means the caller must fall back to the
// per-instance probes (SSH error, or the script's output missed an instance).
func statusNativeBatch(h *host.Host, instances []string) ([]statusBatchResult, bool) {
	script, ok := buildNativeStatusBatchScript(h, instances)
	if !ok {
		return nil, false
	}
	out, err := h.Run(script)
	if err != nil {
		return nil, false
	}
	return parseStatusBatch(out, nativeStatusBatchTag, len(instances), parseNativeStatusTail)
}

// containerStatusBatch answers Manager.Status's container probes for every
// instance in one round trip. ok=false means fall back to the per-instance
// containerLocalStatusOneShot probes.
func containerStatusBatch(h *host.Host, instances []string) ([]statusBatchResult, bool) {
	out, err := h.Run(buildContainerStatusBatchScript(h, instances))
	if err != nil {
		return nil, false
	}
	return parseStatusBatch(out, containerStatusBatchTag, len(instances), parseContainerStatusTail)
}

// parseStatusBatch decodes one batch stream into n per-instance results.
// Lines with a foreign tag, an out-of-range index, or a duplicate index are
// ignored (stray remote output must not corrupt a row); a missing instance
// index fails the whole batch so the caller falls back rather than shows a
// stale/blank row.
func parseStatusBatch(out, tag string, n int, parseTail func(tail string, r *statusBatchResult)) ([]statusBatchResult, bool) {
	results := make([]statusBatchResult, n)
	seen := make([]bool, n)
	for d := range probe.Lines(out) {
		if !d.OK || d.Tag != tag {
			continue
		}
		if d.Index < 1 || d.Index > n || seen[d.Index-1] {
			continue
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

// buildNativeStatusBatchScript renders one remote command that answers the
// full native Linux status state machine for every instance, mirroring
// linuxInstanceProbe + statusNativeState:
//
//  1. Version: `cat $DIR/.runner-version`, whitespace collapsed to single
//     words (mirroring the space-join the V-marker parser applies to
//     multi-line versions; runner versions are single-word in practice).
//  2. svc.sh branch: when $DIR/svc.sh and $DIR/.service exist, the systemctl
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
//     no .runner_pid → stopped, pid readable and kill -0 ok → running,
//     pid unreadable → unknown, pid dead → stopped.
//
// The marker only ever carries a mapped state (running/stopped/service
// error/not installed/unknown — no pipes or newlines), so multi-line raw
// systemctl output like "inactive\ninactive" cannot split the marker line:
// it maps to stopped before the emit, exactly as the Go TrimSpace+compare
// does. Instance names embed raw, matching linuxInstanceProbe and the disk
// batch (config-validated names; $DIR keeps its literal $HOME prefix so the
// remote shell expands it).
func buildNativeStatusBatchScript(h *host.Host, instances []string) (string, bool) {
	bases := make([]string, len(instances))
	for i, inst := range instances {
		san, err := autostart.SanitizeInstance(inst)
		if err != nil {
			// The per-instance probes handle this via their own sanitize
			// error path; let the caller fall back to them.
			return "", false
		}
		bases[i] = autostart.ServiceBasename(san)
	}

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

	w := probe.NewWriter(&b)
	for i, inst := range instances {
		dir := h.RunnerDir(inst)
		base := bases[i]
		userPath := "\"$HOME/.config/systemd/user/" + base + ".service\""
		sysPath := "/etc/systemd/system/" + base + ".service"

		fmt.Fprintf(&b, "st=''; src=''\n")
		fmt.Fprintf(&b, "v=$(cat %s/.runner-version 2>/dev/null || true)\n", dir)
		fmt.Fprintf(&b, "v=$(printf '%%s ' $v); v=${v%% }\n")
		fmt.Fprintf(&b, "if [ -f %s/svc.sh ] && svc_file=\"%s/.service\" && [ -f \"$svc_file\" ]; then\n", dir, dir)
		b.WriteString("  st=$(systemctl is-active \"$(cat \"$svc_file\")\" 2>/dev/null || echo inactive); src=svc\n")
		b.WriteString("fi\n")
		fmt.Fprintf(&b, "if [ -z \"$st\" ] && [ -f %s ]; then\n", userPath)
		fmt.Fprintf(&b, "  st=$(systemctl --user is-active %s.service 2>/dev/null || echo inactive); src=kind\n", base)
		fmt.Fprintf(&b, "elif [ -z \"$st\" ] && [ -f %s ] && [ \"$ce\" = 1 ]; then\n", sysPath)
		fmt.Fprintf(&b, "  st=$($SUDO systemctl is-active %s.service 2>/dev/null || echo inactive); src=kind\n", base)
		b.WriteString("fi\n")
		b.WriteString("rstate=''\n")
		// The two branches map raw states differently, mirroring
		// statusNativeState: the svc.sh branch turns any non-active state
		// into stopped, while the autostart branch only recognises
		// active/failed/activating and leaves everything else (inactive, …)
		// empty so the PID-file fallback decides.
		b.WriteString("if [ -n \"$st\" ]; then\n")
		b.WriteString("  if [ \"$st\" = active ]; then rstate=running\n")
		b.WriteString("  elif [ \"$src\" = svc ]; then rstate=stopped\n")
		b.WriteString("  elif [ \"$st\" = failed ] || [ \"$st\" = activating ]; then rstate='service error'\n")
		b.WriteString("  fi\n")
		b.WriteString("fi\n")
		fmt.Fprintf(&b, "if [ -z \"$rstate\" ]; then\n")
		fmt.Fprintf(&b, "  if [ ! -f %s/.runner ]; then rstate='not installed'\n", dir)
		fmt.Fprintf(&b, "  elif [ ! -f %s/.runner_pid ]; then rstate=stopped\n", dir)
		fmt.Fprintf(&b, "  elif pid=$(cat %s/.runner_pid 2>/dev/null) && kill -0 \"$pid\" 2>/dev/null; then rstate=running\n", dir)
		b.WriteString("  elif [ -n \"$pid\" ]; then rstate=stopped\n")
		b.WriteString("  else rstate=unknown; fi\n")
		b.WriteString("fi\n")
		w.Echo(nativeStatusBatchTag, strconv.Itoa(i+1), "$rstate|$v")
	}
	return b.String(), true
}

// buildContainerStatusBatchScript renders one remote command that runs the
// containerLocalStatusOneShot state machine for every instance and tags each
// result line with its 1-based instance index. The per-instance shell is the
// one-shot script verbatim plus the tagged emit, so the payloads ("not
// installed|||", "failed|||", "status|image|digest|rev") parse identically.
func buildContainerStatusBatchScript(h *host.Host, instances []string) string {
	var b strings.Builder
	b.WriteString(statusBatchScriptHeader + "\n")
	w := probe.NewWriter(&b)
	for i, inst := range instances {
		fmt.Fprintf(&b, "sd=\"$HOME/.gh-sr/runners/%s\"\n", inst)
		fmt.Fprintf(&b, "cid=%s\n", hostshell.PosixSingleQuote(containerName(inst)))
		b.WriteString("if [ -f \"$sd/bootstrap-failed\" ]; then override=failed; else override=; fi\n")
		b.WriteString("line=$(docker inspect --format '{{.State.Status}}|{{.Config.Image}}|{{.Image}}' \"$cid\" 2>/dev/null) || line=\"\"\n")
		b.WriteString("if [ -z \"$line\" ]; then\n")
		b.WriteString("  if [ -n \"$override\" ]; then st='failed|||'; else st='not installed|||'; fi\n")
		b.WriteString("else\n")
		b.WriteString("  digest=${line##*|}\n")
		b.WriteString("  rev=''\n")
		b.WriteString("  if [ -n \"$digest\" ]; then\n")
		b.WriteString("    rev=$(docker image inspect \"$digest\" --format '{{index .Config.Labels \"gh-sr.image-revision\"}}' 2>/dev/null || true)\n")
		b.WriteString("  fi\n")
		b.WriteString("  if [ -n \"$override\" ]; then st=\"$override|${line#*|}|$rev\"\n")
		b.WriteString("  else st=\"$line|$rev\"\n")
		b.WriteString("  fi\n")
		b.WriteString("fi\n")
		w.Echo(containerStatusBatchTag, strconv.Itoa(i+1), "$st")
	}
	return b.String()
}
