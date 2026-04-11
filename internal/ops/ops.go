package ops

import (
	"fmt"
	"io"
	"sync"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
)

// ConnectHost opens a connection to the configured host (SSH or local).
func ConnectHost(hostName string, hcfg config.HostConfig) (*host.Host, error) {
	h := host.NewHost(hostName, hcfg)
	if err := h.Connect(); err != nil {
		return nil, err
	}
	return h, nil
}

// Setup installs and configures runners, mirroring the gh sr setup command.
func Setup(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	hostsDone := map[string]bool{}
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		if hostsDone[rc.Host] {
			continue
		}

		if config.IsLocalAddr(hcfg.Addr) {
			fmt.Fprintf(w, "Setting up on %s (local)...\n", rc.Host)
		} else {
			fmt.Fprintf(w, "Setting up on %s (%s)...\n", rc.Host, hcfg.Addr)
		}
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		if err := mgr.Setup(h, rc); err != nil {
			h.Close()
			return err
		}
		h.Close()
		hostsDone[rc.Host] = true
	}

	fmt.Fprintln(w, "\nSetup complete.")
	fmt.Fprintln(w, "Start runners with: gh sr up [runner-names...] (setup registers the runner; up launches the listener.)")
	return nil
}

// Up starts runners, automatically running setup first if needed.
func Up(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		fmt.Fprintf(w, "Starting %s on %s...\n", rc.Name, rc.Host)
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		if err := mgr.EnsureSetup(h, rc); err != nil {
			h.Close()
			return err
		}
		if err := mgr.Start(h, rc); err != nil {
			h.Close()
			return err
		}
		h.Close()
	}
	return nil
}

// Down stops runners.
func Down(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		fmt.Fprintf(w, "Stopping %s on %s...\n", rc.Name, rc.Host)
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		if err := mgr.Stop(h, rc); err != nil {
			h.Close()
			return err
		}
		h.Close()
	}
	return nil
}

// Restart stops then starts runners.
func Restart(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		fmt.Fprintf(w, "Restarting %s on %s...\n", rc.Name, rc.Host)
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		_ = mgr.Stop(h, rc)
		if err := mgr.Start(h, rc); err != nil {
			h.Close()
			return err
		}
		h.Close()
	}
	return nil
}

// Update removes, sets up, and starts runners again.
func Update(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) error {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	for _, rc := range runners {
		hcfg := cfg.Hosts[rc.Host]
		fmt.Fprintf(w, "Updating %s on %s...\n", rc.Name, rc.Host)
		h, err := ConnectHost(rc.Host, hcfg)
		if err != nil {
			return err
		}
		_ = mgr.Remove(h, rc)
		if err := mgr.Setup(h, rc); err != nil {
			h.Close()
			return err
		}
		if err := mgr.Start(h, rc); err != nil {
			h.Close()
			return err
		}
		h.Close()
	}

	fmt.Fprintln(w, "\nUpdate complete.")
	return nil
}

// CollectStatus gathers runner status rows like gh sr status.
//
// Runners are grouped by host so that each host requires only one SSH connection.
// All host groups are queried concurrently, reducing wall-clock time from
// O(N_hosts × SSH_latency) to O(SSH_latency) for multi-host configurations.
func CollectStatus(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) ([]runner.RunnerStatus, error) {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return nil, err
	}
	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)

	// Group runners by host, preserving original order for deterministic output.
	type hostGroup struct {
		name    string
		runners []config.RunnerConfig
	}
	var groups []hostGroup
	groupIdx := make(map[string]int)
	for _, rc := range runners {
		if i, ok := groupIdx[rc.Host]; ok {
			groups[i].runners = append(groups[i].runners, rc)
		} else {
			groupIdx[rc.Host] = len(groups)
			groups = append(groups, hostGroup{name: rc.Host, runners: []config.RunnerConfig{rc}})
		}
	}

	type groupResult struct {
		statuses []runner.RunnerStatus
		err      error
	}
	results := make([]groupResult, len(groups))

	var wg sync.WaitGroup
	var wMu sync.Mutex // guards writes to w

	for i, g := range groups {
		wg.Add(1)
		go func(i int, g hostGroup) {
			defer wg.Done()
			hcfg := cfg.Hosts[g.name]
			h, err := ConnectHost(g.name, hcfg)
			if err != nil {
				if w != nil {
					wMu.Lock()
					fmt.Fprintf(w, "Warning: cannot connect to %s: %v\n", g.name, err)
					wMu.Unlock()
				}
				var unreachable []runner.RunnerStatus
				for _, rc := range g.runners {
					for _, name := range rc.InstanceNames() {
						repoDisplay := rc.Repo
						if rc.Org != "" {
							repoDisplay = "org:" + rc.Org
						}
						unreachable = append(unreachable, runner.RunnerStatus{
							Instance: name,
							Host:     rc.Host,
							Repo:     repoDisplay,
							Mode:     "native",
							Local:    "unreachable",
						})
					}
				}
				results[i] = groupResult{statuses: unreachable}
				return
			}
			defer h.Close()

			var statuses []runner.RunnerStatus
			for _, rc := range g.runners {
				s, err := mgr.Status(h, rc)
				if err != nil {
					results[i] = groupResult{err: err}
					return
				}
				statuses = append(statuses, s...)
			}
			results[i] = groupResult{statuses: statuses}
		}(i, g)
	}
	wg.Wait()

	var allStatuses []runner.RunnerStatus
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		allStatuses = append(allStatuses, r.statuses...)
	}

	mgr.EnrichWithGitHubStatus(allStatuses, cfg)
	return allStatuses, nil
}

// Logs returns recent log lines for a runner instance or base name.
func Logs(cfg *config.Config, mgr *runner.Manager, filterHost, target string) (string, error) {
	if err := ResolveHostInfo(nil, cfg); err != nil {
		return "", err
	}
	rc, err := cfg.FindRunnerForLogs(target, filterHost)
	if err != nil {
		return "", err
	}
	instance, err := rc.ResolveRunnerInstance(target)
	if err != nil {
		return "", err
	}
	hcfg := cfg.Hosts[rc.Host]
	h, err := ConnectHost(rc.Host, hcfg)
	if err != nil {
		return "", err
	}
	defer h.Close()
	return mgr.Logs(h, *rc, instance)
}

// CleanupOffline removes offline runners via the GitHub API.
func CleanupOffline(w io.Writer, cfg *config.Config, mgr *runner.Manager) (int, error) {
	fmt.Fprintln(w, "Cleaning up offline runners...")
	removed, err := mgr.CleanupOffline(cfg)
	if err != nil {
		return 0, err
	}
	if removed == 0 {
		fmt.Fprintln(w, "No offline runners found.")
	} else {
		fmt.Fprintf(w, "Removed %d offline runner(s).\n", removed)
	}
	return removed, nil
}
