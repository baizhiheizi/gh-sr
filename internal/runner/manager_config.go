package runner

import (
	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/registry"
)

// NewManagerFromConfig builds a Manager fully wired from cfg: the GitHub
// client from pat plus every container-image, cache, registry-mirror and
// shared-cache setting.
//
// This constructor is the single place the config-to-Manager mapping lives.
// The Manager owns these fields, so the mapping belongs in this module, not in
// the orchestrators that consume it. Building the complete value once means
// every orchestrator receives a fully configured Manager and no call site has
// to remember to populate the fields — the "forgotten wiring call" bug class
// (a late CollectStatus once reported every container image as stale
// indefinitely) is gone by construction.
//
// cfg may be nil, in which case all derived fields take their zero values
// (native-only and unit-test contexts).
func NewManagerFromConfig(pat string, cfg *config.Config) *Manager {
	m := NewManager(pat)
	if m == nil || cfg == nil {
		return m
	}
	m.ContainerImageBaseImage = cfg.ContainerRunnerImageBaseImage()
	m.ContainerImageExtraApt = cfg.ContainerRunnerImageExtraAptPackages()
	m.ContainerImageToolcache = cfg.ContainerRunnerImageToolcache()
	m.ContainerMTU = cfg.ContainerRunnerImageMTU()
	m.ContainerDockerdStartTimeout = cfg.ContainerRunnerImageDockerdStartTimeout()
	m.ContainerBootstrapMaxRetries = cfg.ContainerRunnerImageBootstrapMaxRetries()
	m.ContainerStartStaggerSeconds = cfg.ContainerRunnerImageStartStaggerSeconds()
	m.Cache = cache.SettingsFromConfig(cfg)
	m.RegistryMirror = registry.SettingsFromConfig(cfg)
	m.NPMRegistry = cfg.NPMRegistry
	m.SharedNPMCache = cfg.SharedNPMCacheEnabled()
	m.SharedRunnerCache = cfg.SharedHomeCacheEnabled()
	m.SharedGradleCache = cfg.SharedGradleCacheEnabled()
	m.SharedPubCache = cfg.SharedPubCacheEnabled()
	m.SharedMiseData = cfg.SharedMiseDataEnabled()
	m.SharedAndroidSDK = cfg.SharedAndroidSDKEnabled()
	return m
}
