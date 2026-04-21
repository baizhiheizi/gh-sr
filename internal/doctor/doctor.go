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
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/ghawports"
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
// workflowRoot, when non-empty, enables MCP port lint for .github/workflows/*.md under that path.
// When empty and filterRepo is set, uses "." if ./.github/workflows exists.
func Run(w io.Writer, cfgPath, envPath string, cfg *config.Config, cfgErr error, gh *runner.GitHubClient, filterHost, filterRepo string, strict bool, workflowRoot string) Result {
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
		runAgenticMCPWorkflowCheck(w, cfg, filterRepo, workflowRoot, &r)
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
				list, err := gh.ListRunners(repo)
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
					orgResults[idx] = apiResult{sevFail, fmt.Sprintf("org %s: %v", org, err)}
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
			checkNative(&hr.buf, hostName, h, &hr.r)
			checkNativeRunnerInstall(&hr.buf, hostName, h, runners, &hr.r)
			if h.OS == "linux" {
				checkLinuxSudo(&hr.buf, hostName, h, &hr.r)
			}
			hostRunners := cfg.RunnersForHost(hostName)
			if hasNativeAgenticRunners(hostRunners) {
				checkAgenticPrereqs(&hr.buf, hostName, h, &hr.r)
			}
			if hasContainerAgenticRunners(hostRunners) {
				checkContainerAgenticPrereqs(&hr.buf, hostName, h, &hr.r)
			}
			if h.OS == "linux" && hasAgenticRunners(hostRunners) {
				checkAWFHygiene(&hr.buf, hostName, h, &hr.r)
			}
		}(i, hostName)
	}
	hostWg.Wait()

	for i := range hostOrder {
		hr := &hostResults[i]
		_, _ = io.Copy(w, &hr.buf)
		r.Fail += hr.r.Fail
		r.Warn += hr.r.Warn
	}

	runAgenticMCPWorkflowCheck(w, cfg, filterRepo, workflowRoot, &r)

	printSummary(w, r, strict)
	return r
}

func resolveWorkflowRoot(flag, filterRepo string) string {
	flag = strings.TrimSpace(flag)
	if flag != "" {
		return flag
	}
	if filterRepo != "" {
		if st, err := os.Stat(".github/workflows"); err == nil && st.IsDir() {
			return "."
		}
	}
	return ""
}

func runAgenticMCPWorkflowCheck(w io.Writer, cfg *config.Config, filterRepo, workflowRoot string, r *Result) {
	root := resolveWorkflowRoot(workflowRoot, filterRepo)
	if root == "" {
		return
	}
	fmt.Fprintln(w, "\n=== Agentic MCP ports (workflow markdown) ===")
	var mbuf bytes.Buffer
	warns, fails := ghawports.Check(&mbuf, cfg, ghawports.CheckOpts{WorkflowRoot: root, RepoFilter: filterRepo})
	_, _ = io.Copy(w, &mbuf)
	r.Warn += warns
	r.Fail += fails
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
		if rc.Host != hostName {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			out = append(out, [2]string{inst, rc.Name})
		}
	}
	return out
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

// hasAgenticRunners returns true if any runner in the list uses the agentic profile.
func hasAgenticRunners(runners []config.RunnerConfig) bool {
	for _, rc := range runners {
		if rc.IsAgentic() {
			return true
		}
	}
	return false
}

// hasNativeAgenticRunners returns true if any runner uses agentic profile in native mode.
func hasNativeAgenticRunners(runners []config.RunnerConfig) bool {
	for _, rc := range runners {
		if rc.IsAgentic() && !rc.IsContainerMode() {
			return true
		}
	}
	return false
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

// checkAgenticPrereqs verifies Docker is available on the host for agentic workflow containers.
// gh-aw (GitHub Agentic Workflows) uses Docker on the host for AWF sandbox containers.
// It uses agentic.ValidatePrereqs for comprehensive checking and prints remediation guidance.
func checkAgenticPrereqs(w io.Writer, hostName string, h *host.Host, r *Result) {
	failures := agentic.ValidatePrereqs(h)

	for _, f := range failures {
		sev := sevFail
		if f.Severity == agentic.SeverityWarning {
			sev = sevWarn
		}
		r.Fail++
		if sev == sevWarn {
			r.Fail--
			r.Warn++
		}
		printLine(w, sev, hostName, "agentic: "+f.Message)
		// Print remediation guidance inline
		if f.Remediation != "" {
			lines := strings.Split(f.Remediation, "\n")
			for _, line := range lines {
				fmt.Fprintf(w, "       %s\n", line)
			}
		}
		if f.DocRef != "" {
			fmt.Fprintf(w, "       See: %s\n", f.DocRef)
		}
	}

	// If no failures, print a summary OK line
	if len(failures) == 0 {
		// Docker CLI version
		out, _ := h.Run(`docker --version 2>/dev/null`)
		if strings.Contains(out, "Docker version") {
			printLine(w, sevOK, hostName, fmt.Sprintf("agentic: docker CLI %s", strings.TrimSpace(out)))
		}
		// Docker daemon
		out, _ = h.Run(`docker info 2>/dev/null`)
		if _, err := h.Run(`docker info >/dev/null 2>&1`); err == nil {
			printLine(w, sevOK, hostName, "agentic: docker daemon running")
		}
		// Docker compose
		out, _ = h.Run(`docker compose version 2>/dev/null`)
		if _, err := h.Run(`docker compose version >/dev/null 2>&1`); err == nil {
			printLine(w, sevOK, hostName, fmt.Sprintf("agentic: docker compose %s", strings.TrimSpace(out)))
		}
		// Socket access
		out, _ = h.Run(`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker ps 2>/dev/null`)
		if _, err := h.Run(`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker ps >/dev/null 2>&1`); err == nil {
			printLine(w, sevOK, hostName, "agentic: can spawn containers via Docker socket")
		}
		// iptables
		if _, err := h.Run(`command -v iptables >/dev/null 2>&1`); err == nil {
			printLine(w, sevOK, hostName, "agentic: iptables available")
		}
		// sudo iptables
		uid, _ := h.Run(`id -u`)
		if strings.TrimSpace(uid) != "0" {
			out, _ := h.Run(`sudo -n iptables -L DOCKER-USER >/dev/null 2>&1 && echo ok || echo no`)
			if strings.TrimSpace(out) == "ok" {
				printLine(w, sevOK, hostName, "agentic: passwordless sudo for iptables available")
			}
		} else {
			printLine(w, sevOK, hostName, "agentic: running as root (no sudo needed)")
		}
		// host.docker.internal
		out, _ = h.Run(`docker run --rm alpine sh -c "getent hosts host.docker.internal" 2>/dev/null`)
		if fields := strings.Fields(strings.TrimSpace(out)); len(fields) > 0 && fields[0] != "127.0.0.1" && fields[0] != "::1" {
			printLine(w, sevOK, hostName, fmt.Sprintf("agentic: host.docker.internal resolves inside containers (%s)", fields[0]))
		}
		// external DNS
		out, _ = h.Run(`docker run --rm alpine sh -c "nslookup github.com >/dev/null 2>&1 && echo ok || echo failed" 2>/dev/null`)
		if strings.TrimSpace(out) == "ok" {
			printLine(w, sevOK, hostName, "agentic: external DNS resolves inside containers")
		}
	}
}

// checkContainerAgenticPrereqs checks host requirements for container-mode (DinD) agentic runners.
// The inner dockerd, dnsmasq, and iptables all live inside the runner image, so only
// the outer Docker availability and --privileged support are checked here.
func checkContainerAgenticPrereqs(w io.Writer, hostName string, h *host.Host, r *Result) {
	failures := agentic.ValidateContainerPrereqs(h)

	if len(failures) == 0 {
		printLine(w, sevOK, hostName, "agentic(container): docker available and --privileged supported")
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
		printLine(w, sev, hostName, "agentic(container): "+f.Message)
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

// checkAWFHygiene reports stale AWF artefacts (orphan containers, stale iptables rules).
// These are always warnings; they don't block setup but waste resources.
func checkAWFHygiene(w io.Writer, hostName string, h *host.Host, r *Result) {
	failures := agentic.ValidateAWFHygiene(h)
	if len(failures) == 0 {
		printLine(w, sevOK, hostName, "agentic: no orphan AWF containers or stale iptables rules")
		return
	}
	for _, f := range failures {
		sev := sevWarn
		if f.Severity == agentic.SeverityError {
			sev = sevFail
			r.Fail++
		} else {
			r.Warn++
		}
		printLine(w, sev, hostName, "agentic(hygiene): "+f.Message)
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
