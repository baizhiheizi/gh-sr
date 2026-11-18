package ops

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/registry"
)

// registryDescriptor describes the per-host registry pull-through mirrors to
// the shared per-host service orchestrators. The registry is a list of
// mirrors, so its inspect returns one status row per mirror.
var registryDescriptor = serviceDescriptor[registry.Settings, registry.MirrorStatus]{
	label:     "registry",
	title:     "Registry",
	configKey: "registry_mirror",
	workload:  "the registry mirror",
	settings:  registrySettings,
	ensure:    registry.Ensure,
	inspect:   registry.Inspect,
	print:     printRegistryStatus,
	remove:    registry.Remove,
}

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
	return deployService(w, cfg, filterHost, registryDescriptor)
}

// RegistryStatus reports the per-host registry mirror state, health, and
// storage.
func RegistryStatus(w io.Writer, cfg *config.Config, filterHost string) error {
	return statusService(w, cfg, filterHost, registryDescriptor)
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
	fmt.Fprintf(w, "  registry%s: state: %s\n", name, serviceState(m.Info))
	fmt.Fprintf(w, "           image: %s\n", m.Info.Image)
	if m.Info.URL != "" {
		fmt.Fprintf(w, "           url:   %s\n", m.Info.URL)
	}
	fmt.Fprintf(w, "           store: %s\n", serviceStorage(m.Info.StoragePath, m.Info.StorageBytes))
}

// RegistryRemove uninstalls the registry mirror; purgeData also deletes the
// mirrored layers. Runner removal (gh sr remove) never touches the mirror —
// this command is the only uninstall path.
func RegistryRemove(w io.Writer, cfg *config.Config, filterHost string, purgeData bool) error {
	return removeService(w, cfg, filterHost, purgeData, registryDescriptor)
}
