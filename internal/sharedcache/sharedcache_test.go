package sharedcache

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

// TestAll_pinsTable is the single pin on the shared-cache definitions: the
// volume names and mount paths are a contract with the host (docker volumes
// already on disk, the disk report's gh-sr- prefix filter, doctor's inspect
// expectations), so any change here is a behavioural change to reconcile,
// not a refactor.
func TestAll_pinsTable(t *testing.T) {
	t.Parallel()
	rows := All()
	if len(rows) != 6 {
		t.Fatalf("want 6 cache rows, got %d", len(rows))
	}
	want := []struct{ key, volume, mount, env string }{
		{"npm", "gh-sr-npm-cache", "/home/runner/.npm", ""},
		{"home_cache", "gh-sr-runner-cache", "/home/runner/.cache", ""},
		{"gradle", "gh-sr-gradle-cache", "/home/runner/.gradle", ""},
		{"pub", "gh-sr-pub-cache", "/home/runner/.pub-cache", ""},
		{"mise", "gh-sr-mise-data", "/home/runner/.local/share/mise", ""},
		{"android_sdk", "gh-sr-android-sdk", "/opt/android-sdk", "GH_SR_ANDROID_SDK_ROOT"},
	}
	for i, w := range want {
		got := rows[i]
		if got.Key != w.key || got.Volume != w.volume || got.Mount != w.mount || got.Env != w.env {
			t.Errorf("row %d = %+v, want key=%s volume=%s mount=%s env=%q", i, got, w.key, w.volume, w.mount, w.env)
		}
	}
	if rows[5].Default {
		t.Error("android_sdk must default off (opt-in)")
	}
	for _, r := range rows[:5] {
		if !r.Default {
			t.Errorf("%s must default on (opt-out)", r.Key)
		}
	}
}

// TestEnabled_defaults pins the config resolution: a zero section means
// every opt-out cache on and android_sdk off; the master switch kills every
// row; an explicit sub-flag always wins over the row default.
func TestEnabled_defaults(t *testing.T) {
	t.Parallel()
	boolPtr := func(b bool) *bool { return &b }

	keys := func(caches []Cache) []string {
		out := make([]string, 0, len(caches))
		for _, c := range caches {
			out = append(out, c.Key)
		}
		return out
	}
	has := func(caches []Cache, key string) bool {
		for _, k := range keys(caches) {
			if k == key {
				return true
			}
		}
		return false
	}

	defaults := Enabled(config.SharedCachesConfig{})
	for _, key := range []string{"npm", "home_cache", "gradle", "pub", "mise"} {
		if !has(defaults, key) {
			t.Errorf("zero section: %s should default on, got %v", key, keys(defaults))
		}
	}
	if has(defaults, "android_sdk") {
		t.Errorf("zero section: android_sdk must default off, got %v", keys(defaults))
	}

	if got := Enabled(config.SharedCachesConfig{Enabled: boolPtr(false)}); got != nil {
		t.Errorf("master switch off must kill every row, got %v", keys(got))
	}

	npmOff := Enabled(config.SharedCachesConfig{NPM: boolPtr(false)})
	if has(npmOff, "npm") {
		t.Error("npm: false must drop the npm row")
	}
	if !has(npmOff, "home_cache") {
		t.Error("npm: false must not drop the home_cache row")
	}

	androidOn := Enabled(config.SharedCachesConfig{AndroidSDK: boolPtr(true)})
	if !has(androidOn, "android_sdk") {
		t.Error("android_sdk: true must add the android row")
	}
	androidOff := Enabled(config.SharedCachesConfig{AndroidSDK: boolPtr(false)})
	if has(androidOff, "android_sdk") {
		t.Error("android_sdk: false must keep the android row off")
	}

	// Explicit false under a disabled section stays disabled — the master
	// switch short-circuits before per-row flags are read.
	if got := Enabled(config.SharedCachesConfig{Enabled: boolPtr(false), AndroidSDK: boolPtr(true)}); got != nil {
		t.Errorf("master switch wins over sub-flags, got %v", keys(got))
	}
}
