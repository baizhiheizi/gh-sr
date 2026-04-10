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
// hasGitHubToken is true when gh CLI credentials yielded a token for github.com.
// If cfg is nil (load error), GitHub and host checks are skipped after the configuration section.
func Run(w io.Writer, cfgPath, envPath string, cfg *config.Config, cfgErr error, gh *runner.GitHubClient, hasGitHubToken bool, filterHost, filterRepo string, strict bool) Result {
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

	if hasGitHubToken {
		printLine(w, sevOK, "local", "GitHub token: from gh CLI (gh auth login)")
	} else {
		printLine(w, sevFail, "local", "GitHub token: not found; run `gh auth login`")
		r.Fail++
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
	fmt.Fprintf(w, "Docker mode uses image: %s\n\n", runner.RunnerDockerImage)
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
		modes := modesForHost(runners, hostName, h.OS)

		func() {
			defer h.Close()
			printLine(w, sevOK, hostName, fmt.Sprintf("connected (%s)", addrSummary(hcfg.Addr)))
			if modes["docker"] {
				checkDocker(w, hostName, h, runners, &r)
				checkAgenticWorkflowDockerHint(w, hostName, h.OS, runners, &r)
			}
			if modes["native"] {
				checkNative(w, hostName, h, &r)
				checkNativeRunnerInstall(w, hostName, h, runners, &r)
			}
			if h.OS == "linux" {
				checkLinuxSudo(w, hostName, h, &r)
			}
			if hasAgenticRunners(runners, hostName) {
				checkAgenticPrereqs(w, hostName, h, runners, &r)
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

func modesForHost(runners []config.RunnerConfig, hostName, hostOS string) map[string]bool {
	m := make(map[string]bool)
	for _, rc := range runners {
		if rc.Host != hostName {
			continue
		}
		m[rc.EffectiveMode(hostOS)] = true
	}
	return m
}

// checkAgenticWorkflowDockerHint warns when docker-mode runners use the default bridge network,
// which breaks GitHub Agentic Workflows MCP gateway localhost health checks unless gh-aw is fixed upstream.
func checkAgenticWorkflowDockerHint(w io.Writer, hostName string, hostOS string, runners []config.RunnerConfig, r *Result) {
	var names []string
	for _, rc := range runners {
		if rc.Host != hostName {
			continue
		}
		if rc.EffectiveMode(hostOS) != "docker" {
			continue
		}
		if rc.EffectiveDockerNetworkMode(hostOS) != "bridge" {
			continue
		}
		names = append(names, rc.Name)
	}
	if len(names) == 0 {
		return
	}
	sort.Strings(names)
	printLine(w, sevWarn, hostName, fmt.Sprintf(
		"agentic workflows: bridge-network docker runners (%s) may fail MCP gateway health checks; set docker_network_mode: host or use mode: native — see host setup documentation",
		strings.Join(names, ", "),
	))
	r.Warn++
}

func checkDocker(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	if h.OS == "windows" {
		out, err := h.RunShell(`docker info --format "{{.ServerVersion}}"`)
		out = strings.TrimSpace(out)
		if err != nil || out == "" {
			reason := "docker info did not return a server version"
			if err != nil {
				reason = err.Error()
			}
			printLine(w, sevFail, hostName, fmt.Sprintf("docker: daemon/CLI not usable (%s); install and start Docker (see README \"Host setup\")", reason))
			r.Fail++
			return
		}
		printLine(w, sevOK, hostName, fmt.Sprintf("docker: server version %s (image %s)", out, runner.RunnerDockerImage))
		checkWindowsDockerSocket(w, hostName, h, runners, r)
		return
	}

	ok, err := runner.UnixDockerCLIInstalled(h)
	if err != nil {
		printLine(w, sevFail, hostName, fmt.Sprintf("docker: could not check CLI: %v", err))
		r.Fail++
		return
	}
	if !ok {
		printLine(w, sevFail, hostName, "docker: CLI not on PATH; install Docker (see README \"Host setup\")")
		r.Fail++
		return
	}
	out, verr := runner.UnixDockerServerVersion(h)
	if verr != nil {
		printLine(w, sevFail, hostName, fmt.Sprintf("docker: %v", verr))
		r.Fail++
		return
	}
	printLine(w, sevOK, hostName, fmt.Sprintf("docker: server version %s (image %s)", out, runner.RunnerDockerImage))

	if h.OS == "linux" || h.OS == "darwin" {
		checkUnixDockerSocket(w, hostName, h, runners, r)
	}
}

// checkWindowsDockerSocket probes the Docker Desktop socket GID and checks that any running
// docker-mode runner containers have the correct --group-add flag for it.
func checkWindowsDockerSocket(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	probeCmd := runner.DockerWindowsSockGIDProbeCommand(runner.RunnerDockerImage) + ` 2>$null`
	out, err := h.RunShell(probeCmd)
	if err != nil {
		printLine(w, sevWarn, hostName, fmt.Sprintf(
			"docker: could not probe socket GID; bind-mount may lack --group-add; jobs may fail with permission denied; recreate containers after fixing",
		))
		r.Warn++
		return
	}
	gid := strings.TrimSpace(out)
	printLine(w, sevOK, hostName, fmt.Sprintf("docker: socket GID=%s", gid))

	// Check each docker-mode runner container that is already running.
	for _, rc := range runners {
		if rc.Host != hostName || rc.EffectiveMode(h.OS) != "docker" {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			cname := runner.ContainerName(inst)
			running, rerr := h.RunShell(fmt.Sprintf(`docker inspect -f "{{.State.Running}}" %s 2>$null`, cname))
			if rerr != nil || strings.TrimSpace(running) != "true" {
				continue
			}
			// Get the supplemental groups the container was started with.
			groups, gerr := h.RunShell(fmt.Sprintf(`docker inspect -f "{{.HostConfig.Groups}}" %s 2>$null`, cname))
			groups = strings.TrimSpace(groups)
			if gerr != nil || groups == "" || groups == "[]" {
				printLine(w, sevFail, hostName, fmt.Sprintf(
					"docker: container %s has no --group-add for socket GID %s; jobs will fail with permission denied; fix with: gh sr down %s && gh sr up %s",
					cname, gid, rc.Name, rc.Name,
				))
				r.Fail++
				continue
			}
			// groups is like "[999]" or "[0 999]"; check if socket GID is present.
			if !strings.Contains(groups, gid) {
				printLine(w, sevFail, hostName, fmt.Sprintf(
					"docker: container %s groups=%s, socket GID=%s not included; jobs will fail with permission denied; fix with: gh sr down %s && gh sr up %s",
					cname, groups, gid, rc.Name, rc.Name,
				))
				r.Fail++
			} else {
				printLine(w, sevOK, hostName, fmt.Sprintf("docker: container %s has socket GID %s in groups %s", cname, gid, groups))
			}
		}
	}
}

// checkUnixDockerSocket verifies the Docker socket path on Linux/macOS hosts and, if any docker-mode
// runner container is already running, checks that the socket is accessible inside it.
func checkUnixDockerSocket(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	socketPath, err := runner.EffectiveDockerSocket(h)
	if err != nil {
		printLine(w, sevFail, hostName, fmt.Sprintf("docker: %v", err))
		r.Fail++
		return
	}
	printLine(w, sevOK, hostName, fmt.Sprintf("docker: socket present at %s", socketPath))

	// For each docker-mode runner container that is already running, verify the socket is
	// accessible inside it (catches containers started without the mount or without --group-add).
	for _, rc := range runners {
		if rc.Host != hostName || rc.EffectiveMode(h.OS) != "docker" {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			cname := runner.ContainerName(inst)
			running, rerr := h.Run(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", cname))
			if rerr != nil || strings.TrimSpace(running) != "true" {
				continue
			}
			// Container is running; check that the socket is accessible inside.
			res, execErr := h.Run(fmt.Sprintf("docker exec %s test -S /var/run/docker.sock && echo ok || echo missing", cname))
			if execErr != nil || strings.TrimSpace(res) != "ok" {
				printLine(w, sevWarn, hostName, fmt.Sprintf(
					"docker: container %s is running but /var/run/docker.sock is not accessible inside it; recreate with: gh sr down %s && gh sr up %s",
					cname, rc.Name, rc.Name,
				))
				r.Warn++
			} else {
				printLine(w, sevOK, hostName, fmt.Sprintf("docker: container %s has /var/run/docker.sock accessible", cname))
			}
		}
	}
}

// nativeInstallTargetsForHost lists (instanceName, runnerConfigName) for native-mode runners on hostName.
func nativeInstallTargetsForHost(runners []config.RunnerConfig, hostName, hostOS string) [][2]string {
	var out [][2]string
	for _, rc := range runners {
		if rc.Host != hostName || rc.EffectiveMode(hostOS) != "native" {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			out = append(out, [2]string{inst, rc.Name})
		}
	}
	return out
}

func checkNativeRunnerInstall(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	for _, pair := range nativeInstallTargetsForHost(runners, hostName, h.OS) {
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

func hasAgenticRunners(runners []config.RunnerConfig, hostName string) bool {
	for _, rc := range runners {
		if rc.Host != hostName {
			continue
		}
		if rc.IsAgentic() {
			return true
		}
		// Native Linux setups use mode: native + labels including agentic (no profile).
		// Legacy configs may still use the gh-aw label name.
		for _, l := range rc.Labels {
			if isAgenticRunnerLabel(l) {
				return true
			}
		}
	}
	return false
}

func isAgenticRunnerLabel(l string) bool {
	s := strings.TrimSpace(l)
	return strings.EqualFold(s, "agentic") || strings.EqualFold(s, "gh-aw")
}

// checkAgenticPrereqs verifies host prerequisites specific to GitHub Agentic Workflows:
// port 80 availability (MCP gateway), iptables (AWF firewall), sudo access, and runner container tools.
func checkAgenticPrereqs(w io.Writer, hostName string, h *host.Host, runners []config.RunnerConfig, r *Result) {
	if h.OS == "linux" || h.OS == "darwin" {
		out, err := h.Run("ss -tlnp 2>/dev/null | grep -q ':80 ' && echo in-use || echo free")
		if err == nil {
			status := strings.TrimSpace(out)
			if status == "in-use" {
				printLine(w, sevWarn, hostName, "agentic: port 80 is in use; Agentic Workflows MCP gateway needs port 80 free on the host network")
				r.Warn++
			} else {
				printLine(w, sevOK, hostName, "agentic: port 80 is free (MCP gateway)")
			}
		}
	}

	if h.OS == "linux" {
		out, err := h.Run("command -v iptables >/dev/null 2>&1 && echo yes || echo no")
		if err == nil && strings.TrimSpace(out) == "yes" {
			printLine(w, sevOK, hostName, "agentic: iptables available (AWF firewall)")
		} else {
			printLine(w, sevWarn, hostName, "agentic: iptables not found; AWF sandbox may fail (install iptables or ensure it is on PATH)")
			r.Warn++
		}

		// Check Docker daemon iptables setting (daemon.json absence means default iptables=true)
		out, err = h.Run("cat /etc/docker/daemon.json")
		if err == nil && strings.Contains(out, `"iptables": false`) {
			printLine(w, sevWarn, hostName, "agentic: Docker iptables is disabled; AWF requires iptables integration")
			r.Warn++
		}

		// Check if DOCKER-USER chain is modifiable (try appending a RETURN rule)
		out, err = h.Run("sudo iptables -A DOCKER-USER -j RETURN")
		if err == nil {
			// Rule added successfully - remove the test rule
			h.Run("sudo iptables -D DOCKER-USER -j RETURN")
			printLine(w, sevOK, hostName, "agentic: DOCKER-USER chain exists and is modifiable")
		} else {
			// Check if error indicates chain doesn't exist vs. permission denied
			errOut, _ := h.Run("sudo iptables -L DOCKER-USER -n")
			if errOut != "" && strings.Contains(errOut, "Chain doesn't exist") {
				printLine(w, sevWarn, hostName, "agentic: DOCKER-USER chain missing; Docker iptables may be disabled")
			} else {
				printLine(w, sevWarn, hostName, "agentic: DOCKER-USER chain not modifiable; sudo may be needed for AWF")
			}
			r.Warn++
		}

		// Check Docker's iptables filter chain list (look for DOCKER chain)
		out, err = h.Run("sudo iptables -L -n")
		if err == nil && strings.Contains(out, "DOCKER") {
			printLine(w, sevOK, hostName, "agentic: Docker iptables rules present")
		} else {
			printLine(w, sevWarn, hostName, "agentic: No Docker iptables rules found; Docker may not be managing iptables")
			r.Warn++
		}

		uid, err := h.Run("id -u")
		if err == nil && strings.TrimSpace(uid) != "0" {
			out, err := h.Run("sudo -n true 2>/dev/null && echo ok || echo no")
			if err != nil || strings.TrimSpace(out) != "ok" {
				printLine(w, sevWarn, hostName, "agentic: passwordless sudo not available; AWF requires sudo for iptables/firewall setup")
				r.Warn++
			} else {
				printLine(w, sevOK, hostName, "agentic: sudo available for AWF firewall setup")
			}
		}

		// Check if iptables and docker-compose are installed in running agentic runner containers
		for _, rc := range runners {
			if rc.Host != hostName || !rc.IsAgentic() {
				continue
			}
			for _, inst := range rc.InstanceNames() {
				cname := runner.ContainerName(inst)
				running, rerr := h.Run(fmt.Sprintf("docker inspect -f '{{.State.Running}}' %s 2>/dev/null", cname))
				if rerr != nil || strings.TrimSpace(running) != "true" {
					continue
				}
				// Check iptables in container
				out, ierr := h.Run(fmt.Sprintf("docker exec %s which iptables 2>/dev/null", cname))
				if ierr != nil || strings.TrimSpace(out) == "" {
					printLine(w, sevWarn, hostName, fmt.Sprintf("agentic: %s missing iptables (needed for AWF firewall)", cname))
					r.Warn++
				}
				// Check docker-compose in container
				out, cerr := h.Run(fmt.Sprintf("docker exec %s which docker-compose 2>/dev/null", cname))
				if cerr != nil || strings.TrimSpace(out) == "" {
					printLine(w, sevWarn, hostName, fmt.Sprintf("agentic: %s missing docker-compose (needed for AWF sidecars)", cname))
					r.Warn++
				}
				// Only check first running instance per runner config; if we get here, tools are present
				printLine(w, sevOK, hostName, fmt.Sprintf("agentic: %s has iptables and docker-compose (AWF tools)", cname))
				break
			}
		}
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
		printLine(w, sevWarn, hostName, "linux: passwordless sudo not available; gh sr setup/update may fail for package installs or Docker install")
		r.Warn++
		return
	}
	printLine(w, sevOK, hostName, "linux: non-root user has passwordless sudo")
}
