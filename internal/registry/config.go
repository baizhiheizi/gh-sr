package registry

import "github.com/an-lee/gh-sr/internal/config"

// SettingsFromConfig builds Settings from the registry_mirror: section of
// runners.yml. nil when the mirror is disabled — zero-value configs (no Load)
// are disabled; the enabled-by-default default is applied by
// config.applyDefaults.
func SettingsFromConfig(cfg *config.Config) *Settings {
	if cfg == nil || !cfg.RegistryMirrorEnabled() {
		return nil
	}
	return &Settings{
		Enabled:     true,
		Port:        cfg.RegistryMirror.Port,
		BindAddr:    cfg.RegistryMirror.BindAddr,
		StoragePath: cfg.RegistryMirror.StoragePath,
		Image:       cfg.RegistryMirror.Image,
		Upstream:    cfg.RegistryMirror.Upstream,
		URLOverride: cfg.RegistryMirror.URLOverride,
	}
}
