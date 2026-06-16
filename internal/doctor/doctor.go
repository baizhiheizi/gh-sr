package doctor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/an-lee/gh-sr/internal/agentic"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
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
func Run(w io.Writer, cfgPath, envPath string, cfg *config.Config, cfgErr error, gh *runner.GitHubClient, filterHost, filterRepo string, strict bool) Result {
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
		repos := uniqueRepos(runners)
		orgs := uniqueOrgs(runners)

		type apiResult struct {
			sev string
			msg string
		}
		repoResults := make([]apiResult, len(repos))
		orgResults := make([]apiResult, len(orgs))

		var apiWg sync.WaitGroup
		for i, repo := range repos {
			apiWg.Add(1)
			go func(idx int, repo string) {
				defer apiWg.Done()
				list, err := gh.ListRunnersScoped("repo", repo)
				if err != nil {
					repoResults[idx] = apiResult{sevFail, fmt.Sprintf("%s: %v", repo, err)}
				} else {
					repoResults[idx] = apiResult{sevOK, fmt.Sprintf("%s: list runners OK (%d registered)", repo, len(list))}
				}
			}(i, repo)
		}
		for i, org := range orgs {
			apiWg.Add(1)
			go func(idx int, org string) {
				defer apiWg.Done()
				list, err := gh.ListRunnersScoped("org", org)
				if err != nil {
					orgResults[idx] = apiResult{sevFail, formatOrgAPIError(org, err)}
				} else {
					orgResults[idx] = apiResult{sevOK, fmt.Sprintf("org %s: list runners OK (%d registered)", org, len(list))}
				}
			}(i, org)
		}
		apiWg.Wait()

		for _, res := range repoResults {
			printLine(w, res.sev, "github", res.msg)
			if res.sev == sevFail {
				r.Fail++
			}
		}
		for _, res := range orgResults {
			printLine(w, res.sev, "github", res.msg)
			if res.sev == sevFail {
				r.Fail++
			}
		}
	}

	fmt.Fprintln(w, "\n=== Hosts ===")
	hostOrder := uniqueHostNames(runners)

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
			runHostChecks(&hr.buf, hostName, h, runners, &hr.r)
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

func uniqueRepos(runners []config.RunnerConfig) []string {
	seen := make(map[string]struct{})
	for _, rc := range runners {
		if rc.Repo != "" {
			seen[rc.Repo] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for repo := range seen {
		out = append(out, repo)
	}
	sort.Strings(out)
	return out
}

func uniqueOrgs(runners []config.RunnerConfig) []string {
	seen := make(map[string]struct{})
	for _, rc := range runners {
		if rc.Org != "" {
			seen[rc.Org] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for org := range seen {
		out = append(out, org)
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

func uniqueHostNames(runners []config.RunnerConfig) []string {
	seen := make(map[string]struct{})
	for _, rc := range runners {
		seen[rc.Host] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// nativeInstallTargetsForHost lists (instanceName, runnerConfigName) for native-mode runners on hostName.
func nativeInstallTargetsForHost(runners []config.RunnerConfig, hostName string) [][2]string {
	var out [][2]string
	for _, rc := range runners {
		if rc.Host != hostName || rc.IsContainerMode() {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			out = append(out, [2]string{inst, rc.Name})
		}
	}
	return out
}

// containerInstallTargetsForHost lists (instanceName, runnerConfigName) for container-mode runners on hostName.
func containerInstallTargetsForHost(runners []config.RunnerConfig, hostName string) [][2]string {
	var out [][2]string
	for _, rc := range runners {
		if rc.Host != hostName || !rc.IsContainerMode() {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			out = append(out, [2]string{inst, rc.Name})
		}
	}
	return out
}

// containerAgenticInstallTargetsForHost lists container-mode instances that use profile: agentic.
func containerAgenticInstallTargetsForHost(runners []config.RunnerConfig, hostName string) [][2]string {
	var out [][2]string
	for _, rc := range runners {
		if rc.Host != hostName || !rc.IsContainerMode() || !rc.IsAgentic() {
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
// already-filtered runners slice (respects --host / --repo filters).
func runHostChecks(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	hostRunners := runnersForHost(runners, hostName)
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
				checkContainerAgenticInnerHygiene(w, hostName, h, runners, r)
			}
		}
	}
	if h.OS == "linux" || h.OS == "darwin" || h.OS == "windows" {
		checkOrphanRunners(w, hostName, h, runners, r)
	}
	checkRunnerDiskUsage(w, hostName, h, runners, r)
}

func checkNativeRunnerInstall(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	for _, pair := range nativeInstallTargetsForHost(runners, hostName) {
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

func checkNative(w io.Writer, hostName string, h *host.Host, r *Result) {
	switch h.OS {
	case "linux":
		out, err := h.Run(`if command -v curl >/dev/null 2>&1 && command -v tar >/dev/null 2>&1; then echo ok; else echo missing; fi`)
		out = strings.TrimSpace(out)
		if err != nil || out != "ok" {
			printLine(w, sevFail, hostName, fmt.Sprintf("native: need curl and tar on PATH (%v)", err))
			r.Fail++
			return
		}
		printLine(w, sevOK, hostName, "native: curl and tar present")
	case "darwin":
		out, err := h.Run(`command -v curl >/dev/null 2>&1 && echo ok || echo missing`)
		out = strings.TrimSpace(out)
		if err != nil || out != "ok" {
			printLine(w, sevFail, hostName, fmt.Sprintf("native: need curl (%v)", err))
			r.Fail++
			return
		}
		printLine(w, sevOK, hostName, "native: curl present")
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

// checkContainerHostPrereqs checks host requirements for runner_mode: container (DinD).
// The inner dockerd, dnsmasq, and iptables live inside the runner image; only the
// outer Docker daemon and --privileged support are checked here.
func checkContainerHostPrereqs(w io.Writer, hostName string, h *host.Host, r *Result) {
	failures := agentic.ValidateContainerPrereqs(h)

	if len(failures) == 0 {
		printLine(w, sevOK, hostName, "container: host Docker available and --privileged supported")
		return
	}

	for _, f := range failures {
		sev := sevFail
		if f.Severity == agentic.SeverityWarning {
			sev = sevWarn
			r.Warn++
		} else {
			r.Fail++
		}
		printLine(w, sev, hostName, "container: "+f.Message)
		if f.Remediation != "" {
			for _, line := range strings.Split(f.Remediation, "\n") {
				fmt.Fprintf(w, "       %s\n", line)
			}
		}
		if f.DocRef != "" {
			fmt.Fprintf(w, "       See: %s\n", f.DocRef)
		}
	}
}

// checkContainerRunnerInstall verifies each DinD runner container exists on the host,
// is running (warn otherwise), inner dockerd responds, and .runner is present inside the image path.
func checkContainerRunnerInstall(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	for _, pair := range containerInstallTargetsForHost(runners, hostName) {
		inst, runnerName := pair[0], pair[1]
		cname := runner.ContainerDockerName(inst)
		q := strconv.Quote(cname)

		out, err := h.Run(fmt.Sprintf(`docker inspect --format '{{.State.Status}}' %s 2>/dev/null || echo missing`, q))
		status := strings.TrimSpace(out)
		if err != nil || status == "missing" || status == "" {
			printLine(w, sevFail, hostName, fmt.Sprintf("container: instance %s (%s) — Docker container %s not found; run: gh sr setup %s", inst, runnerName, cname, runnerName))
			r.Fail++
			continue
		}
		if status != "running" && status != "restarting" {
			printLine(w, sevWarn, hostName, fmt.Sprintf("container: instance %s (%s) — %s state is %q (expected running); run: gh sr up %s", inst, runnerName, cname, status, runnerName))
			r.Warn++
			continue
		}

		if _, err := h.Run(fmt.Sprintf("docker exec %s docker info >/dev/null 2>&1", q)); err != nil {
			printLine(w, sevWarn, hostName, fmt.Sprintf("container: instance %s — inner dockerd not responding inside %s", inst, cname))
			r.Warn++
		} else {
			printLine(w, sevOK, hostName, fmt.Sprintf("container: instance %s — inner dockerd healthy (%s)", inst, cname))
		}

		out, _ = h.Run(fmt.Sprintf("docker exec %s test -f /home/runner/actions-runner/.runner && echo ok || echo no", q))
		if strings.TrimSpace(out) != "ok" {
			printLine(w, sevFail, hostName, fmt.Sprintf("container: instance %s — actions runner not configured inside %s (missing .runner); run: gh sr setup %s", inst, cname, runnerName))
			r.Fail++
			continue
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("container: instance %s — registered (.runner present in %s)", inst, cname))
	}
}

// checkContainerAgenticInnerHygiene runs AWF orphan and network checks against the inner Docker in each running DinD agentic runner.
func checkContainerAgenticInnerHygiene(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	targets := containerAgenticInstallTargetsForHost(runners, hostName)
	if len(targets) == 0 {
		return
	}
	// Host egress MTU is host-level; detect once and reuse for every instance's MTU check.
	hostEgressMTU := runner.DetectHostEgressMTU(h)

	for _, pair := range targets {
		inst := pair[0]
		runnerName := pair[1]
		cname := runner.ContainerDockerName(inst)
		q := strconv.Quote(cname)

		out, err := h.Run(fmt.Sprintf(`docker inspect --format '{{.State.Status}}' %s 2>/dev/null || echo missing`, q))
		status := strings.TrimSpace(out)
		if err != nil || status != "running" {
			continue
		}

		failures := agentic.ValidateAWFHygieneInner(h, cname)
		failures = append(failures, agentic.ValidateContainerNodeNPM(h, cname, runnerName)...)
		failures = append(failures, agentic.ValidateContainerAWF(h, cname, runnerName)...)
		failures = append(failures, agentic.ValidateContainerInnerNetwork(h, cname, runnerName)...)
		failures = append(failures, agentic.ValidateContainerInnerResolv(h, cname, runnerName)...)
		failures = append(failures, agentic.ValidateContainerAWFServiceRouting(h, cname, runnerName)...)
		failures = append(failures, agentic.ValidateContainerMTU(h, cname, runnerName, hostEgressMTU)...)
		if len(failures) == 0 {
			printLine(w, sevOK, hostName, fmt.Sprintf("container(agent): awf installed, inner Docker clean, host.docker.internal reachable, resolv.conf pinned to dnsmasq, and AWF service-routing bypass present (%s)", cname))
			continue
		}
		for _, f := range failures {
			sev := sevWarn
			if f.Severity == agentic.SeverityError {
				sev = sevFail
				r.Fail++
			} else {
				r.Warn++
			}
			printLine(w, sev, hostName, "container(agent): "+f.Message)
			if f.Remediation != "" {
				for _, line := range strings.Split(f.Remediation, "\n") {
					fmt.Fprintf(w, "       %s\n", line)
				}
			}
			if f.DocRef != "" {
				fmt.Fprintf(w, "       See: %s\n", f.DocRef)
			}
		}
	}
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

	for _, inst := range instances {
		rc := rcByInstance[inst]
		entry := runner.MeasureDiskUsage(h, hostName, inst, rc)
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
