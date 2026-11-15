package sharedcache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

// TestLoad_resolvesEnabled pins the end-to-end yaml → Enabled resolution:
// the section (and each sub-flag) defaults on — the shared_caches: block
// exists only to tune or disable, mirroring the cache:/registry_mirror:
// opt-out convention — and the resolved rows come out in table order.
func TestLoad_resolvesEnabled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `
hosts:
  h1:
    addr: user@10.0.0.1
runners:
  - name: r1
    repo: o/r
    host: h1
`
	write := func(name, extra string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content+extra), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
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
	load := func(t *testing.T, name, extra string) []Cache {
		t.Helper()
		cfg, err := config.Load(write(name, extra))
		if err != nil {
			t.Fatal(err)
		}
		return Enabled(cfg.SharedCaches)
	}

	caches := load(t, "default.yml", "")
	wantKeys := []string{"npm", "home_cache", "gradle", "pub", "mise"}
	if got := keys(caches); len(got) != len(wantKeys) {
		t.Fatalf("shared caches should default to the five opt-out rows, got %v", got)
	}
	for i, key := range wantKeys {
		if caches[i].Key != key {
			t.Errorf("row %d = %s, want %s (table order preserved)", i, caches[i].Key, key)
		}
	}

	// Master switch kills every row regardless of sub-flags.
	if got := load(t, "off.yml", "shared_caches:\n  enabled: false\n"); got != nil {
		t.Errorf("shared_caches.enabled: false must disable every shared volume, got %v", keys(got))
	}

	// Sub-flags disable selectively.
	npmOff := load(t, "npm-off.yml", "shared_caches:\n  npm: false\n")
	if has(npmOff, "npm") {
		t.Error("shared_caches.npm: false must disable the npm volume")
	}
	if !has(npmOff, "home_cache") {
		t.Error("shared_caches.npm: false must not disable the home-cache volume")
	}
}

// TestLoad_gradlePubMiseOptOut pins the opt-out behavior of the
// gradle/pub/mise sub-flags: each turns off only its own volume, and the
// opt-in android_sdk only mounts when explicitly enabled.
func TestLoad_gradlePubMiseOptOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	base := `
hosts:
  h1:
    addr: user@10.0.0.1
runners:
  - name: r1
    repo: o/r
    host: h1
`
	write := func(name, extra string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(base+extra), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	has := func(caches []Cache, key string) bool {
		for _, c := range caches {
			if c.Key == key {
				return true
			}
		}
		return false
	}
	load := func(t *testing.T, name, extra string) []Cache {
		t.Helper()
		cfg, err := config.Load(write(name, extra))
		if err != nil {
			t.Fatal(err)
		}
		return Enabled(cfg.SharedCaches)
	}

	caches := load(t, "gradle-off.yml", "shared_caches:\n  gradle: false\n")
	if has(caches, "gradle") {
		t.Error("shared_caches.gradle: false must disable the gradle volume")
	}
	if !has(caches, "pub") || !has(caches, "mise") || !has(caches, "npm") || !has(caches, "home_cache") {
		t.Error("shared_caches.gradle: false must not disable the other volumes")
	}

	caches = load(t, "pub-off.yml", "shared_caches:\n  pub: false\n")
	if has(caches, "pub") {
		t.Error("shared_caches.pub: false must disable the pub volume")
	}
	if !has(caches, "gradle") || !has(caches, "mise") {
		t.Error("shared_caches.pub: false must not disable the other volumes")
	}

	caches = load(t, "mise-off.yml", "shared_caches:\n  mise: false\n")
	if has(caches, "mise") {
		t.Error("shared_caches.mise: false must disable the mise volume")
	}
	if !has(caches, "gradle") || !has(caches, "pub") {
		t.Error("shared_caches.mise: false must not disable the other volumes")
	}

	// android_sdk is opt-in: explicit true enables it alongside the defaults.
	caches = load(t, "android-on.yml", "shared_caches:\n  android_sdk: true\n")
	if !has(caches, "android_sdk") {
		t.Error("shared_caches.android_sdk: true must enable the android sdk volume")
	}
	if !has(caches, "gradle") || !has(caches, "pub") || !has(caches, "mise") {
		t.Error("shared_caches.android_sdk: true must not disable the other volumes")
	}

	// Explicit android_sdk: false keeps the opt-in off.
	caches = load(t, "android-off.yml", "shared_caches:\n  android_sdk: false\n")
	if has(caches, "android_sdk") {
		t.Error("shared_caches.android_sdk: false must keep the android sdk volume off")
	}
}
