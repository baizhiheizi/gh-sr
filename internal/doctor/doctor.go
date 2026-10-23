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
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/runner"
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

// Run prints diagnostics to w. cfg and cfgErr come from config.LoadFromPath (or Load) after BootstrapEnv.
// If cfg is nil (load error), GitHub and host checks are skipped after the configuration section.
// checkLockfiles additionally scans compiled gh-aw workflows (*.lock.yml) in the
// scoped repos for retired sudo/iptables-era artifacts (GitHub API reads).
func Run(w io.Writer, cfgPath, envPath string, cfg *config.Config, cfgErr error, gh *runner.GitHubClient, filterHost, filterRepo string, strict, checkLockfiles bool) Result {
	var r Result

	fmt.Fprintln(w, "=== Local environment ===")

	if _, err := os.Stat(cfgPath); err != nil {
		printLine(w, sevFail, "local", fmt.Sprintf("config path: %v", err))
		r.Fail++
	} else {
		printLine(w, sevOK, "local", fmt.Sprintf("config file exists: %s", cfgPath))
	}

	switch _, err := os.Stat(envPath); {
	case os.IsNotExist(err):
		printLine(w, sevWarn, "local", fmt.Sprintf("env file not found: %s (optional)", envPath))
		r.Warn++
	case err != nil:
		printLine(w, sevWarn, "local", fmt.Sprintf("env file: %v", err))
		r.Warn++
	default:
		printLine(w, sevOK, "local", fmt.Sprintf("env file present: %s", envPath))
	}

	needSSH := cfg == nil
	if cfg != nil {
		for _, rc := range config.FilterRunners(cfg, filterHost, filterRepo, nil) {
			hc := cfg.Hosts[rc.Host]
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
			printLine(w, sevWarn, "local", "no SSH agent (SSH_AUTH_SOCK) or default ~/.ssh keys; required for remote hosts")
			r.Warn++
		}
	} else {
		printLine(w, sevOK, "local", "only local hosts in scope; SSH client keys not required")
	}

	fmt.Fprintln(w, "\n=== Configuration ===")
	if cfgErr != nil {
		printLine(w, sevFail, "config", cfgErr.Error())
		r.Fail++
		printSummary(w, r, strict)
		return r
	}
	printLine(w, sevOK, "config", "YAML valid and constraints satisfied")

	runners := config.FilterRunners(cfg, filterHost, filterRepo, nil)
	if len(runners) == 0 {
		printLine(w, sevWarn, "config", "no runners match --host / --repo filters")
		r.Warn++
		printSummary(w, r, strict)
		return r
	}

	fmt.Fprintln(w, "\n=== GitHub API ===")
	if gh == nil {
		printLine(w, sevFail, "github", "skipped: no GitHub token (run `gh auth login`)")
		r.Fail++
	} else {
		repos := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Repo })
		orgs := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Org })

		repoResults := make([]apiResult, len(repos))
		orgResults := make([]apiResult, len(orgs))

		var apiWg sync.WaitGroup
		checkRunnerScope(&apiWg, gh, "repo", repos, repoResults,
			func(name string, err error) string { return fmt.Sprintf("%s: %v", name, err) },
			func(name string, n int) string { return fmt.Sprintf("%s: list runners OK (%d registered)", name, n) },
		)
		checkRunnerScope(&apiWg, gh, "org", orgs, orgResults,
			formatOrgAPIError,
			func(name string, n int) string {
				return fmt.Sprintf("org %s: list runners OK (%d registered)", name, n)
			},
		)
		apiWg.Wait()

		printAPIFailures(w, &r, repoResults)
		printAPIFailures(w, &r, orgResults)
	}

	if checkLockfiles {
		fmt.Fprintln(w, "\n=== Compiled agentic workflows (--check-lockfiles) ===")
		if gh == nil {
			printLine(w, sevWarn, "lockfiles", "skipped: no GitHub token (run `gh auth login`)")
			r.Warn++
		} else {
			repos := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Repo })
			checkLockWorkflows(w, gh, repos, &r)
		}
	}

	fmt.Fprintln(w, "\n=== Hosts ===")
	hostOrder := uniqueStringsBy(runners, func(rc config.RunnerConfig) string { return rc.Host })
	cacheSet := cache.SettingsFromConfig(cfg)
	cacheEnabled := cacheSet != nil
	mirrorSet := registry.SettingsFromConfig(cfg)

	type hostResult struct {
		buf bytes.Buffer
		r   Result
	}
	hostResults := make([]hostResult, len(hostOrder))

	var hostWg sync.WaitGroup
	for i, hostName := range hostOrder {
		hostWg.Add(1)
		go func(idx int, hostName string) {
			defer hostWg.Done()
			hr := &hostResults[idx]
			hcfg := cfg.Hosts[hostName]

			h := host.NewHost(hostName, hcfg)
			if err := h.Connect(); err != nil {
				printLine(&hr.buf, sevFail, hostName, fmt.Sprintf("connect: %v", err))
				hr.r.Fail++
				return
			}
			if err := ensureDoctorHostOS(h, hcfg.Addr); err != nil {
				printLine(&hr.buf, sevFail, hostName, fmt.Sprintf("detect os: %v", err))
				hr.r.Fail++
				_ = h.Close()
				return
			}

			defer h.Close()
			printLine(&hr.buf, sevOK, hostName, fmt.Sprintf("connected (%s)", addrSummary(hcfg.Addr)))
			runHostChecks(&hr.buf, hostName, h, runners, &hr.r, cacheSet, cacheEnabled, mirrorSet, containerExtras{
				NPMRegistry:       cfg.NPMRegistry,
				SharedNPMCache:    cfg.SharedNPMCacheEnabled(),
				SharedRunnerCache: cfg.SharedHomeCacheEnabled(),
				SharedGradleCache: cfg.SharedGradleCacheEnabled(),
				SharedPubCache:    cfg.SharedPubCacheEnabled(),
				SharedMiseData:    cfg.SharedMiseDataEnabled(),
				SharedAndroidSDK:  cfg.SharedAndroidSDKEnabled(),
			})
		}(i, hostName)
	}
	hostWg.Wait()

	for i := range hostOrder {
		hr := &hostResults[i]
		_, _ = io.Copy(w, &hr.buf)
		r.Fail += hr.r.Fail
		r.Warn += hr.r.Warn
	}

	printSummary(w, r, strict)
	return r
}

func printLine(w io.Writer, sev, scope, msg string) {
	fmt.Fprintf(w, "%-5s [%-12s] %s\n", sev, scope, msg)
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
// shared cache volume switches (runners.yml npm_registry / shared_caches).
type containerExtras struct {
	NPMRegistry       string
	SharedNPMCache    bool
	SharedRunnerCache bool
	SharedGradleCache bool
	SharedPubCache    bool
	SharedMiseData    bool
	SharedAndroidSDK  bool
}

// checkContainerExtras verifies the first container-mode runner on the host
// actually carries the npm-registry env and the shared-cache volume mounts.
// Both are baked at `docker create`, so containers created before the option
// was enabled silently keep the old behavior (no mirror, private cold caches)
// until recreated — the same drift class probeRegistryFromRunner guards
// against for GH_SR_REGISTRY_MIRROR.
func checkContainerExtras(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, extras containerExtras, r *Result) {
	if !extras.SharedNPMCache && !extras.SharedRunnerCache && !extras.SharedGradleCache &&
		!extras.SharedPubCache && !extras.SharedMiseData && !extras.SharedAndroidSDK && extras.NPMRegistry == "" {
		return
	}
	inst := ""
	for i := range hostRunners {
		if hostRunners[i].IsContainerMode() && len(hostRunners[i].InstanceNames()) > 0 {
			inst = hostRunners[i].InstanceNames()[0]
			break
		}
	}
	if inst == "" {
		return
	}
	cname := runner.ContainerDockerName(inst)

	var problems []string
	if extras.NPMRegistry != "" {
		envOut, err := h.Run(fmt.Sprintf(
			"docker inspect --format='{{range .Config.Env}}{{println .}}{{end}}' %s 2>/dev/null | grep -c '^GH_SR_NPM_REGISTRY=' || true",
			runner.QuoteContainerName(cname)))
		if err == nil && strings.TrimSpace(envOut) == "0" {
			problems = append(problems, fmt.Sprintf("no GH_SR_NPM_REGISTRY env (npm_registry set); npm keeps using registry.npmjs.org"))
		}
	}
	var wantMounts []string
	if extras.SharedNPMCache {
		wantMounts = append(wantMounts, "gh-sr-npm-cache /home/runner/.npm")
	}
	if extras.SharedRunnerCache {
		wantMounts = append(wantMounts, "gh-sr-runner-cache /home/runner/.cache")
	}
	if extras.SharedGradleCache {
		wantMounts = append(wantMounts, "gh-sr-gradle-cache /home/runner/.gradle")
	}
	if extras.SharedPubCache {
		wantMounts = append(wantMounts, "gh-sr-pub-cache /home/runner/.pub-cache")
	}
	if extras.SharedMiseData {
		wantMounts = append(wantMounts, "gh-sr-mise-data /home/runner/.local/share/mise")
	}
	if extras.SharedAndroidSDK {
		wantMounts = append(wantMounts, "gh-sr-android-sdk /opt/android-sdk")
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
		printLine(w, sevWarn, hostName, fmt.Sprintf("extras: runner container %s predates the config (%s); recreate with: gh sr rebuild %s", cname, p, inst))
		r.Warn++
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
		printLine(w, sevWarn, hostName, fmt.Sprintf("dns: could not check name resolution: %v", err))
		r.Warn++
		return
	}
	switch strings.TrimSpace(out) {
	case "ok":
		printLine(w, sevOK, hostName, "dns: resolves api.github.com")
	default:
		printLine(w, sevFail, hostName, "dns: host cannot resolve api.github.com; runner registration and job-time network access will fail until the host resolver is fixed (e.g. sudo systemctl restart systemd-resolved), then re-run: gh sr setup")
		r.Fail++
	}
}

func checkNativeRunnerInstall(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	for _, pair := range installTargetsForHost(runners, hostName, func(rc *config.RunnerConfig) bool { return !rc.IsContainerMode() }) {
		inst, runnerName := pair[0], pair[1]
		dir := h.RunnerDir(inst)
		ok, err := runner.NativeRunnerConfigPresent(h, inst)
		if err != nil {
			printLine(w, sevFail, hostName, fmt.Sprintf("native: instance %s: %v", inst, err))
			r.Fail++
			continue
		}
		if !ok {
			printLine(w, sevFail, hostName, fmt.Sprintf("native: instance %s not installed (missing .runner under %s); run: gh sr setup %s", inst, dir, runnerName))
			r.Fail++
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
		printLine(w, sevFail, hostName, fmt.Sprintf("%s (%v)", failMsg, err))
		r.Fail++
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
			printLine(w, sevFail, hostName, fmt.Sprintf("native: PowerShell check failed (%v)", err))
			r.Fail++
			return
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("native: PowerShell %s", out))
	default:
		printLine(w, sevWarn, hostName, fmt.Sprintf("native: unknown os %q", h.OS))
		r.Warn++
	}
}

func checkLinuxSudo(w io.Writer, hostName string, h *host.Host, r *Result) {
	uid, err := h.Run(`id -u`)
	if err != nil {
		printLine(w, sevWarn, hostName, fmt.Sprintf("linux: could not check uid: %v", err))
		r.Warn++
		return
	}
	if strings.TrimSpace(uid) == "0" {
		return
	}
	out, err := h.Run(`sudo -n true 2>/dev/null && echo ok || echo no`)
	out = strings.TrimSpace(out)
	if err != nil || out != "ok" {
		printLine(w, sevWarn, hostName, "linux: passwordless sudo not available; gh sr setup/update may fail for package installs")
		r.Warn++
		return
	}
	printLine(w, sevOK, hostName, "linux: non-root user has passwordless sudo")
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
		printLine(w, sevFail, hostName, fmt.Sprintf("container: probe failed: %v", err))
		r.Fail++
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
			printLine(w, sevWarn, hostName, fmt.Sprintf("container: instance %s (%s) — no probe result returned; re-run: gh sr doctor %s", inst, runnerName, hostName))
			r.Warn++
			continue
		}

		if probe.BootstrapFailed {
			printLine(w, sevFail, hostName, fmt.Sprintf(
				"container: instance %s (%s) — bootstrap failed (inner dockerd did not start after repeated attempts); fix the host then run: gh sr up %s (or gh sr rebuild %s)",
				inst, runnerName, runnerName, runnerName))
			r.Fail++
			continue
		}

		if probe.State == "missing" || probe.State == "" {
			printLine(w, sevFail, hostName, fmt.Sprintf("container: instance %s (%s) — Docker container %s not found; run: gh sr setup %s", inst, runnerName, cname, runnerName))
			r.Fail++
			continue
		}
		if !runner.IsContainerAcceptingJobs(probe.State) {
			printLine(w, sevWarn, hostName, fmt.Sprintf("container: instance %s (%s) — %s state is %q (expected running); run: gh sr up %s", inst, runnerName, cname, probe.State, runnerName))
			r.Warn++
			continue
		}

		if !probe.InnerDockerdOK {
			printLine(w, sevWarn, hostName, fmt.Sprintf("container: instance %s — inner dockerd not responding inside %s", inst, cname))
			r.Warn++
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
			printLine(w, sevFail, hostName, fmt.Sprintf(
				"container: instance %s — actions runner not configured inside %s (missing .runner); run: gh sr down %s && gh sr setup %s (the registration token is baked in at container create and is single-use, so the container must be recreated)",
				inst, cname, runnerName, runnerName))
			r.Fail++
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

	for _, pair := range targets {
		inst := pair[0]
		runnerName := pair[1]
		cname := runner.ContainerDockerName(inst)

		out, err := runner.ContainerStateStatus(h, cname)
		status := out
		// Stricter than IsContainerAcceptingJobs: the inner agentic probes
		// below need a fully-running container, not a transient "restarting"
		// state, so we require exactly "running".
		if err != nil || status != "running" {
			continue
		}

		failures := agentic.ValidateAWFHygieneInner(h, cname)
		// Fan the inner-docker agentic prereq probes (InnerNetwork, NodeNPM,
		// DockerSocketUser, Zstd, gated CacheEnv/MTU) into a single
		// `docker exec` round-trip via ValidateContainerAgenticFanout.
		failures = append(failures, agentic.ValidateContainerAgenticFanout(h, cname, runnerName, hostEgressMTU, cacheEnabled)...)
		if len(failures) == 0 {
			printLine(w, sevOK, hostName, fmt.Sprintf("container(agent): inner Docker clean, host-gateway alias resolvable, node/npm + zstd on PATH, socket user ok%s (%s)", cacheGateSummary(cacheEnabled), cname))
			continue
		}
		printAgenticFailures(w, hostName, r, sevWarn, "container(agent): ", failures)
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
		printLine(w, sevWarn, hostName, fmt.Sprintf("cache: inspect failed: %v", err))
		r.Warn++
		return
	}
	if info.State == "" {
		printLine(w, sevWarn, hostName, "cache: enabled but not deployed; run: gh sr cache deploy")
		r.Warn++
		return
	}
	if !info.Healthy {
		url := info.URL
		if url == "" {
			url = "(no container-reachable URL resolved)"
		}
		printLine(w, sevFail, hostName, fmt.Sprintf("cache: server not healthy at %s; check the gh-sr-cache container logs: docker logs %s", url, cache.ContainerName))
		r.Fail++
		return
	}
	printLine(w, sevOK, hostName, fmt.Sprintf("cache: healthy at %s (storage %s)", info.URL, info.StoragePath))
	probeCacheFromRunner(w, hostName, h, hostRunners, s, r)

	// A 0.0.0.0 bind means the cache API answers on every host interface.
	if effectiveBind(s, h) == "0.0.0.0" {
		printLine(w, sevWarn, hostName, "cache: bound to 0.0.0.0 — the cache API is reachable from your LAN; set cache.bind_addr (e.g. the docker0 gateway IP) in runners.yml")
		r.Warn++
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
	inst := ""
	for i := range hostRunners {
		if hostRunners[i].IsContainerMode() && len(hostRunners[i].InstanceNames()) > 0 {
			inst = hostRunners[i].InstanceNames()[0]
			break
		}
	}
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
	out, err := h.Run(fmt.Sprintf(
		"docker exec %s sh -c %s 2>/dev/null",
		runner.QuoteContainerName(cname), hostshell.PosixSingleQuote(script)))
	switch {
	case err != nil:
		printLine(w, sevWarn, hostName, fmt.Sprintf("cache: could not probe from runner container %s: %v", cname, err))
		r.Warn++
	case strings.Contains(strings.TrimSpace(out), "PROBE_FAIL"):
		bind := effectiveBind(s, h)
		printLine(w, sevFail, hostName, fmt.Sprintf(
			"cache: not reachable from runner container %s at %s (host-side health is fine — the host INPUT firewall is blocking container traffic); allow it with: sudo ufw allow in on docker0 to %s port %d proto tcp comment 'allow-docker-cache' (or the equivalent nftables/iptables rule), then run: gh sr cache deploy && gh sr rebuild %s",
			cname, url, bind, s.EffectivePort(), inst))
		r.Fail++
	default:
		printLine(w, sevOK, hostName, fmt.Sprintf("cache: reachable from runner container %s at %s", cname, url))
	}
}

// effectiveBind resolves the bind address the deploy actually chose: an
// explicit cache.bind_addr wins; the auto path binds the docker0 gateway, or
// 0.0.0.0 when no gateway was found.
func effectiveBind(s *cache.Settings, h *host.Host) string {
	if s.BindAddr != "" {
		return s.BindAddr
	}
	gw, err := cache.ResolveGatewayIP(h)
	if err != nil || gw == "" {
		return "0.0.0.0"
	}
	return gw
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
		printLine(w, sevWarn, hostName, fmt.Sprintf("registry: inspect failed: %v", err))
		r.Warn++
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
			printLine(w, sevWarn, hostName, fmt.Sprintf("%s: enabled but not deployed; run: gh sr registry deploy", label))
			r.Warn++
			continue
		}
		if !m.Info.Healthy {
			url := m.Info.URL
			if url == "" {
				url = "(no container-reachable URL resolved)"
			}
			printLine(w, sevFail, hostName, fmt.Sprintf("%s: mirror not healthy at %s; check the mirror container logs: docker logs %s", label, url, m.Container))
			r.Fail++
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
	if mirrorEffectiveBind(s, h) == "0.0.0.0" {
		printLine(w, sevWarn, hostName, "registry: bound to 0.0.0.0 — the mirror is reachable from your LAN; set registry_mirror.bind_addr (e.g. the docker0 gateway IP) in runners.yml")
		r.Warn++
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
	for _, line := range strings.Split(out, "\n") {
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
		printLine(w, sevWarn, hostName, fmt.Sprintf(
			"registry: %s runs on this host but is not in registry_mirror.mirrors; remove it with: docker rm -f %s (its storage dir keeps occupying disk)",
			name, name))
		r.Warn++
	}
}

// probeRegistryFromRunner curls the mirror's /v2/ endpoint from inside the
// first container-mode runner on the host (same host-INPUT firewall failure
// mode probeCacheFromRunner guards against) and verifies the runner container
// actually carries GH_SR_REGISTRY_MIRROR — the env is baked at docker create,
// so containers created before the mirror was enabled keep pulling from
// Docker Hub until recreated.
func probeRegistryFromRunner(w io.Writer, hostName string, h *host.Host, hostRunners []config.RunnerConfig, s *registry.Settings, r *Result) {
	inst := ""
	for i := range hostRunners {
		if hostRunners[i].IsContainerMode() && len(hostRunners[i].InstanceNames()) > 0 {
			inst = hostRunners[i].InstanceNames()[0]
			break
		}
	}
	if inst == "" {
		return
	}
	url := s.RunnerURL(h)
	if url == "" {
		return
	}
	cname := runner.ContainerDockerName(inst)
	envOut, err := h.Run(fmt.Sprintf(
		"docker inspect --format='{{range .Config.Env}}{{println .}}{{end}}' %s 2>/dev/null | grep -c '^GH_SR_REGISTRY_MIRROR=' || true",
		runner.QuoteContainerName(cname)))
	if err == nil && strings.TrimSpace(envOut) == "0" {
		printLine(w, sevWarn, hostName, fmt.Sprintf(
			"registry: runner container %s predates the mirror (no GH_SR_REGISTRY_MIRROR env); recreate with: gh sr rebuild %s",
			cname, inst))
		r.Warn++
		return
	}
	script := fmt.Sprintf("curl -fsS -m 5 -o /dev/null -w %%{http_code} %s/v2/ || echo PROBE_FAIL", strings.TrimSuffix(url, "/"))
	out, err := h.Run(fmt.Sprintf(
		"docker exec %s sh -c %s 2>/dev/null",
		runner.QuoteContainerName(cname), hostshell.PosixSingleQuote(script)))
	switch {
	case err != nil:
		printLine(w, sevWarn, hostName, fmt.Sprintf("registry: could not probe from runner container %s: %v", cname, err))
		r.Warn++
	case strings.Contains(strings.TrimSpace(out), "PROBE_FAIL") || strings.TrimSpace(out) != "200":
		bind := mirrorEffectiveBind(s, h)
		printLine(w, sevFail, hostName, fmt.Sprintf(
			"registry: not reachable from runner container %s at %s (host-side health is fine — the host INPUT firewall is blocking container traffic); allow it with: sudo ufw allow in on docker0 to %s port %d proto tcp comment 'allow-docker-registry-mirror' (or the equivalent nftables/iptables rule)",
			cname, url, bind, s.EffectivePort()))
		r.Fail++
	default:
		printLine(w, sevOK, hostName, fmt.Sprintf("registry: reachable from runner container %s at %s", cname, url))
	}
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
	inst := ""
	for i := range hostRunners {
		if hostRunners[i].IsContainerMode() && len(hostRunners[i].InstanceNames()) > 0 {
			inst = hostRunners[i].InstanceNames()[0]
			break
		}
	}
	if inst == "" {
		return
	}
	cname := runner.ContainerDockerName(inst)
	script := fmt.Sprintf("curl -fsS -m 5 -o /dev/null -w %%{http_code} %s/v2/ || echo PROBE_FAIL", strings.TrimSuffix(url, "/"))
	out, err := h.Run(fmt.Sprintf(
		"docker exec %s sh -c %s 2>/dev/null",
		runner.QuoteContainerName(cname), hostshell.PosixSingleQuote(script)))
	switch {
	case err != nil:
		printLine(w, sevWarn, hostName, fmt.Sprintf("%s: could not probe from runner container %s: %v", label, cname, err))
		r.Warn++
	case strings.Contains(strings.TrimSpace(out), "PROBE_FAIL") || strings.TrimSpace(out) != "200":
		printLine(w, sevWarn, hostName, fmt.Sprintf("%s: not reachable from runner container %s at %s (host INPUT firewall); not fatal — prefetch pulls run on the host", label, cname, url))
		r.Warn++
	default:
		printLine(w, sevOK, hostName, fmt.Sprintf("%s: reachable from runner container %s at %s", label, cname, url))
	}
}

// mirrorEffectiveBind resolves the bind address the mirror deploy actually
// chose: an explicit registry_mirror.bind_addr wins; the auto path binds the
// docker0 gateway, or 0.0.0.0 when no gateway was found.
func mirrorEffectiveBind(s *registry.Settings, h *host.Host) string {
	if s.BindAddr != "" {
		return s.BindAddr
	}
	gw, err := cache.ResolveGatewayIP(h)
	if err != nil || gw == "" {
		return "0.0.0.0"
	}
	return gw
}

func checkOrphanRunners(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	configured := runner.ConfiguredInstanceSet(runners, hostName)
	orphans, err := runner.OrphanInstances(h, configured)
	if err != nil {
		printLine(w, sevWarn, hostName, fmt.Sprintf("orphan runners: list: %v", err))
		r.Warn++
		return
	}
	if len(orphans) == 0 {
		return
	}
	printLine(w, sevWarn, hostName, fmt.Sprintf("orphan runners: %d instance(s) not in runners.yml (%s); run: gh sr service cleanup",
		len(orphans), strings.Join(orphans, ", ")))
	r.Warn++
}

func checkRunnerDiskUsage(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	threshold := runner.DiskWarnThresholdBytes()
	rcByInstance := make(map[string]*config.RunnerConfig)
	for i := range runners {
		rc := &runners[i]
		if rc.Host != hostName {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			rcByInstance[inst] = rc
		}
	}

	seen := make(map[string]struct{})
	instances := make([]string, 0, len(rcByInstance))
	for inst := range rcByInstance {
		instances = append(instances, inst)
		seen[inst] = struct{}{}
	}
	if diskDirs, err := runner.ListRunnerInstanceDirs(h); err == nil {
		for _, inst := range diskDirs {
			if _, ok := seen[inst]; ok {
				continue
			}
			seen[inst] = struct{}{}
			instances = append(instances, inst)
		}
	}
	sort.Strings(instances)

	// One batched SSH round-trip per host replaces the prior per-instance
	// MeasureDiskUsage loop (N+1 fold; matches the win-class of PRs
	// #358/#361/#372/#380/#379).
	entries := runner.MeasureDiskUsageBatch(h, hostName, instances, rcByInstance)
	for _, inst := range instances {
		entry := entries[inst]
		if entry.Err != nil {
			printLine(w, sevWarn, hostName, fmt.Sprintf("disk: instance %s: %v", inst, entry.Err))
			r.Warn++
			continue
		}
		if entry.TotalBytes >= threshold {
			label := inst
			if entry.Orphan {
				label = inst + " (orphan)"
			}
			printLine(w, sevWarn, hostName, fmt.Sprintf("disk: instance %s uses %s under %s; run: gh sr disk prune --yes",
				label, runner.FormatBytesHuman(entry.TotalBytes), entry.Path))
			r.Warn++
		}
	}
}
