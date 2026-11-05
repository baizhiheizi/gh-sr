package ops

import (
	"fmt"
	"io"
	"sort"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// serviceTargets returns the sorted host names a per-host Docker service
// command (cache server, registry mirror) should run on: Linux hosts only
// (these services are Linux/Docker workloads), optionally narrowed by --host.
// workload names the service in the skip note, e.g. "the cache server".
func serviceTargets(w io.Writer, cfg *config.Config, filterHost, workload string) ([]string, error) {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return nil, err
	}
	var hosts []string
	for name, h := range cfg.Hosts {
		if filterHost != "" && name != filterHost {
			continue
		}
		if h.OS != "linux" {
			fmt.Fprintf(w, "Skipping host %s (os: %s; %s requires Linux)\n", name, h.OS, workload)
			continue
		}
		hosts = append(hosts, name)
	}
	sort.Strings(hosts)
	return hosts, nil
}

// forEachServiceHost runs fn for each target host sequentially with the shared
// lifecycle every per-host service orchestrator repeats: banner → connect →
// fn → close. Connect and fn errors are fail-fast (the loop aborts and the
// error propagates), matching the pre-existing per-orchestrator loops. fn
// receives the effective service settings resolved once by settings (nil when
// the service is disabled in runners.yml) so it can render the per-host
// disabled note itself.
func forEachServiceHost[T any](w io.Writer, cfg *config.Config, hosts []string, bannerVerb string, settings func(*config.Config) *T, fn func(w io.Writer, h *host.Host, s *T) error) error {
	s := settings(cfg)
	for _, name := range hosts {
		writeHostBanner(w, bannerVerb+" on "+name, cfg.Hosts[name].Addr)
		h, err := connectHostFn(name, cfg.Hosts[name])
		if err != nil {
			return err
		}
		err = fn(w, h, s)
		h.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ensureServiceForHost deploys the per-host service container before container
// runner work via the service's Ensure. A nil settings (service disabled in
// runners.yml) is a no-op. Idempotent and cheap when already running (one
// docker inspect).
func ensureServiceForHost[T any](w io.Writer, h *host.Host, s *T, ensure func(io.Writer, *host.Host, T) error) error {
	if s == nil {
		return nil
	}
	return ensure(w, h, *s)
}
