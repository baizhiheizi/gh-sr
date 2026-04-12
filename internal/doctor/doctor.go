package doctor

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

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
		for _, repo := range repos {
			list, err := gh.ListRunners(repo)
			if err != nil {
				printLine(w, sevFail, "github", fmt.Sprintf("%s: %v", repo, err))
				r.Fail++
				continue
			}
			printLine(w, sevOK, "github", fmt.Sprintf("%s: list runners OK (%d registered)", repo, len(list)))
		}
		orgs := uniqueOrgs(runners)
		for _, org := range orgs {
			list, err := gh.ListRunnersScoped("org", org)
			if err != nil {
				printLine(w, sevFail, "github", fmt.Sprintf("org %s: %v", org, err))
				r.Fail++
				continue
			}
			printLine(w, sevOK, "github", fmt.Sprintf("org %s: list runners OK (%d registered)", org, len(list)))
		}
	}

	fmt.Fprintln(w, "\n=== Hosts ===")
	hostOrder := uniqueHostNames(runners)
	for _, hostName := range hostOrder {
		hcfg := cfg.Hosts[hostName]

		h := host.NewHost(hostName, hcfg)
		if err := h.Connect(); err != nil {
			printLine(w, sevFail, hostName, fmt.Sprintf("connect: %v", err))
			r.Fail++
			continue
		}
		if err := ensureDoctorHostOS(h, hcfg.Addr); err != nil {
			printLine(w, sevFail, hostName, fmt.Sprintf("detect os: %v", err))
			r.Fail++
			_ = h.Close()
			continue
		}

		func() {
			defer h.Close()
			printLine(w, sevOK, hostName, fmt.Sprintf("connected (%s)", addrSummary(hcfg.Addr)))
			checkNative(w, hostName, h, &r)
			checkNativeRunnerInstall(w, hostName, h, runners, &r)
			if h.OS == "linux" {
				checkLinuxSudo(w, hostName, h, &r)
			}
			if hasAgenticRunners(cfg.RunnersForHost(hostName)) {
				checkAgenticPrereqs(w, hostName, h, &r)
			}
		}()
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

// checkAgenticPrereqs verifies Docker is available on the host for agentic workflow containers.
// gh-aw (GitHub Agentic Workflows) uses Docker on the host for AWF sandbox containers.
func checkAgenticPrereqs(w io.Writer, hostName string, h *host.Host, r *Result) {
	if h.OS != "linux" {
		printLine(w, sevFail, hostName, "agentic: gh-aw only supported on Linux hosts")
		r.Fail++
		return
	}

	// Check docker CLI.
	out, err := h.Run(`docker --version 2>/dev/null`)
	if err != nil || !strings.Contains(out, "Docker version") {
		printLine(w, sevFail, hostName, "agentic: docker CLI not found on PATH; gh-aw requires Docker to be pre-installed on the host")
		r.Fail++
		return
	}
	printLine(w, sevOK, hostName, fmt.Sprintf("agentic: docker CLI %s", strings.TrimSpace(out)))

	// Check docker daemon is running.
	out, err = h.Run(`docker info 2>/dev/null`)
	if err != nil {
		printLine(w, sevFail, hostName, "agentic: docker daemon not running (docker info failed); start docker or ensure it starts at boot")
		r.Fail++
		return
	}
	printLine(w, sevOK, hostName, "agentic: docker daemon running")

	// Check docker compose plugin.
	out, err = h.Run(`docker compose version 2>/dev/null`)
	if err != nil {
		printLine(w, sevWarn, hostName, "agentic: docker compose plugin not found (docker compose version failed); gh-aw may need it for multi-container setup")
		r.Warn++
	} else {
		printLine(w, sevOK, hostName, fmt.Sprintf("agentic: docker compose %s", strings.TrimSpace(out)))
	}

	// Check Docker-in-Docker: MCP gateway spawns containers via Docker socket.
	out, err = h.Run(`docker run --rm -v /var/run/docker.sock:/var/run/docker.sock docker:cli docker ps 2>/dev/null`)
	if err != nil {
		printLine(w, sevFail, hostName, "agentic: cannot spawn containers via Docker socket; MCP gateway will fail to launch (see docs agentic-workflows.md §4c)")
		r.Fail++
	} else {
		printLine(w, sevOK, hostName, "agentic: can spawn containers via Docker socket")
	}

	// Check iptables is available.
	out, err = h.Run(`command -v iptables >/dev/null 2>&1 && echo ok || echo missing`)
	if err != nil || strings.TrimSpace(out) != "ok" {
		printLine(w, sevFail, hostName, "agentic: iptables not found on PATH; gh-aw needs it for network egress control (DOCKER-USER chain)")
		r.Fail++
		return
	}
	printLine(w, sevOK, hostName, "agentic: iptables available")

	// Check DOCKER-USER chain exists (gh-aw creates it).
	out, err = h.Run(`iptables -L DOCKER-USER 2>/dev/null && echo exists || echo missing`)
	if err != nil || strings.TrimSpace(out) != "exists" {
		printLine(w, sevWarn, hostName, "agentic: DOCKER-USER chain not found; gh-aw creates it on first run; ensure passwordless sudo for iptables")
		r.Warn++
	} else {
		printLine(w, sevOK, hostName, "agentic: DOCKER-USER chain exists")
	}

	// Check RUNNER_TEMP for agentic runners.
	// gh-aw uses /tmp/gh-aw for its runtime tree. If the runner's RUNNER_TEMP is /tmp,
	// bind mounts and security isolation will conflict with gh-aw's own files.
	out, err = h.Run(`echo "${RUNNER_TEMP:-}"`)
	if err == nil {
		rt := strings.TrimSpace(out)
		if rt == "/tmp" {
			printLine(w, sevWarn, hostName, "agentic: RUNNER_TEMP=/tmp conflicts with gh-aw runtime tree at /tmp/gh-aw (mount and isolation conflicts)")
			printLine(w, sevWarn, hostName, "agentic: fix: set RUNNER_TEMP to a path under the runner work directory, e.g. ~/.gh-sr/runners/<name>/_work/_temp")
			r.Warn++
		}
	}

	// Check passwordless sudo for iptables (needed for gh-aw egress rules).
	uid, err := h.Run(`id -u`)
	if err != nil {
		printLine(w, sevWarn, hostName, fmt.Sprintf("agentic: could not check uid for iptables sudo: %v", err))
		r.Warn++
		return
	}
	if strings.TrimSpace(uid) != "0" {
		out, err = h.Run(`sudo -n iptables -L DOCKER-USER >/dev/null 2>&1 && echo ok || echo no`)
		if err != nil || strings.TrimSpace(out) != "ok" {
			printLine(w, sevWarn, hostName, "agentic: passwordless sudo for iptables not available; gh-aw may fail to set egress rules")
			r.Warn++
			return
		}
		printLine(w, sevOK, hostName, "agentic: passwordless sudo for iptables available")
	}

	// Check host.docker.internal resolution inside containers.
	// gh-aw relies on host.docker.internal to reach the MCP gateway from agent containers.
	out, err = h.Run(`docker run --rm alpine sh -c "getent hosts host.docker.internal || echo failed" 2>/dev/null`)
	out = strings.TrimSpace(out)
	if err != nil || out == "failed" || out == "" {
		printLine(w, sevFail, hostName, "agentic: host.docker.internal does not resolve inside containers; configure Docker DNS (see README)")
		r.Fail++
	} else if strings.Contains(out, "127.0.0.1") {
		printLine(w, sevFail, hostName, "agentic: host.docker.internal resolves to 127.0.0.1 inside containers; this breaks gh-aw MCP gateway (see README)")
		r.Fail++
	} else {
		fields := strings.Fields(out)
		ip := ""
		if len(fields) > 0 {
			ip = fields[0]
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("agentic: host.docker.internal resolves correctly inside containers (%s)", ip))
	}

	// Check general DNS resolution inside containers.
	// If dnsmasq is configured without upstream servers, it only answers static records
	// and REFUSES everything else, breaking external API access (model providers, etc.).
	out, err = h.Run(`docker run --rm alpine sh -c "nslookup github.com >/dev/null 2>&1 && echo ok || echo failed" 2>/dev/null`)
	out = strings.TrimSpace(out)
	if err != nil || out != "ok" {
		printLine(w, sevFail, hostName, "agentic: external DNS (github.com) does not resolve inside containers; check Docker DNS / dnsmasq upstream server config (see README)")
		r.Fail++
	} else {
		printLine(w, sevOK, hostName, "agentic: external DNS resolves inside containers")
	}

	// Check container → host TCP reachability via host.docker.internal.
	// The MCP gateway listens on the host; agent containers must be able to TCP-connect to it.
	tcpCheck, err := h.Run(`docker run --rm --add-host=host.docker.internal:host-gateway alpine sh -c "(echo > /dev/tcp/host.docker.internal/80) 2>/dev/null && echo ok || echo failed" 2>/dev/null`)
	tcpCheck = strings.TrimSpace(tcpCheck)
	if err != nil || tcpCheck == "failed" {
		printLine(w, sevFail, hostName, "agentic: container cannot TCP-connect to host via host.docker.internal:80; MCP gateway will be unreachable (see README §4b)")
		r.Fail++
	} else {
		printLine(w, sevOK, hostName, "agentic: container can reach host via host.docker.internal:80")
	}
}
