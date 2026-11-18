package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostsvc"
	"github.com/an-lee/gh-sr/internal/runner"
)

// serviceDescriptor describes one managed per-host Docker service — the local
// Actions cache server, the registry pull-through mirrors — so the deploy,
// status and remove choreography lives in one module instead of being copied
// per service.
//
// The two services are not the same service: the cache server is a single
// container with a management API to prune, the registry is a list of mirrors
// with one container each. What they share is the shape of the choreography —
// resolve the settings, narrow to the Linux target hosts, connect to each,
// run the service's own operation, render the shared status fields. The
// descriptor carries that shape plus the per-service parts, so adding a third
// managed service is a descriptor literal instead of a fourth copy.
//
// S is the service's Settings type; I the status row type it reports (one row
// for the cache server, one row per mirror for the registry).
type serviceDescriptor[S any, I any] struct {
	// label names the service in user-facing output ("cache", "registry").
	label string
	// title names the service in the status banner ("Cache", "Registry").
	title string
	// configKey is the service's runners.yml section ("cache",
	// "registry_mirror"); it appears in the "disabled" error.
	configKey string
	// workload names the service in the skip note ("the cache server").
	workload string
	// settings resolves the effective settings from runners.yml. A nil
	// result means the service is disabled.
	settings func(*config.Config) *S
	// ensure deploys the service container(s) idempotently.
	ensure func(io.Writer, *host.Host, S) error
	// inspect reports the service's status rows.
	inspect func(*host.Host, S) ([]I, error)
	// print renders one status row.
	print func(io.Writer, I)
	// remove uninstalls the service container(s); purgeData also deletes the
	// service's stored data.
	remove func(io.Writer, *host.Host, S, bool) error
}

// deployService ensures the service's containers exist and run on every
// target host. A service disabled in runners.yml is an error here: deploying
// is an explicit request, so silently doing nothing would hide the mistake.
func deployService[S any, I any](w io.Writer, cfg *config.Config, filterHost string, d serviceDescriptor[S, I]) error {
	if d.settings(cfg) == nil {
		return fmt.Errorf("%s is disabled in runners.yml (%s.enabled: false); enable it or remove the setting", d.configKey, d.configKey)
	}
	hosts, err := serviceTargets(w, cfg, filterHost, d.workload)
	if err != nil {
		return err
	}
	return forEachServiceHost(w, cfg, hosts, "Deploying "+d.label, d.settings, func(w io.Writer, h *host.Host, s *S) error {
		return d.ensure(w, h, *s)
	})
}

// statusService reports the service's state, health and storage on every
// target host. A service disabled in runners.yml is reported per host rather
// than treated as an error: status is a query, not a request to change
// anything.
func statusService[S any, I any](w io.Writer, cfg *config.Config, filterHost string, d serviceDescriptor[S, I]) error {
	hosts, err := serviceTargets(w, cfg, filterHost, d.workload)
	if err != nil {
		return err
	}
	return forEachServiceHost(w, cfg, hosts, d.title+" status", d.settings, func(w io.Writer, h *host.Host, s *S) error {
		if s == nil {
			fmt.Fprintf(w, "  %s: disabled in runners.yml\n", d.label)
			return nil
		}
		rows, err := d.inspect(h, *s)
		if err != nil {
			return err
		}
		for _, row := range rows {
			d.print(w, row)
		}
		return nil
	})
}

// removeService uninstalls the service's containers on every target host.
// purgeData also deletes the service's stored data.
func removeService[S any, I any](w io.Writer, cfg *config.Config, filterHost string, purgeData bool, d serviceDescriptor[S, I]) error {
	hosts, err := serviceTargets(w, cfg, filterHost, d.workload)
	if err != nil {
		return err
	}
	return forEachServiceHost(w, cfg, hosts, "Removing "+d.label, d.settings, func(w io.Writer, h *host.Host, s *S) error {
		// A service disabled in runners.yml still has containers on the
		// host from an earlier deploy: remove them with the zero-value
		// settings rather than skipping the host.
		var settings S
		if s != nil {
			settings = *s
		}
		return d.remove(w, h, settings, purgeData)
	})
}

// serviceState renders a service's state, appending "(healthy)" when its
// health endpoint answered.
func serviceState(info hostsvc.StatusInfo) string {
	state := info.State
	if info.Healthy {
		state += " (healthy)"
	}
	return state
}

// serviceStorage renders a service's storage path with its human-readable
// size.
func serviceStorage(path string, bytes int64) string {
	return fmt.Sprintf("%s (%s)", path, runner.FormatBytesHuman(bytes))
}
