package runner

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

// TestNewManagerFromConfig pins the config-to-Manager mapping: the single
// constructor is the interface through which every derived field is set, so
// no caller has to remember to wire them.
func TestNewManagerFromConfig(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	t.Run("nil config leaves derived fields at zero", func(t *testing.T) {
		t.Parallel()
		mgr := NewManagerFromConfig("pat", nil)
		if mgr == nil {
			t.Fatal("nil manager")
		}
		if mgr.ContainerImageExtraApt != nil || mgr.ContainerImageBaseImage != "" ||
			mgr.Cache != nil || mgr.RegistryMirror != nil || mgr.NPMRegistry != "" ||
			mgr.SharedNPMCache || mgr.SharedAndroidSDK {
			t.Errorf("nil config should leave derived fields zeroed, got %+v", mgr)
		}
	})

	t.Run("config with no extras yields empty apt list", func(t *testing.T) {
		t.Parallel()
		mgr := NewManagerFromConfig("pat", &config.Config{})
		if len(mgr.ContainerImageExtraApt) != 0 {
			t.Errorf("expected empty, got %v", mgr.ContainerImageExtraApt)
		}
	})

	t.Run("image extras and shared-cache defaults populate", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			ContainerRunnerImage: config.ContainerRunnerImageConfig{
				ExtraAptPackages:    []string{"sqlite3", "ffmpeg"},
				MTU:                 1400,
				DockerdStartTimeout: 90,
				BootstrapMaxRetries: 5,
				StartStaggerSeconds: 3,
			},
		}
		mgr := NewManagerFromConfig("pat", cfg)
		if len(mgr.ContainerImageExtraApt) != 2 ||
			mgr.ContainerImageExtraApt[0] != "sqlite3" || mgr.ContainerImageExtraApt[1] != "ffmpeg" {
			t.Errorf("ContainerImageExtraApt = %v, want [sqlite3 ffmpeg]", mgr.ContainerImageExtraApt)
		}
		if mgr.ContainerMTU != 1400 || mgr.ContainerDockerdStartTimeout != 90 ||
			mgr.ContainerBootstrapMaxRetries != 5 || mgr.ContainerStartStaggerSeconds != 3 {
			t.Errorf("image knobs not propagated: %+v", mgr)
		}
		// Zero-value Config: shared caches default on, android_sdk stays
		// opt-in off.
		if !mgr.SharedNPMCache || !mgr.SharedRunnerCache || !mgr.SharedGradleCache ||
			!mgr.SharedPubCache || !mgr.SharedMiseData {
			t.Errorf("shared caches should default on, got %+v", mgr)
		}
		if mgr.SharedAndroidSDK {
			t.Error("android sdk must default off")
		}
	})

	t.Run("shared caches opt-outs and android opt-in propagate", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			SharedCaches: config.SharedCachesConfig{
				Enabled:    boolPtr(true),
				NPM:        boolPtr(false),
				HomeCache:  boolPtr(false),
				Gradle:     boolPtr(false),
				Pub:        boolPtr(false),
				Mise:       boolPtr(false),
				AndroidSDK: boolPtr(true),
			},
		}
		mgr := NewManagerFromConfig("pat", cfg)
		if mgr.SharedNPMCache || mgr.SharedRunnerCache || mgr.SharedGradleCache ||
			mgr.SharedPubCache || mgr.SharedMiseData {
			t.Errorf("opted-out caches must stay off, got %+v", mgr)
		}
		if !mgr.SharedAndroidSDK {
			t.Error("android_sdk: true must reach the Manager")
		}
	})
}
