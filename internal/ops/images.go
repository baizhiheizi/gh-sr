package ops

import (
	"fmt"
	"io"
	"sort"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/runner"
)

// ImagesSync implements `gh sr images sync`: for every Linux host with
// container-mode runners, ensure the mirrors are deployed, then pull each
// registry_mirror.prefetch image once on the host (through the matching
// mirror) and load it into every runner container's inner dockerd.
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
	var syncErr error
	for _, name := range hosts {
		containers := containerRunnerContainers(cfg, name)
		if len(containers) == 0 {
			fmt.Fprintf(w, "Skipping host %s (no container-mode runners)\n", name)
			continue
		}
		writeHostBanner(w, "Syncing images on "+name, cfg.Hosts[name].Addr)
		h, err := connectHostFn(name, cfg.Hosts[name])
		if err != nil {
			return err
		}
		err = func() error {
			defer h.Close()
			if s != nil {
				if err := registry.Ensure(w, h, *s); err != nil {
					return err
				}
			}
			res, err := registry.SyncPrefetch(w, h, s, prefetch, containers)
			if res.Pulled+res.Skipped > 0 || res.Loaded+res.Cached > 0 {
				fmt.Fprintf(w, "  images: %d pulled (%d already on host), %d loaded into runners (%d cached)\n",
					res.Pulled, res.Skipped, res.Loaded, res.Cached)
			}
			return err
		}()
		if err != nil {
			syncErr = err
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
// preceded it (runners are already running at that point).
func syncPrefetchAfterStart(w io.Writer, cfg *config.Config, runners []config.RunnerConfig) error {
	prefetch := cfg.RegistryMirrorPrefetch()
	if len(prefetch) == 0 {
		return nil
	}
	s := registrySettings(cfg)
	done := make(map[string]bool)
	for _, g := range groupRunnersByHost(runners) {
		if done[g.name] {
			continue
		}
		done[g.name] = true
		containers := containerRunnerContainers(cfg, g.name)
		if len(containers) == 0 {
			continue
		}
		hcfg := cfg.Hosts[g.name]
		if hcfg.OS != "" && hcfg.OS != "linux" {
			continue
		}
		h, err := connectHostFn(g.name, hcfg)
		if err != nil {
			fmt.Fprintf(w, "Warning: image sync skipped on %s: %v\n", g.name, err)
			continue
		}
		func() {
			defer h.Close()
			fmt.Fprintf(w, "Prefetching images on %s...\n", g.name)
			if s != nil {
				if err := registry.Ensure(w, h, *s); err != nil {
					fmt.Fprintf(w, "Warning: image sync skipped on %s: %v\n", g.name, err)
					return
				}
			}
			res, err := registry.SyncPrefetch(w, h, s, prefetch, containers)
			if err != nil {
				fmt.Fprintf(w, "Warning: image sync on %s: %v\n", g.name, err)
				return
			}
			fmt.Fprintf(w, "  images: %d pulled (%d already on host), %d loaded into runners (%d cached)\n",
				res.Pulled, res.Skipped, res.Loaded, res.Cached)
		}()
	}
	return nil
}
