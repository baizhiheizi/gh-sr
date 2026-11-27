package doctor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/an-lee/gh-sr/internal/agentic"
	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/dockerutil"
	"github.com/an-lee/gh-sr/internal/fanout"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/sharedcache"
)

const (
	sevOK   = "OK"
	sevWarn = "WARN"
	sevFail = "FAIL"
)

// Result aggregates doctor check outcomes for exit code decisions.
type Result struct {
	Fail int
	Warn int
}

// ExitCode returns a process exit code: 1 if any FAIL, or if strict and any WARN.
func ExitCode(res Result, strict bool) int {
	if res.Fail > 0 {
		return 1
	}
	if strict && res.Warn > 0 {
		return 1
	}
	return 0
}

// RunInput carries everything Run needs about the calling process's wiring:
// where the config and env files live, what was loaded from them (Config +
// ConfigErr, the config.LoadFromPath contract), the GitHub client for
// API-backed checks, and the run's scoping flags. Callers assemble it once —
// session.Session.DoctorInput is the production assembler — instead of
// re-listing ten positional parameters per call site.
type RunInput struct {
	ConfigPath     string
	EnvPath        string
	Config         *config.Config
	ConfigErr      error
	GitHub         *runner.GitHubClient
	FilterHost     string
	FilterRepo     string
	Strict         bool
	CheckLockfiles bool
}

// Run prints diagnostics to w. Config and ConfigErr come from config.LoadFromPath
// (or session.Load) after BootstrapEnv. If Config is nil (load error), GitHub and
// host checks are skipped after the configuration section.
// CheckLockfiles additionally scans compiled gh-aw workflows (*.lock.yml) in the
// scoped repos for retired sudo/iptables-era artifacts (GitHub API reads).
func Run(w io.Writer, in RunInput) Result {
	var r Result

	fmt.Fprintln(w, "=== Local environment ===")

	if _, err := os.Stat(in.ConfigPath); err != nil {
		r.failf(w, "local", "config path: %v", err)
	} else {
		printLine(w, sevOK, "local", fmt.Sprintf("config file exists: %s", in.ConfigPath))
	}

	switch _, err := os.Stat(in.EnvPath); {
	case os.IsNotExist(err):
		r.warnf(w, "local", "env file not found: %s (optional)", in.EnvPath)
	case err != nil:
		r.warnf(w, "local", "env file: %v", err)
	default:
		printLine(w, sevOK, "local", fmt.Sprintf("env file present: %s", in.EnvPath))
	}

	needSSH := in.Config == nil
	if in.Config != nil {
		for _, rc := range config.FilterRunners(in.Config, in.FilterHost, in.FilterRepo, nil) {
			hc := in.Config.Hosts[rc.Host]
			if !config.IsLocalAddr(hc.Addr) {
				needSSH = true
				break
			}
		}
	}
	if needSSH {
		if host.HasSSHAuth() {
			printLine(w, sevOK, "local", "SSH client auth available (agent or default ~/.ssh keys)")
		} else {
			r.warnf(w, "local", "no SSH agent (SSH_AUTH_SOCK) or default ~/.ssh keys; required for remote hosts")
		}
	} else {
		printLine(w, sevOK, "local", "only local hosts in scope; SSH client keys not required")
	}

	fmt.Fprintln(w, "\n=== Configuration ===")
	if in.ConfigErr != nil {
		r.failf(w, "config", "%s", in.ConfigErr.Error())
		printSummary(w, r, in.Strict)
		return r
	}
	printLine(w, sevOK, "config", "YAML valid and constraints satisfied")

	runners := config.FilterRunners(in.Config, in.FilterHost, in.FilterRepo, nil)
	if len(runners) == 0 {
		r.warnf(w, "config", "no runners match --host / --repo filters")
		printSummary(w, r, in.Strict)
		return r
	}

	fmt.Fprintln(w, "\n=== GitHub API ===")
	if in.GitHub == nil {
		r.failf(w, "github", "skipped: no GitHub token (run `gh auth login`)")
	} else {
		repos := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Repo })
		orgs := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Org })

		repoResults := make([]apiResult, len(repos))
		orgResults := make([]apiResult, len(orgs))

		var apiWg sync.WaitGroup
		checkRunnerScope(&apiWg, in.GitHub, "repo", repos, repoResults,
			func(name string, err error) string { return fmt.Sprintf("%s: %v", name, err) },
			func(name string, n int) string { return fmt.Sprintf("%s: list runners OK (%d registered)", name, n) },
		)
		checkRunnerScope(&apiWg, in.GitHub, "org", orgs, orgResults,
			formatOrgAPIError,
			func(name string, n int) string {
				return fmt.Sprintf("org %s: list runners OK (%d registered)", name, n)
			},
		)
		apiWg.Wait()

		printAPIFailures(w, &r, repoResults)
		printAPIFailures(w, &r, orgResults)
	}

	if in.CheckLockfiles {
		fmt.Fprintln(w, "\n=== Compiled agentic workflows (--check-lockfiles) ===")
		if in.GitHub == nil {
			r.warnf(w, "lockfiles", "skipped: no GitHub token (run `gh auth login`)")
		} else {
			repos := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Repo })
			checkLockWorkflows(w, in.GitHub, repos, &r)
		}
	}

	fmt.Fprintln(w, "\n=== Hosts ===")
	hostOrder := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Host })
	cacheSet := cache.SettingsFromConfig(in.Config)
	cacheEnabled := cacheSet != nil
	mirrorSet := registry.SettingsFromConfig(in.Config)

	type hostResult struct {
		buf bytes.Buffer
		r   Result
	}
	// One goroutine per host; each writes into its own buffer, so the results
	// need no synchronisation beyond the fan-out's own ordering.
	hostResults := fanout.Indexed(hostOrder, func(_ int, hostName string) *hostResult {
		hr := &hostResult{}
		hcfg := in.Config.Hosts[hostName]

		h := host.NewHost(hostName, hcfg)
		if err := h.Connect(); err != nil {
			printLine(&hr.buf, sevFail, hostName, fmt.Sprintf("connect: %v", err))
			hr.r.Fail++
			return hr
		}
		if err := ensureDoctorHostOS(h, hcfg.Addr); err != nil {
			printLine(&hr.buf, sevFail, hostName, fmt.Sprintf("detect os: %v", err))
			hr.r.Fail++
			_ = h.Close()
			return hr
		}

		defer h.Close()
		printLine(&hr.buf, sevOK, hostName, fmt.Sprintf("connected (%s)", addrSummary(hcfg.Addr)))
		runHostChecks(&hr.buf, hostName, h, runners, &hr.r, cacheSet, cacheEnabled, mirrorSet, containerExtras{
			NPMRegistry:  in.Config.NPMRegistry,
			SharedCaches: sharedcache.Enabled(in.Config.SharedCaches),
		})
		return hr
	})

	for i := range hostOrder {
		hr := hostResults[i]
		_, _ = io.Copy(w, &hr.buf)
		r.Fail += hr.r.Fail
		r.Warn += hr.r.Warn
	}

	printSummary(w, r, in.Strict)
	return r
}

func printLine(w io.Writer, sev, scope, msg string) {
	fmt.Fprintf(w, "%-5s [%-12s] %s\n", sev, scope, msg)
}

// warnf prints a WARN line for scope and counts it on r; failf is its FAIL
// counterpart. Every non-OK line the doctor emits goes through one of these,
// so a line and its counter cannot drift apart.
func (r *Result) warnf(w io.Writer, scope, format string, args ...any) {
	printLine(w, sevWarn, scope, fmt.Sprintf(format, args...))
	r.Warn++
}

func (r *Result) failf(w io.Writer, scope, format string, args ...any) {
	printLine(w, sevFail, scope, fmt.Sprintf(format, args...))
	r.Fail++
}

func printSummary(w io.Writer, r Result, strict bool) {
	fmt.Fprintln(w, "\n--- Summary ---")
	fmt.Fprintf(w, "%d failed, %d warnings", r.Fail, r.Warn)
	if strict {
		fmt.Fprint(w, " (strict: warnings fail the run)")
	}
	fmt.Fprintln(w)
}

func addrSummary(addr string) string {
	if config.IsLocalAddr(addr) {
		return "local"
	}
	return "ssh " + addr
}

// ensureDoctorHostOS sets h.OS when empty: local hosts use runtime.GOOS; remote hosts use DetectOS over the existing connection.
func ensureDoctorHostOS(h *host.Host, addr string) error {
	if h.OS != "" {
		return nil
	}
	if config.IsLocalAddr(addr) {
		h.OS = runtime.GOOS
		return nil
	}
	detected, err := host.DetectOS(h)
	if err != nil {
		return err
	}
	h.OS = detected
	return nil
}

// uniqueStringsBy returns a sorted, deduplicated list of the strings produced
// by applying key to each runner. Empty keys are skipped so callers don't
// need a guard for the absent-Repo / absent-Org case.
func uniqueStringsBy(runners []config.RunnerConfig, key func(config.RunnerConfig) string) []string {
	seen := make(map[string]struct{})
	for _, rc := range runners {
		k := key(rc)
		if k == "" {
			continue
		}
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// formatOrgAPIError formats a GitHub org runner API failure with a permission
// hint when the error looks like missing org admin access.
func formatOrgAPIError(org string, err error) string {
	msg := fmt.Sprintf("org %s: %v", org, err)
	if err == nil {
		return msg
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "http 403") || strings.Contains(lower, "permission") || strings.Contains(lower, "forbidden") {
		msg += " (org-level runners require org owner access or admin:org scope — run `gh auth login` with sufficient org permissions)"
	}
	return msg
}

// apiResult is one (severity, message) pair produced by the GitHub-API
// section of Run. Shared by the repo and org checks so both can flow through
// the same waitgroup / print / counter-update plumbing.
type apiResult struct {
	sev string
	msg string
}

// checkRunnerScope runs gh.ListRunnersScoped in parallel for each name in
// names and fills results[i] with the (sev, msg) pair produced by errMsg /
// okMsg. The caller owns the WaitGroup so it can fan both the repo and org
// scopes out on the same wg before waiting once. errMsg and okMsg are passed
// per scope because the repo and org surfaces use different wording (plain
// "name: err" vs formatOrgAPIError; no prefix vs "org " prefix).
func checkRunnerScope(wg *sync.WaitGroup, gh *runner.GitHubClient, scope string, names []string, results []apiResult, errMsg func(string, error) string, okMsg func(string, int) string) {
	for i, name := range names {
		wg.Add(1)
		go func(idx int, name string) {
			defer wg.Done()
			list, err := gh.ListRunnersScoped(scope, name)
			if err != nil {
				results[idx] = apiResult{sevFail, errMsg(name, err)}
			} else {
				results[idx] = apiResult{sevOK, okMsg(name, len(list))}
			}
		}(i, name)
	}
}

// printAPIFailures writes each apiResult to w as a `github` line and bumps
// r.Fail for every FAIL entry. Used for both the repo and org slices in Run
// so the print + counter-update lives in one place.
func printAPIFailures(w io.Writer, r *Result, results []apiResult) {
	for _, res := range results {
		printLine(w, res.sev, "github", res.msg)
		if res.sev == sevFail {
			r.Fail++
		}
	}
}

// installTargetsForHost lists (instanceName, runnerConfigName) pairs for
// runners on hostName that satisfy predicate. The predicate is the single
// place the caller expresses mode/profile policy (native vs container vs
// container+agentic vs any future bucket).
func installTargetsForHost(runners []config.RunnerConfig, hostName string, predicate func(*config.RunnerConfig) bool) [][2]string {
	var out [][2]string
	for i := range runners {
		rc := &runners[i]
		if rc.Host != hostName || !predicate(rc) {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			out = append(out, [2]string{inst, rc.Name})
		}
	}
	return out
}

// runnersForHost returns filtered runners assigned to hostName.
func runnersForHost(runners []config.RunnerConfig, hostName string) []config.RunnerConfig {
	var out []config.RunnerConfig
	for _, rc := range runners {
		if rc.Host == hostName {
			out = append(out, rc)
		}
	}
	return out
}

func hasNativeModeRunners(runners []config.RunnerConfig) bool {
	for _, rc := range runners {
		if !rc.IsContainerMode() {
			return true
		}
	}
	return false
}

func hasContainerModeRunners(runners []config.RunnerConfig) bool {
	for _, rc := range runners {
		if rc.IsContainerMode() {
			return true
		}
	}
	return false
}

// firstContainerInstance returns the name of the first instance belonging to a
// container-mode runner in the supplied slice, or "" if none are configured.
func firstContainerInstance(runners []config.RunnerConfig) string {
	for i := range runners {
		if runners[i].IsContainerMode() && len(runners[i].InstanceNames()) > 0 {
			return runners[i].InstanceNames()[0]
		}
	}
	return ""
}

// runnerContainerHasEnv checks whether the specified runner container carries
// an environment variable starting with the given key (e.g. "GH_SR_NPM_REGISTRY").
func runnerContainerHasEnv(h *host.Host, cname, key string) (bool, error) {
	out, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{range .Config.Env}}{{println .}}{{end}}' %s 2>/dev/null | grep -c '^%s=' || true",
		runner.QuoteContainerName(cname), key))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "0", nil
}

// runHostChecks runs native and/or container doctor checks for hostName using the
// already-filtered runners slice (respects --host / --repo filters). cacheSet is
// the effective per-host cache configuration (nil = disabled); cacheEnabled is
// passed through so agentic container probes include the cache-env check only
func runHostChecks(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result, cacheSet *cache.Settings, cacheEnabled bool, mirrorSet *registry.Settings, extras containerExtras) {
	hostRunners := runnersForHost(runners, hostName)
	if h.OS == "linux" {
		checkHostDNS(w, hostName, h, r)
	}
	if hasNativeModeRunners(hostRunners) {
		checkNative(w, hostName, h, r)
		checkNativeRunnerInstall(w, hostName, h, runners, r)
		if h.OS == "linux" {
			checkLinuxSudo(w, hostName, h, r)
		}
	}
	if hasContainerModeRunners(hostRunners) {
		checkContainerHostPrereqs(w, hostName, h, r)
		if h.OS == "linux" {
			checkContainerRunnerInstall(w, hostName, h, runners, r)
			if hasContainerAgenticRunners(hostRunners) {
				checkContainerAgenticInnerHygiene(w, hostName, h, runners, r, cacheEnabled)
			}
			checkCacheReachability(w, hostName, h, hostRunners, cacheSet, r)
			checkRegistryMirror(w, hostName, h, hostRunners, mirrorSet, r)
			checkContainerExtras(w, hostName, h, hostRunners, extras, r)
		}
	}
	if h.OS == "linux" || h.OS == "darwin" || h.OS == "windows" {
		checkOrphanRunners(w, hostName, h, runners, r)
	}
	checkRunnerDiskUsage(w, hostName, h, runners, r)
}

// containerExtras carries the container-create-time options doctor cannot
// derive from the cache/registry settings: the npm registry override and the
// enabled shared-cache rows (runners.yml npm_registry / shared_caches). The
// rows come from the sharedcache table, so the mount expectations checked
// below are derived from the same definition the container module mounts
// from.
type containerExtras struct {
	NPMRegistry  string
	SharedCaches []sharedcache.Cache
}

// checkContainerExtras verifies the first container-mode runner on the host
// actually carries the npm-registry env and the shared-cache volume mounts.
// Both are baked at `docker create`, so containers created before the option
// was enabled silently keep the old behavior (no mirror, private cold caches)
// until recreated — the same drift class probeRegistryFromRunner guards
// against for GH_SR_REGISTRY_MIRROR.
func checkContainerExtras(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, extras containerExtras, r *Result) {
	if len(extras.SharedCaches) == 0 && extras.NPMRegistry == "" {
		return
	}
	inst := firstContainerInstance(hostRunners)
	if inst == "" {
		return
	}
	cname := runner.ContainerDockerName(inst)

	var problems []string
	if extras.NPMRegistry != "" {
		if hasEnv, err := runnerContainerHasEnv(h, cname, "GH_SR_NPM_REGISTRY"); err == nil && !hasEnv {
			problems = append(problems, fmt.Sprintf("no GH_SR_NPM_REGISTRY env (npm_registry set); npm keeps using registry.npmjs.org"))
		}
	}
	var wantMounts []string
	for _, c := range extras.SharedCaches {
		wantMounts = append(wantMounts, c.Volume+" "+c.Mount)
	}
	if len(wantMounts) > 0 {
		mountOut, err := h.Run(fmt.Sprintf(
			"docker inspect --format='{{range .Mounts}}{{println .Name .Destination}}{{end}}' %s 2>/dev/null",
			runner.QuoteContainerName(cname)))
		if err == nil {
			for _, want := range wantMounts {
				if !strings.Contains(mountOut, want) {
					problems = append(problems, fmt.Sprintf("missing shared volume %s", want))
				}
			}
		}
	}
	if len(problems) == 0 {
		printLine(w, sevOK, hostName, "extras: npm registry + shared caches wired into runner containers")
		return
	}
	for _, p := range problems {
		r.warnf(w, hostName, "extras: runner container %s predates the config (%s); recreate with: gh sr rebuild %s", cname, p, inst)
	}
}

// checkHostDNS probes host-side resolution of api.github.com on a Linux host.
//
// A host whose stub resolver is wedged (systemd-resolved accepting connections
// but never answering, a stale dnsmasq, a blackholed upstream) passes every
// other doctor row: runner directories exist, docker is healthy, the published
// cache/registry ports answer on their IP literals. It only surfaces
// downstream as "missing .runner" on the runners registered from that host —
// container entrypoints fail `config.sh` on the resolve and crash-loop while
// the host-side rows stay green — which sends remediation at `gh sr setup`
// instead of at the resolver that is actually broken.
//
// getent is wrapped in `timeout`: a wedged resolver does not answer, it hangs,
// and an unbounded probe would stall the whole doctor run behind the SSH
// session rather than reporting a row.
func checkHostDNS(w io.Writer, hostName string, h *host.Host, r *Result) {
	out, err := h.Run(`r=$(timeout 5 getent hosts api.github.com 2>/dev/null); [ -n "$r" ] && echo ok || echo noresolve`)
	if err != nil {
		r.warnf(w, hostName, "dns: could not check name resolution: %v", err)
		return
	}
	switch strings.TrimSpace(out) {
	case "ok":
		printLine(w, sevOK, hostName, "dns: resolves api.github.com")
	default:
		r.failf(w, hostName, "dns: host cannot resolve api.github.com; runner registration and job-time network access will fail until the host resolver is fixed (e.g. sudo systemctl restart systemd-resolved), then re-run: gh sr setup")
	}
}

func checkNativeRunnerInstall(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	for _, pair := range installTargetsForHost(runners, hostName, func(rc *config.RunnerConfig) bool { return !rc.IsContainerMode() }) {
		inst, runnerName := pair[0], pair[1]
		dir := h.RunnerDir(inst)
		ok, err := runner.NativeRunnerConfigPresent(h, inst)
		if err != nil {
			r.failf(w, hostName, "native: instance %s: %v", inst, err)
			continue
		}
		if !ok {
			r.failf(w, hostName, "native: instance %s not installed (missing .runner under %s); run: gh sr setup %s", inst, dir, runnerName)
			continue
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("native: instance %s installed", inst))
	}
}

// checkShellOK runs cmd on h and reports whether trimmed stdout equals want.
// On success it prints okMsg with sevOK; on failure it prints failMsg with
// sevFail (including the underlying err in parentheses), increments r.Fail,
// and returns false. Used by checkNative to collapse the repeated
// run+trim+check idiom across the linux and darwin branches.
func checkShellOK(w io.Writer, hostName string, h *host.Host, r *Result, cmd, want, okMsg, failMsg string) bool {
	out, err := h.Run(cmd)
	out = strings.TrimSpace(out)
	if err != nil || out != want {
		r.failf(w, hostName, "%s (%v)", failMsg, err)
		return false
	}
	printLine(w, sevOK, hostName, okMsg)
	return true
}

func checkNative(w io.Writer, hostName string, h *host.Host, r *Result) {
	switch h.OS {
	case "linux":
		checkShellOK(w, hostName, h, r,
			`if command -v curl >/dev/null 2>&1 && command -v tar >/dev/null 2>&1; then echo ok; else echo missing; fi`,
			"ok",
			"native: curl and tar present",
			"native: need curl and tar on PATH")
	case "darwin":
		checkShellOK(w, hostName, h, r,
			`command -v curl >/dev/null 2>&1 && echo ok || echo missing`,
			"ok",
			"native: curl present",
			"native: need curl")
	case "windows":
		out, err := h.RunShell(`$PSVersionTable.PSVersion.ToString()`)
		out = strings.TrimSpace(out)
		if err != nil || out == "" {
			r.failf(w, hostName, "native: PowerShell check failed (%v)", err)
			return
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("native: PowerShell %s", out))
	default:
		r.warnf(w, hostName, "native: unknown os %q", h.OS)
	}
}

// linuxSudoUIDErrToken marks a failed `id -u` in the folded probe's output.
const linuxSudoUIDErrToken = "uiderr"

// linuxSudoProbeScript folds the uid gate and the sudo probe into one SSH
// round trip: separately they cost two remote invocations for information one
// remote shell can produce in a single pass.
//
// It writes exactly one token on stdout:
//
//	"root"   — the remote user is root, so no sudo probe is needed
//	"ok"     — non-root with passwordless sudo
//	"no"     — non-root without passwordless sudo
//	"uiderr" — `id -u` itself failed; the rest of the line is id's stderr
//
// The uiderr token keeps the pre-fold diagnostic. The two-probe code ran
// `id -u` on its own, so a failing id (exit 127 when id is not on PATH, exit
// 1 when the SSH account has no passwd entry) surfaced as "could not check
// uid" and the sudo probe never ran. Folding the probes into one script
// would otherwise swallow that failure — `$( )` discards the command's exit
// status — and report the misleading "passwordless sudo not available" for a
// host whose real problem is an unresolvable account.
const linuxSudoProbeScript = `u=$(id -u 2>&1) || { echo "uiderr $u"; exit 0; }; [ "$u" = "0" ] && echo root || { sudo -n true 2>/dev/null && echo ok || echo no; }`

// checkLinuxSudo runs the folded uid+sudo probe (linuxSudoProbeScript) and
// maps its tokens back to the two-probe contract this replaced: "root"
// returns silently, "ok"/"no" map to the OK/WARN lines, "uiderr" maps to
// the uid-check failure, and a transport error still reports the uid-check
// failure rather than a misleading missing-sudo warning.
func checkLinuxSudo(w io.Writer, hostName string, h *host.Host, r *Result) {
	out, err := h.Run(linuxSudoProbeScript)
	out = strings.TrimSpace(out)
	switch {
	case err != nil:
		r.warnf(w, hostName, "linux: could not check uid: %v", err)
	case strings.HasPrefix(out, linuxSudoUIDErrToken):
		r.warnf(w, hostName, "linux: could not check uid: %s", uidErrDetail(out))
	case out == "root":
		return
	case out != "ok":
		r.warnf(w, hostName, "linux: passwordless sudo not available; gh sr setup/update may fail for package installs")
	default:
		printLine(w, sevOK, hostName, "linux: non-root user has passwordless sudo")
	}
}

// uidErrDetail extracts id's own error text from the folded probe's
// "uiderr <detail>" output. id writes one line of stderr, but a shell
// "command not found" line carries its own prefix; only the first line is
// reported so the warning stays a single line.
func uidErrDetail(out string) string {
	detail := strings.TrimSpace(strings.TrimPrefix(out, linuxSudoUIDErrToken))
	if i := strings.IndexByte(detail, '\n'); i >= 0 {
		detail = strings.TrimSpace(detail[:i])
	}
	if detail == "" {
		return "id -u failed on the remote host"
	}
	return detail
}

// hasContainerAgenticRunners returns true if any runner uses agentic profile in container mode.
func hasContainerAgenticRunners(runners []config.RunnerConfig) bool {
	for _, rc := range runners {
		if rc.IsAgentic() && rc.IsContainerMode() {
			return true
		}
	}
	return false
}

// printAgenticFailures renders a slice of agentic failures against the doctor
// output, classifying each by severity and emitting remediation lines + doc-ref.
// defaultSev governs the severity for any failure whose Severity is neither
// SeverityError nor SeverityWarning (call sites use it to bias toward
// fail-by-default for host-prereqs vs. warn-by-default for inner-hygiene).
// Each failure increments either r.Fail or r.Warn as it is rendered.
func printAgenticFailures(w io.Writer, hostName string, r *Result, defaultSev, prefix string, failures []agentic.PrereqFailure) {
	for _, f := range failures {
		sev := defaultSev
		switch f.Severity {
		case agentic.SeverityError:
			sev = sevFail
			r.Fail++
		case agentic.SeverityWarning:
			sev = sevWarn
			r.Warn++
		default:
			if defaultSev == sevWarn {
				r.Warn++
			} else {
				r.Fail++
			}
		}
		printLine(w, sev, hostName, prefix+f.Message)
		if f.Remediation != "" {
			for line := range strings.SplitSeq(f.Remediation, "\n") {
				fmt.Fprintf(w, "       %s\n", line)
			}
		}
		if f.DocRef != "" {
			fmt.Fprintf(w, "       See: %s\n", f.DocRef)
		}
	}
}

// checkContainerHostPrereqs checks host requirements for runner_mode: container (DinD).
// The inner dockerd, dnsmasq, and iptables live inside the runner image; only the
// outer Docker daemon and --privileged support are checked here.
func checkContainerHostPrereqs(w io.Writer, hostName string, h *host.Host, r *Result) {
	failures := agentic.ValidateContainerPrereqs(h)

	if len(failures) == 0 {
		printLine(w, sevOK, hostName, "container: host Docker available and --privileged supported")
		return
	}

	printAgenticFailures(w, hostName, r, sevFail, "container: ", failures)
}

// checkContainerRunnerInstall verifies each DinD runner container exists on the host,
// is running (warn otherwise), inner dockerd responds, and .runner is present inside the image path.
func checkContainerRunnerInstall(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	targets := installTargetsForHost(runners, hostName, func(rc *config.RunnerConfig) bool { return rc.IsContainerMode() })
	if len(targets) == 0 {
		return
	}
	instances := make([]string, 0, len(targets))
	for _, pair := range targets {
		instances = append(instances, pair[0])
	}
	// One batched probe for the whole host. The per-instance chain this
	// replaces (state-dir resolve + marker test + docker inspect + docker
	// exec) cost four SSH round-trips per instance, so a 12-container host
	// spent ~48 of them on this single check.
	probes, err := runner.ProbeContainersBatch(h, instances)
	if err != nil {
		r.failf(w, hostName, "container: probe failed: %v", err)
		return
	}
	for _, pair := range targets {
		inst, runnerName := pair[0], pair[1]
		cname := runner.ContainerDockerName(inst)

		probe, reported := probes[inst]
		if !reported {
			// The batched probe prints one line per instance, so a missing
			// line means the remote script was cut short — report that
			// instead of claiming the container is gone.
			r.warnf(w, hostName, "container: instance %s (%s) — no probe result returned; re-run: gh sr doctor %s", inst, runnerName, hostName)
			continue
		}

		if probe.BootstrapFailed {
			r.failf(w, hostName, "container: instance %s (%s) — bootstrap failed (inner dockerd did not start after repeated attempts); fix the host then run: gh sr up %s (or gh sr rebuild %s)", inst, runnerName, runnerName, runnerName)
			continue
		}

		if probe.State == "missing" || probe.State == "" {
			r.failf(w, hostName, "container: instance %s (%s) — Docker container %s not found; run: gh sr setup %s", inst, runnerName, cname, runnerName)
			continue
		}
		if !runner.IsContainerAcceptingJobs(probe.State) {
			r.warnf(w, hostName, "container: instance %s (%s) — %s state is %q (expected running); run: gh sr up %s", inst, runnerName, cname, probe.State, runnerName)
			continue
		}

		if !probe.InnerDockerdOK {
			r.warnf(w, hostName, "container: instance %s — inner dockerd not responding inside %s", inst, cname)
		} else {
			printLine(w, sevOK, hostName, fmt.Sprintf("container: instance %s — inner dockerd healthy (%s)", inst, cname))
		}

		if !probe.Registered {
			// "run: gh sr setup" alone does not fix this. The registration
			// token is baked in at `docker create` (one-time use, ~1h TTL), and
			// EnsureSetup skips any runner whose container already exists — so
			// for a container that exists but never registered (typically after
			// a host DNS outage left it crash-looping on config.sh), setup
			// reports "already set up" and restarts the same stale token.
			// Tearing the instance down first forces a fresh token.
			r.failf(w, hostName, "container: instance %s — actions runner not configured inside %s (missing .runner); run: gh sr down %s && gh sr setup %s (the registration token is baked in at container create and is single-use, so the container must be recreated)", inst, cname, runnerName, runnerName)
			continue
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("container: instance %s — registered (.runner present in %s)", inst, cname))
	}
}

// checkContainerAgenticInnerHygiene runs agentic hygiene and inner-Docker prereq
// checks against the inner Docker in each running DinD agentic runner.
func checkContainerAgenticInnerHygiene(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result, cacheEnabled bool) {
	targets := installTargetsForHost(runners, hostName, func(rc *config.RunnerConfig) bool { return rc.IsAgentic() })
	if len(targets) == 0 {
		return
	}
	// Host egress MTU is host-level; detect once and reuse for every instance's MTU check.
	hostEgressMTU := runner.DetectHostEgressMTU(h)

	// One batched probe for the whole host. The per-instance chain this
	// replaces (state inspect + 2 hygiene probes + agentic fanout) cost four
	// SSH round-trips per instance, so an 8-agentic-container host spent ~32
	// of them on this single check.
	batch := make([]agentic.AgenticInnerTarget, len(targets))
	for i, pair := range targets {
		batch[i] = agentic.AgenticInnerTarget{Instance: pair[0], RunnerName: pair[1]}
	}
	probes, err := agentic.ValidateAgenticInnerBatch(h, batch, hostEgressMTU, cacheEnabled)
	if err != nil {
		r.failf(w, hostName, "container(agent): probe failed: %v", err)
		return
	}
	for i, pair := range targets {
		inst, runnerName := pair[0], pair[1]
		cname := runner.ContainerDockerName(inst)

		probe := probes[i]
		if !probe.Reported {
			// The batched probe prints one section per instance, so a missing
			// section means the remote script was cut short (or the instance
			// name was rejected before probing) — report that instead of
			// claiming the container is clean.
			r.warnf(w, hostName, "container(agent): instance %s (%s) — no probe result returned; re-run: gh sr doctor %s", inst, runnerName, hostName)
			continue
		}

		// Stricter than IsContainerAcceptingJobs: the inner agentic probes
		// need a fully-running container, not a transient "restarting"
		// state, so we require exactly "running".
		if probe.State != "running" {
			continue
		}

		if len(probe.Failures) == 0 {
			printLine(w, sevOK, hostName, fmt.Sprintf("container(agent): inner Docker clean, host-gateway alias resolvable, node/npm + zstd on PATH, socket user ok%s (%s)", cacheGateSummary(cacheEnabled), cname))
			continue
		}
		printAgenticFailures(w, hostName, r, sevWarn, "container(agent): ", probe.Failures)
	}
}

func cacheGateSummary(cacheEnabled bool) string {
	if cacheEnabled {
		return ", cache URL wired"
	}
	return ""
}

// checkCacheReachability verifies the per-host local cache server is deployed,
// healthy from the host, and — probed from inside a container-mode runner —
// actually reachable where it matters. Skipped entirely when the cache is
// disabled (runners then use GitHub's cache service).
func checkCacheReachability(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, s *cache.Settings, r *Result) {
	if s == nil {
		return
	}
	info, err := cache.Inspect(h, *s)
	if err != nil {
		r.warnf(w, hostName, "cache: inspect failed: %v", err)
		return
	}
	if info.State == "" {
		r.warnf(w, hostName, "cache: enabled but not deployed; run: gh sr cache deploy")
		return
	}
	if !info.Healthy {
		url := info.URL
		if url == "" {
			url = "(no container-reachable URL resolved)"
		}
		r.failf(w, hostName, "cache: server not healthy at %s; check the gh-sr-cache container logs: docker logs %s", url, cache.ContainerName)
		return
	}
	printLine(w, sevOK, hostName, fmt.Sprintf("cache: healthy at %s (storage %s)", info.URL, info.StoragePath))
	probeCacheFromRunner(w, hostName, h, hostRunners, s, r)

	// A 0.0.0.0 bind means the cache API answers on every host interface.
	if dockerutil.EffectiveBind(h, s.BindAddr) == "0.0.0.0" {
		r.warnf(w, hostName, "cache: bound to 0.0.0.0 — the cache API is reachable from your LAN; set cache.bind_addr (e.g. the docker0 gateway IP) in runners.yml")
	}
}

func probeFailed(out string) bool {
	return strings.Contains(out, "PROBE_FAIL")
}

func http200ProbeFailed(out string) bool {
	return strings.Contains(out, "PROBE_FAIL") || out != "200"
}

// probeFromRunnerContainer executes a shell probe inside a runner container via
// docker exec, checks for execution errors and probe failure conditions, and records
// the diagnostic outcome into the Result collector.
func probeFromRunnerContainer(
	w io.Writer,
	hostName string,
	h *host.Host,
	cname string,
	label string,
	script string,
	isFailed func(out string) bool,
	failSev string,
	failMsg func() string,
	okMsg string,
	r *Result,
) {
	out, err := h.Run(fmt.Sprintf(
		"docker exec %s sh -c %s 2>/dev/null",
		runner.QuoteContainerName(cname), hostshell.PosixSingleQuote(script)))
	switch {
	case err != nil:
		r.warnf(w, hostName, "%s: could not probe from runner container %s: %v", label, cname, err)
	case isFailed(strings.TrimSpace(out)):
		printLine(w, failSev, hostName, failMsg())
		if failSev == sevFail {
			r.Fail++
		} else {
			r.Warn++
		}
	default:
		printLine(w, sevOK, hostName, okMsg)
	}
}

// probeCacheFromRunner curls the cache health endpoint from inside the first
// container-mode runner instance on the host. The host reaches the published
// port through the loopback path even when the runner path is blocked — a
// host INPUT firewall with a default-deny policy (ufw et al) silently
// blackholes container→host-port traffic — which is exactly the failure mode
// that times out every upload-artifact and actions/cache step while the
// host-side check stays green. The inner command echoes PROBE_FAIL instead of
// failing so "curl cannot connect" is distinguishable from "container not
// running" (the latter is already reported by the runner checks and only
// warns here).
func probeCacheFromRunner(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, s *cache.Settings, r *Result) {
	inst := firstContainerInstance(hostRunners)
	if inst == "" {
		return
	}
	url := s.RunnerURL(h)
	if url == "" {
		return
	}
	cname := runner.ContainerDockerName(inst)
	// RunnerURL is a prefix and keeps its trailing slash, so the probe must
	// trim it before appending "/health": concatenating the two yields
	// "//health", which the cache server never answers — curl burns its full
	// -m budget and the check reports the host INPUT firewall as the cause of
	// a URL that is in fact reachable. The inner script is quoted once, as a
	// whole: quoting the URL separately nests single quotes inside the
	// already-quoted `sh -c` payload and silently rewrites the request.
	script := fmt.Sprintf("curl -fsS -m 5 %s/health || echo PROBE_FAIL", strings.TrimSuffix(url, "/"))
	probeFromRunnerContainer(w, hostName, h, cname, "cache", script, probeFailed, sevFail, func() string {
		bind := dockerutil.EffectiveBind(h, s.BindAddr)
		return fmt.Sprintf(
			"cache: not reachable from runner container %s at %s (host-side health is fine — the host INPUT firewall is blocking container traffic); allow it with: sudo ufw allow in on docker0 to %s port %d proto tcp comment 'allow-docker-cache' (or the equivalent nftables/iptables rule), then run: gh sr cache deploy && gh sr rebuild %s",
			cname, url, bind, s.EffectivePort(), inst)
	}, fmt.Sprintf("cache: reachable from runner container %s at %s", cname, url), r)
}

// checkRegistryMirror verifies every per-host Docker registry pull-through
// mirror is deployed and healthy from the host, that the first (Docker Hub)
// mirror is reachable from a container-mode runner and wired into the runner
// container's env (the inner dockerd only picks up registry-mirrors from
// daemon.json written at container start), and that prefetch-target mirrors
// are reachable from runners too. Skipped entirely when the mirror section is
// disabled (runners pull from Docker Hub directly).
func checkRegistryMirror(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, s *registry.Settings, r *Result) {
	if s == nil {
		return
	}
	mirrors, err := registry.Inspect(h, *s)
	if err != nil {
		r.warnf(w, hostName, "registry: inspect failed: %v", err)
		return
	}
	for i, m := range mirrors {
		// Single-mirror configs keep the historical "registry:" prefix;
		// multi-mirror output names each mirror.
		label := "registry"
		if len(mirrors) > 1 {
			label = "registry[" + m.Name + "]"
		}
		if m.Info.State == "" {
			r.warnf(w, hostName, "%s: enabled but not deployed; run: gh sr registry deploy", label)
			continue
		}
		if !m.Info.Healthy {
			url := m.Info.URL
			if url == "" {
				url = "(no container-reachable URL resolved)"
			}
			r.failf(w, hostName, "%s: mirror not healthy at %s; check the mirror container logs: docker logs %s", label, url, m.Container)
			continue
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("%s: mirror healthy at %s (storage %s)", label, m.Info.URL, m.Info.StoragePath))
		if i > 0 {
			// Non-first mirrors have no daemon.json wiring; a runner-side
			// reachability probe is all doctor can verify.
			probeMirrorURLFromRunner(w, hostName, h, hostRunners, m.Info.URL, label, r)
		}
	}
	probeRegistryFromRunner(w, hostName, h, hostRunners, s, r)
	checkOrphanMirrors(w, hostName, h, s, r)

	// A 0.0.0.0 bind means the mirror answers on every host interface.
	if dockerutil.EffectiveBind(h, s.BindAddr) == "0.0.0.0" {
		r.warnf(w, hostName, "registry: bound to 0.0.0.0 — the mirror is reachable from your LAN; set registry_mirror.bind_addr (e.g. the docker0 gateway IP) in runners.yml")
	}
}

// checkOrphanMirrors flags gh-sr-registry* containers on the host that the
// current registry_mirror.mirrors list no longer configures (an entry that was
// renamed or deleted). Both Ensure and `gh sr registry remove` iterate only
// the configured list, so an orphan keeps serving pulls nobody asked for and
// keeps its storage dir on disk, and neither gh sr disk usage nor
// `gh sr registry remove --purge-data` can reach it.
func checkOrphanMirrors(w io.Writer, hostName string, h *host.Host, s *registry.Settings, r *Result) {
	configured := make(map[string]struct{})
	for _, n := range s.ContainerNames() {
		configured[n] = struct{}{}
	}
	out, err := h.Run("docker ps -a --format '{{.Names}}' 2>/dev/null || true")
	if err != nil {
		return // Best-effort: never turn an unreadable container list into a FAIL.
	}
	var orphans []string
	for line := range strings.SplitSeq(out, "\n") {
		name := strings.TrimSpace(line)
		if name != registry.ContainerName && !strings.HasPrefix(name, registry.ContainerNamePrefix) {
			continue
		}
		if _, ok := configured[name]; !ok {
			orphans = append(orphans, name)
		}
	}
	sort.Strings(orphans)
	for _, name := range orphans {
		r.warnf(w, hostName, "registry: %s runs on this host but is not in registry_mirror.mirrors; remove it with: docker rm -f %s (its storage dir keeps occupying disk)", name, name)
	}
}

// probeRegistryFromRunner curls the mirror's /v2/ endpoint from inside the
// first container-mode runner on the host (same host-INPUT firewall failure
// mode probeCacheFromRunner guards against) and verifies the runner container
// actually carries GH_SR_REGISTRY_MIRROR — the env is baked at docker create,
// so containers created before the mirror was enabled keep pulling from
// Docker Hub until recreated.
func probeRegistryFromRunner(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, s *registry.Settings, r *Result) {
	inst := firstContainerInstance(hostRunners)
	if inst == "" {
		return
	}
	url := s.RunnerURL(h)
	if url == "" {
		return
	}
	cname := runner.ContainerDockerName(inst)
	if hasEnv, err := runnerContainerHasEnv(h, cname, "GH_SR_REGISTRY_MIRROR"); err == nil && !hasEnv {
		r.warnf(w, hostName, "registry: runner container %s predates the mirror (no GH_SR_REGISTRY_MIRROR env); recreate with: gh sr rebuild %s", cname, inst)
		return
	}
	script := fmt.Sprintf("curl -fsS -m 5 -o /dev/null -w %%{http_code} %s/v2/ || echo PROBE_FAIL", strings.TrimSuffix(url, "/"))
	probeFromRunnerContainer(w, hostName, h, cname, "registry", script, http200ProbeFailed, sevFail, func() string {
		bind := dockerutil.EffectiveBind(h, s.BindAddr)
		return fmt.Sprintf(
			"registry: not reachable from runner container %s at %s (host-side health is fine — the host INPUT firewall is blocking container traffic); allow it with: sudo ufw allow in on docker0 to %s port %d proto tcp comment 'allow-docker-registry-mirror' (or the equivalent nftables/iptables rule)",
			cname, url, bind, s.EffectivePort())
	}, fmt.Sprintf("registry: reachable from runner container %s at %s", cname, url), r)
}

// probeMirrorURLFromRunner curls one mirror's /v2/ endpoint from inside the
// first container-mode runner on the host. Used for the prefetch-only mirrors
// (index > 0): they have no daemon.json wiring to verify, but `gh sr images
// sync` still pulls through them from the host and a blocked INPUT chain is
// worth surfacing.
func probeMirrorURLFromRunner(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, url, label string, r *Result) {
	if url == "" {
		return
	}
	inst := firstContainerInstance(hostRunners)
	if inst == "" {
		return
	}
	cname := runner.ContainerDockerName(inst)
	script := fmt.Sprintf("curl -fsS -m 5 -o /dev/null -w %%{http_code} %s/v2/ || echo PROBE_FAIL", strings.TrimSuffix(url, "/"))
	probeFromRunnerContainer(w, hostName, h, cname, label, script, http200ProbeFailed, sevWarn, func() string {
		return fmt.Sprintf("%s: not reachable from runner container %s at %s (host INPUT firewall); not fatal — prefetch pulls run on the host", label, cname, url)
	}, fmt.Sprintf("%s: reachable from runner container %s at %s", label, cname, url), r)
}

func checkOrphanRunners(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	configured := runner.ConfiguredInstanceSet(runners, hostName)
	orphans, err := runner.OrphanInstances(h, configured)
	if err != nil {
		r.warnf(w, hostName, "orphan runners: list: %v", err)
		return
	}
	if len(orphans) == 0 {
		return
	}
	r.warnf(w, hostName, "orphan runners: %d instance(s) not in runners.yml (%s); run: gh sr service cleanup", len(orphans), strings.Join(orphans, ", "))
}

func checkRunnerDiskUsage(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	threshold := runner.DiskWarnThresholdBytes()
	inv := runner.HostInventoryFor(h, runners, hostName, true)
	if inv.ListErr != nil {
		// Degradation policy: still report the configured instances, but say
		// why the on-disk half (and any orphans) is missing instead of
		// silently dropping them.
		r.warnf(w, hostName, "disk: could not list on-disk runner directories: %v", inv.ListErr)
	}
	instances := inv.Names()
	rcByInstance := inv.ConfigByInstance()

	// One batched SSH round-trip per host replaces the prior per-instance
	// MeasureDiskUsage loop (N+1 fold; matches the win-class of PRs
	// #358/#361/#372/#380/#379).
	entries := runner.MeasureDiskUsageBatch(h, hostName, instances, rcByInstance)
	for _, inst := range instances {
		entry := entries[inst]
		if entry.Err != nil {
			r.warnf(w, hostName, "disk: instance %s: %v", inst, entry.Err)
			continue
		}
		if entry.TotalBytes >= threshold {
			label := inst
			if entry.Orphan {
				label = inst + " (orphan)"
			}
			r.warnf(w, hostName, "disk: instance %s uses %s under %s; run: gh sr disk prune --yes", label, runner.FormatBytesHuman(entry.TotalBytes), entry.Path)
		}
	}
}
