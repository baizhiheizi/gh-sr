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

// registrySettings builds the effective registry-mirror settings from
// runners.yml. nil when the mirror is disabled (zero-value Configs without a
// Load are disabled — the enabled default is applied by config.applyDefaults).
func registrySettings(cfg *config.Config) *registry.Settings {
	return registry.SettingsFromConfig(cfg)
}

// ensureRegistryForHost deploys the per-host registry mirror container before
// container runner work. Idempotent and cheap when already running (one docker
// inspect).
func ensureRegistryForHost(w io.Writer, h *host.Host, s *registry.Settings) error {
	if s == nil {
		return nil
	}
	return registry.Ensure(w, h, *s)
}

// registryTargets returns the sorted host names a registry command should run
// on: Linux hosts only (the mirror is a Linux/Docker workload), optionally
// narrowed by --host.
func registryTargets(w io.Writer, cfg *config.Config, filterHost string) ([]string, error) {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return nil, err
	}
	var hosts []string
	for name, h := range cfg.Hosts {
		if filterHost != "" && name != filterHost {
			continue
		}
		if h.OS != "linux" {
			fmt.Fprintf(w, "Skipping host %s (os: %s; the registry mirror requires Linux)\n", name, h.OS)
			continue
		}
		hosts = append(hosts, name)
	}
	sort.Strings(hosts)
	return hosts, nil
}

// forEachRegistryHost mirrors forEachCacheHost's lifecycle: banner → connect →
// fn → close, fail-fast on errors. fn receives the effective mirror settings
// (nil when disabled in runners.yml) so it can render the per-host disabled
// note itself.
func forEachRegistryHost(w io.Writer, cfg *config.Config, hosts []string, bannerVerb string, fn func(w io.Writer, h *host.Host, s *registry.Settings) error) error {
	s := registrySettings(cfg)
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

// RegistryDeploy ensures the per-host registry mirror container exists and
// runs.
func RegistryDeploy(w io.Writer, cfg *config.Config, filterHost string) error {
	if registrySettings(cfg) == nil {
		return fmt.Errorf("registry_mirror is disabled in runners.yml (registry_mirror.enabled: false); enable it or remove the setting")
	}
	hosts, err := registryTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	return forEachRegistryHost(w, cfg, hosts, "Deploying registry", func(w io.Writer, h *host.Host, s *registry.Settings) error {
		return registry.Ensure(w, h, *s)
	})
}

// RegistryStatus reports the per-host registry mirror state, health, and
// storage.
func RegistryStatus(w io.Writer, cfg *config.Config, filterHost string) error {
	hosts, err := registryTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	return forEachRegistryHost(w, cfg, hosts, "Registry status", func(w io.Writer, h *host.Host, s *registry.Settings) error {
		if s == nil {
			fmt.Fprintf(w, "  registry: disabled in runners.yml\n")
			return nil
		}
		mirrors, err := registry.Inspect(h, *s)
		if err != nil {
			return err
		}
		for _, m := range mirrors {
			printRegistryStatus(w, m)
		}
		return nil
	})
}

func printRegistryStatus(w io.Writer, m registry.MirrorStatus) {
	name := ""
	if m.Name != "" && m.Name != config.DefaultRegistryMirrorName {
		name = "[" + m.Name + "]"
	}
	if m.Info.State == "" {
		fmt.Fprintf(w, "  registry%s: not deployed\n", name)
		return
	}
	state := m.Info.State
	if m.Info.Healthy {
		state += " (healthy)"
	}
	fmt.Fprintf(w, "  registry%s: state: %s\n", name, state)
	fmt.Fprintf(w, "           image: %s\n", m.Info.Image)
	if m.Info.URL != "" {
		fmt.Fprintf(w, "           url:   %s\n", m.Info.URL)
	}
	fmt.Fprintf(w, "           store: %s (%s)\n", m.Info.StoragePath, runner.FormatBytesHuman(m.Info.StorageBytes))
}

// RegistryRemove uninstalls the registry mirror; purgeData also deletes the
// mirrored layers. Runner removal (gh sr remove) never touches the mirror —
// this command is the only uninstall path.
func RegistryRemove(w io.Writer, cfg *config.Config, filterHost string, purgeData bool) error {
	hosts, err := registryTargets(w, cfg, filterHost)
	if err != nil {
		return err
	}
	return forEachRegistryHost(w, cfg, hosts, "Removing registry", func(w io.Writer, h *host.Host, s *registry.Settings) error {
		var settings registry.Settings
		if s != nil {
			settings = *s
		}
		return registry.Remove(w, h, settings, purgeData)
	})
}
