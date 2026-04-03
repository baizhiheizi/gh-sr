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
		printLine(w, sevWarn, "local", fmt.Sprintf("env file not found: %s (optional if secrets are exported)", envPath))
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
				checkDocker(w, hostName, h, &r)
			}
			if modes["native"] {
				checkNative(w, hostName, h, &r)
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

func checkDocker(w io.Writer, hostName string, h *host.Host, r *Result) {
	var out string
	var err error
	if h.OS == "windows" {
		out, err = h.RunShell(`docker info --format "{{.ServerVersion}}"`)
	} else {
		out, err = h.Run("docker info --format '{{.ServerVersion}}' 2>/dev/null || echo 'not found'")
	}
	out = strings.TrimSpace(out)
	if err != nil || out == "" || strings.Contains(out, "not found") {
		reason := "docker info did not return a server version"
		if err != nil {
			reason = err.Error()
		}
		printLine(w, sevFail, hostName, fmt.Sprintf("docker: daemon/CLI not usable (%s); install and start Docker (see README \"Host setup\")", reason))
		r.Fail++
		return
	}
	printLine(w, sevOK, hostName, fmt.Sprintf("docker: server version %s (image %s)", out, runner.RunnerDockerImage))
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
