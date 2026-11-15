// Package sharedcache owns the single definition of the shared runner cache
// volumes (runners.yml shared_caches): one Cache row per cache, and the
// resolution of a SharedCachesConfig into the enabled set. Every consumer
// derives from All or Enabled — the container create args, the doctor's
// mount expectations, the Manager wiring — so adding a cache is one row
// here, and the mount expectations doctor checks against come from the same
// row the container module mounts from, not a re-typed copy.
//
// The named volumes are host-wide and survive `gh sr rebuild` (only an
// explicit `docker volume rm` drops them — the rebuild flow removes
// containers and images but never volumes). The first container that
// downloads a package or tool warms the volume for every other container on
// the host.
//
// gradle / pub / mise mount at the tools' own default cache locations
// (GRADLE_USER_HOME / PUB_CACHE / MISE_DATA_DIR), so no env var has to
// survive the entrypoint's `su -` login shell. The Android SDK has no
// default location on the image, so its row carries an env export
// (GH_SR_ANDROID_SDK_ROOT, propagated by the entrypoint into the runner .env
// as ANDROID_SDK_ROOT / ANDROID_HOME — docker -e vars alone would not
// survive the login-shell environment reset).
package sharedcache

import (
	"github.com/an-lee/gh-sr/internal/config"
)

// Cache is one shared runner cache: a named Docker volume on the host
// daemon, mounted into every container-mode runner instance at Mount.
type Cache struct {
	// Key is the runners.yml shared_caches sub-flag this row answers to.
	Key string
	// Volume is the host-wide named Docker volume. The gh-sr- prefix is a
	// contract: the `gh sr` disk report filters volume rows on it.
	Volume string
	// Mount is the container path the volume is mounted at.
	Mount string
	// Env, when non-empty, is exported at docker create as Env=Mount for the
	// entrypoint to propagate into the runner .env. Most caches mount at the
	// tool's own default location and carry no env export.
	Env string
	// Default resolves the row's sub-flag when runners.yml leaves it unset.
	// Every cache defaults on except android_sdk, which is opt-in: the SDK
	// is large, and pointing ANDROID_SDK_ROOT at a shared volume changes
	// behavior for repos that install their own SDK at job time.
	Default bool

	// field selects the row's runners.yml sub-flag from the config section.
	field func(s *config.SharedCachesConfig) *bool
}

// All returns every shared cache definition, in canonical order. That order
// is the docker create -v emission order and the doctor expectation order.
func All() []Cache {
	return []Cache{
		{
			// npm's cacache is content-addressed and safe for concurrent
			// writers, so N runner containers installing simultaneously
			// share instead of racing.
			Key:     "npm",
			Volume:  "gh-sr-npm-cache",
			Mount:   "/home/runner/.npm",
			Default: true,
			field:   func(s *config.SharedCachesConfig) *bool { return s.NPM },
		},
		{
			// The XDG cache dir where job-time download caches land (pinned
			// Chrome-for-Testing builds, pip, …).
			Key:     "home_cache",
			Volume:  "gh-sr-runner-cache",
			Mount:   "/home/runner/.cache",
			Default: true,
			field:   func(s *config.SharedCachesConfig) *bool { return s.HomeCache },
		},
		{
			// Gradle guards cross-process cache access with file locks
			// inside GRADLE_USER_HOME, so N containers building
			// simultaneously share the dependency and wrapper-distribution
			// caches instead of each re-downloading them over the WAN.
			Key:     "gradle",
			Volume:  "gh-sr-gradle-cache",
			Mount:   "/home/runner/.gradle",
			Default: true,
			field:   func(s *config.SharedCachesConfig) *bool { return s.Gradle },
		},
		{
			// pub writes each package version atomically, making the cache
			// safe for concurrent writers.
			Key:     "pub",
			Volume:  "gh-sr-pub-cache",
			Mount:   "/home/runner/.pub-cache",
			Default: true,
			field:   func(s *config.SharedCachesConfig) *bool { return s.Pub },
		},
		{
			// The mise download cache already lives under ~/.cache and is
			// therefore shared by the home_cache row; this row shares the
			// tool-install dir itself. Every container runs as the same uid
			// (runner, 1001), so the multi-user permission issues of a
			// shared mise data dir do not apply here; a concurrent install
			// of the same tool+version is last-writer-wins and a fresh
			// install heals any partial race.
			Key:     "mise",
			Volume:  "gh-sr-mise-data",
			Mount:   "/home/runner/.local/share/mise",
			Default: true,
			field:   func(s *config.SharedCachesConfig) *bool { return s.Mise },
		},
		{
			Key:     "android_sdk",
			Volume:  "gh-sr-android-sdk",
			Mount:   "/opt/android-sdk",
			Env:     "GH_SR_ANDROID_SDK_ROOT",
			Default: false,
			field:   func(s *config.SharedCachesConfig) *bool { return s.AndroidSDK },
		},
	}
}

// Enabled resolves the shared caches the config turns on, in table order.
// The section defaults to on (a `shared_caches:` block exists only to tune
// or disable); each sub-flag defaults to its row Default, except that an
// explicit value always wins. The master switch
// (shared_caches.enabled: false) kills every row.
func Enabled(s config.SharedCachesConfig) []Cache {
	if s.Enabled != nil && !*s.Enabled {
		return nil
	}
	out := make([]Cache, 0, len(All()))
	for _, c := range All() {
		enabled := c.Default
		if b := c.field(&s); b != nil {
			enabled = *b
		}
		if enabled {
			out = append(out, c)
		}
	}
	return out
}
