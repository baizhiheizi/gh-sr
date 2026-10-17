package registry

import (
	"os"

	"github.com/an-lee/gh-sr/internal/config"
)

// SettingsFromConfig builds Settings from the registry_mirror: section of
// runners.yml. nil when the mirror is disabled — zero-value configs (no Load)
// are disabled; the enabled-by-default default is applied by
// config.applyDefaults.
//
// The mirror list is resolved here: an absent mirrors: list synthesizes the
// legacy single Docker Hub mirror; applyDefaults has already merged the legacy
// port/upstream keys into mirrors[0] and defaulted names and ports.
func SettingsFromConfig(cfg *config.Config) *Settings {
	if cfg == nil || !cfg.RegistryMirrorEnabled() {
		return nil
	}
	s := &Settings{
		Enabled:     true,
		BindAddr:    cfg.RegistryMirror.BindAddr,
		StoragePath: cfg.RegistryMirror.StoragePath,
		Image:       cfg.RegistryMirror.Image,
		URLOverride: cfg.RegistryMirror.URLOverride,
	}
	entries := cfg.RegistryMirrorEntries()
	if len(entries) == 0 {
		s.Mirrors = []MirrorSettings{{
			Name:     config.DefaultRegistryMirrorName,
			Upstream: cfg.RegistryMirror.Upstream,
			Port:     cfg.RegistryMirror.Port,
		}}
		return s
	}
	s.Mirrors = make([]MirrorSettings, len(entries))
	for i, e := range entries {
		s.Mirrors[i] = MirrorSettings{
			Name:     e.Name,
			Upstream: e.Upstream,
			Port:     e.Port,
			Username: e.Username,
			// Resolved here (deploy side), never stored in runners.yml.
			Password: resolvePasswordEnv(e.PasswordEnv),
		}
	}
	return s
}

// resolvePasswordEnv reads the named variable; empty when unset or unnamed.
func resolvePasswordEnv(name string) string {
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}
