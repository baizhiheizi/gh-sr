package ops

import (
	"fmt"
	"io"

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

// registryTargets returns the sorted host names a registry command should run
// on, narrowed by serviceTargets' shared Linux-only/--host filtering.
func registryTargets(w io.Writer, cfg *config.Config, filterHost string) ([]string, error) {
	return serviceTargets(w, cfg, filterHost, "the registry mirror")
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
	return forEachServiceHost(w, cfg, hosts, "Deploying registry", registrySettings, func(w io.Writer, h *host.Host, s *registry.Settings) error {
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
	return forEachServiceHost(w, cfg, hosts, "Registry status", registrySettings, func(w io.Writer, h *host.Host, s *registry.Settings) error {
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
	return forEachServiceHost(w, cfg, hosts, "Removing registry", registrySettings, func(w io.Writer, h *host.Host, s *registry.Settings) error {
		var settings registry.Settings
		if s != nil {
			settings = *s
		}
		return registry.Remove(w, h, settings, purgeData)
	})
}
