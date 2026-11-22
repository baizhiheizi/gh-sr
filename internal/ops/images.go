package ops

import (
	"fmt"
	"io"
	"sort"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/runner"
)

// ImagesSync implements `gh sr images sync`: for every Linux host with
// container-mode runners, ensure the mirrors are deployed, then pull each
// registry_mirror.prefetch image once on the host (through the matching
// mirror) and load it into every runner container's inner dockerd. Hosts are
// synced concurrently (one SSH connection per host), so wall-clock time is
// the slowest host instead of the sum of all hosts. A connect failure still
// fails the command (reported after every host had its turn, no longer
// cancelling the other hosts' syncs); a host that connected but failed to
// sync is best-effort — the last such error is returned.
func ImagesSync(w io.Writer, cfg *config.Config, filterHost string) error {
	prefetch := cfg.RegistryMirrorPrefetch()
	if len(prefetch) == 0 {
		return fmt.Errorf("no registry_mirror.prefetch images configured in runners.yml; add image references to registry_mirror.prefetch or remove this command")
	}
	hosts, err := registryTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	s := registrySettings(cfg)
	var groups []hostGroup
	for _, name := range hosts {
		if len(containerRunnerContainers(cfg, name)) == 0 {
			fmt.Fprintf(w, "Skipping host %s (no container-mode runners)\n", name)
			continue
		}
		// The banner prints in the selection loop, before the fan-out, so
		// every host with container-mode runners gets one — including hosts
		// whose connect later fails (fanOutHostsCore skips the callback on a
		// connect error, which would otherwise drop the banner).
		writeHostBanner(w, "Syncing images on "+name, cfg.Hosts[name].Addr)
		groups = append(groups, hostGroup{name: name})
	}
	results := fanOutHostsCore(w, cfg, groups, false, func(w io.Writer, h *host.Host, g hostGroup) (struct{}, error) {
		if s != nil {
			if err := registry.Ensure(w, h, *s); err != nil {
				return struct{}{}, err
			}
		}
		res, err := registry.SyncPrefetch(w, h, s, prefetch, containerRunnerContainers(cfg, g.name))
		if res.Pulled > 0 || res.Loaded+res.Cached > 0 {
			fmt.Fprintf(w, "  images: %d pulled, %d loaded into runners (%d already cached there)\n",
				res.Pulled, res.Loaded, res.Cached)
		}
		return struct{}{}, err
	})
	for _, r := range results {
		if r.ConnectErr != nil {
			return r.ConnectErr
		}
	}
	var syncErr error
	for _, r := range results {
		if r.Err != nil {
			syncErr = r.Err
		}
	}
	return syncErr
}

// containerRunnerContainers lists the Docker container names of every
// container-mode runner instance on one host.
func containerRunnerContainers(cfg *config.Config, hostName string) []string {
	var out []string
	for _, rc := range cfg.RunnersForHost(hostName) {
		if !rc.IsContainerMode() {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			out = append(out, runner.ContainerDockerName(inst))
		}
	}
	sort.Strings(out)
	return out
}

// syncPrefetchAfterStart runs one prefetch sync pass over the given runners'
// hosts after `gh sr up` / `gh sr rebuild` has started the containers. It is a
// no-op without registry_mirror.prefetch entries, and best-effort by design:
// a sync failure logs a warning instead of failing the up/rebuild that
// preceded it (runners are already running at that point). Hosts sync
// concurrently (one SSH connection per host), so wall-clock time is the
// slowest host instead of the sum of all hosts.
func syncPrefetchAfterStart(w io.Writer, cfg *config.Config, runners []config.RunnerConfig) error {
	prefetch := cfg.RegistryMirrorPrefetch()
	if len(prefetch) == 0 {
		return nil
	}
	s := registrySettings(cfg)
	var groups []hostGroup
	for _, g := range groupRunnersByHost(runners) {
		if len(containerRunnerContainers(cfg, g.name)) == 0 {
			continue
		}
		if hcfg := cfg.Hosts[g.name]; hcfg.OS != "" && hcfg.OS != "linux" {
			continue
		}
		groups = append(groups, g)
	}
	results := fanOutHostsCore(w, cfg, groups, false, func(w io.Writer, h *host.Host, g hostGroup) (struct{}, error) {
		fmt.Fprintf(w, "Prefetching images on %s...\n", g.name)
		if s != nil {
			if err := registry.Ensure(w, h, *s); err != nil {
				fmt.Fprintf(w, "Warning: image sync skipped on %s: %v\n", g.name, err)
				return struct{}{}, nil
			}
		}
		res, err := registry.SyncPrefetch(w, h, s, prefetch, containerRunnerContainers(cfg, g.name))
		if err != nil {
			fmt.Fprintf(w, "Warning: image sync on %s: %v\n", g.name, err)
			return struct{}{}, nil
		}
		fmt.Fprintf(w, "  images: %d pulled, %d loaded into runners (%d already cached there)\n",
			res.Pulled, res.Loaded, res.Cached)
		return struct{}{}, nil
	})
	// Best-effort: a connect failure warns (with the skip wording) and the
	// function still returns nil.
	for _, r := range results {
		if r.ConnectErr != nil {
			fmt.Fprintf(w, "Warning: image sync skipped on %s: %v\n", r.Name, r.ConnectErr)
		}
	}
	return nil
}
