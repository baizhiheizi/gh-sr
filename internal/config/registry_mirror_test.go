package config

import (
	"os"
	"path/filepath"
	"testing"
)

// loadFromContent writes extra YAML after a minimal valid config and loads it.
func loadFromContent(t *testing.T, extra string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runners.yml")
	content := `
hosts:
  h1:
    addr: user@10.0.0.1
runners:
  - name: r1
    repo: o/r
    host: h1
` + extra
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(path)
}

// TestLoad_registryMirrorMirrorsDefaults pins the mirrors-list resolution:
// mirrors[0] inherits the legacy upstream/port and defaults its name, later
// entries default their ports to 5577+index, and prefetch flows through.
func TestLoad_registryMirrorMirrorsDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := loadFromContent(t, `
registry_mirror:
  port: 6100
  mirrors:
    - upstream: https://registry-1.docker.io
    - name: ghcr
      upstream: https://ghcr.io
  prefetch:
    - ghcr.io/github/gh-aw-mcpg:latest
`)
	if err != nil {
		t.Fatal(err)
	}
	entries := cfg.RegistryMirrorEntries()
	if len(entries) != 2 {
		t.Fatalf("got %d mirror entries, want 2: %+v", len(entries), entries)
	}
	if entries[0].Name != DefaultRegistryMirrorName || entries[0].Port != 6100 || entries[0].Upstream != "https://registry-1.docker.io" {
		t.Errorf("mirrors[0] must inherit legacy port and default its name/upstream, got %+v", entries[0])
	}
	if entries[1].Port != 5578 {
		t.Errorf("mirrors[1].port must default to 5578, got %d", entries[1].Port)
	}
	if got := cfg.RegistryMirrorPrefetch(); len(got) != 1 || got[0] != "ghcr.io/github/gh-aw-mcpg:latest" {
		t.Errorf("prefetch: got %v", got)
	}
}

// TestLoad_registryMirrorNoMirrorsLegacy pins that an absent mirrors list
// stays absent (the legacy single mirror is synthesized at deploy time) and
// prefetch alone is still valid.
func TestLoad_registryMirrorNoMirrorsLegacy(t *testing.T) {
	t.Parallel()
	cfg, err := loadFromContent(t, `
registry_mirror:
  prefetch:
    - postgres:16
`)
	if err != nil {
		t.Fatal(err)
	}
	if entries := cfg.RegistryMirrorEntries(); len(entries) != 0 {
		t.Errorf("legacy config must not synthesize entries, got %+v", entries)
	}
	if got := cfg.RegistryMirrorPrefetch(); len(got) != 1 {
		t.Errorf("prefetch: got %v", got)
	}
}

// TestValidate_registryMirrorMirrors pins the validation surface: duplicate
// names/ports, missing name on later entries, bad upstreams, split
// credentials, legacy conflicts, and invalid prefetch refs.
func TestValidate_registryMirrorMirrors(t *testing.T) {
	t.Parallel()
	bad := map[string]string{
		"duplicate name": `
registry_mirror:
  mirrors:
    - name: ghcr
      upstream: https://ghcr.io
    - name: ghcr
      upstream: https://ghcr.io
`,
		"duplicate port": `
registry_mirror:
  mirrors:
    - name: hub
      port: 5580
    - name: ghcr
      upstream: https://ghcr.io
      port: 5580
`,
		"missing name on later entry": `
registry_mirror:
  mirrors:
    - upstream: https://registry-1.docker.io
    - upstream: https://ghcr.io
`,
		"invalid name": `
registry_mirror:
  mirrors:
    - name: GHCR!
      upstream: https://ghcr.io
`,
		"missing upstream on later entry": `
registry_mirror:
  mirrors:
    - name: hub
    - name: ghcr
`,
		"bad upstream URL": `
registry_mirror:
  mirrors:
    - name: ghcr
      upstream: ghcr.io
`,
		"username without password_env": `
registry_mirror:
  mirrors:
    - name: ghcr
      upstream: https://ghcr.io
      username: bot
`,
		"legacy upstream conflict": `
registry_mirror:
  upstream: https://registry-1.docker.io
  mirrors:
    - name: hub
      upstream: https://mirror.example.com
`,
		"legacy port conflict": `
registry_mirror:
  port: 5590
  mirrors:
    - name: hub
      port: 5591
`,
		"invalid prefetch ref": `
registry_mirror:
  prefetch:
    - "ghcr.io/github/gh-aw mcpg:latest"
`,
		"empty prefetch ref": `
registry_mirror:
  prefetch:
    - ""
`,
	}
	for name, extra := range bad {
		if _, err := loadFromContent(t, extra); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

// TestValidate_registryMirrorMirrorsEqualLegacy pins that legacy keys equal
// to mirrors[0]'s own values are accepted (setting both is redundant, not
// conflicting).
func TestValidate_registryMirrorMirrorsEqualLegacy(t *testing.T) {
	t.Parallel()
	if _, err := loadFromContent(t, `
registry_mirror:
  port: 5577
  upstream: https://registry-1.docker.io
  mirrors:
    - name: hub
      port: 5577
      upstream: https://registry-1.docker.io
    - name: ghcr
      upstream: https://ghcr.io
`); err != nil {
		t.Fatalf("equal legacy values must not conflict: %v", err)
	}
}
