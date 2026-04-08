package doctor

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
	"github.com/an-lee/ghr/internal/runner"
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
// tokenSource indicates how the GitHub token was obtained ("pat", "gh", or "" if unavailable).
// If cfg is nil (load error), GitHub and host checks are skipped after the configuration section.
func Run(w io.Writer, cfgPath, envPath string, cfg *config.Config, cfgErr error, gh *runner.GitHubClient, tokenSource, filterHost, filterRepo string, strict bool) Result {
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
		printLine(w, sevWarn, "local", fmt.Sprintf("env file not found: %s (optional if secrets are exported)", envPath))
		r.Warn++
	case err != nil:
		printLine(w, sevWarn, "local", fmt.Sprintf("env file: %v", err))
		r.Warn++
	default:
		printLine(w, sevOK, "local", fmt.Sprintf("env file present: %s", envPath))
	}

	switch tokenSource {
	case config.TokenSourcePAT:
		printLine(w, sevOK, "local", "GitHub token: from PAT (config or environment)")
	case config.TokenSourceGH:
		printLine(w, sevOK, "local", "GitHub token: from gh CLI (gh auth login)")
	default:
		printLine(w, sevFail, "local", "GitHub token: not found; set github.pat in runners.yml, export GITHUB_PAT, or run `gh auth login`")
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

	fmt.Fprintln(w, "\n=== Hosts ===")
	fmt.Fprintf(w, "Docker mode uses image: %s\n\n", runner.RunnerDockerImage)
	hostOrder := uniqueHostNames(runners)
	for _, hostName := range hostOrder {
		hcfg := cfg.Hosts[hostName]
		modes := modesForHost(runners, hostName, hcfg.OS)

		h := host.NewHost(hostName, hcfg)
		if err := h.Connect(); err != nil {
			printLine(w, sevFail, hostName, fmt.Sprintf("connect: %v", err))
			r.Fail++
			continue
		}
		func() {
			defer h.Close()
			printLine(w, sevOK, hostName, fmt.Sprintf("connected (%s)", addrSummary(hcfg.Addr)))
			if modes["docker"] {
				checkDocker(w, hostName, h, hcfg, runners, &r)
			}
			if modes["native"] {
				checkNative(w, hostName, h, &r)
				checkNativeRunnerInstall(w, hostName, h, hcfg, runners, &r)
			}
			if h.OS == "linux" {
				checkLinuxSudo(w, hostName, h, &r)
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

func uniqueRepos(runners []config.RunnerConfig) []string {
	seen := make(map[string]struct{})
	for _, rc := range runners {
		seen[rc.Repo] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for repo := range seen {
		out = append(out, repo)
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

func checkDocker(w io.Writer, hostName string, h *host.Host, hcfg config.HostConfig, runners []config.RunnerConfig, r *Result) {
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
		checkUnixDockerSocket(w, hostName, h, hcfg, runners, r)
	}
}

// checkUnixDockerSocket verifies the Docker socket path on Linux/macOS hosts and, if any docker-mode
// runner container is already running, checks that the socket is accessible inside it.
func checkUnixDockerSocket(w io.Writer, hostName string, h *host.Host, hcfg config.HostConfig, runners []config.RunnerConfig, r *Result) {
	socketPath := hcfg.DockerSocket
	if socketPath == "" {
		socketPath = runner.DefaultDockerSocket
	}

	// Verify the socket exists on the host.
	out, err := h.Run(fmt.Sprintf("test -S %s && echo ok || echo missing", socketPath))
	if err != nil || strings.TrimSpace(out) != "ok" {
		hint := "ensure Docker daemon is running (rootless Docker? set docker_socket in config)"
		if h.OS == "darwin" {
			hint = "ensure Docker Desktop/OrbStack/Colima is running (non-default socket path? set docker_socket in config)"
		}
		printLine(w, sevFail, hostName, fmt.Sprintf(
			"docker: socket not found at %s; %s", socketPath, hint,
		))
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
					"docker: container %s is running but /var/run/docker.sock is not accessible inside it; recreate with: ghr down %s && ghr up %s",
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

func checkNativeRunnerInstall(w io.Writer, hostName string, h *host.Host, hcfg config.HostConfig, runners []config.RunnerConfig, r *Result) {
	for _, pair := range nativeInstallTargetsForHost(runners, hostName, hcfg.OS) {
		inst, runnerName := pair[0], pair[1]
		dir := h.RunnerDir(inst)
		ok, err := runner.NativeRunnerConfigPresent(h, inst)
		if err != nil {
			printLine(w, sevFail, hostName, fmt.Sprintf("native: instance %s: %v", inst, err))
			r.Fail++
			continue
		}
		if !ok {
			printLine(w, sevFail, hostName, fmt.Sprintf("native: instance %s not installed (missing .runner under %s); run: ghr setup %s", inst, dir, runnerName))
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
		printLine(w, sevWarn, hostName, "linux: passwordless sudo not available; ghr setup/update may fail for package installs or Docker install")
		r.Warn++
		return
	}
	printLine(w, sevOK, hostName, "linux: non-root user has passwordless sudo")
}
