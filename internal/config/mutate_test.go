package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAddHost(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  existing:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: existing
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(cfgPath, "newhost", "user@10.0.0.1", "", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "newhost") {
		t.Errorf("config should contain newhost: %s", content)
	}
	if !strings.Contains(content, "user@10.0.0.1") {
		t.Errorf("config should contain addr: %s", content)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after AddHost: %v", err)
	}
	if _, ok := cfg.Hosts["newhost"]; !ok {
		t.Error("newhost not found in loaded config")
	}
}

func TestAddHost_duplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	err := AddHost(cfgPath, "h1", "c@d", "", "")
	if err == nil {
		t.Fatal("expected error for duplicate host")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveRunner(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
  - name: r2
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RemoveRunner(cfgPath, "r2"); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after RemoveRunner: %v", err)
	}
	if len(cfg.Runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(cfg.Runners))
	}
	if cfg.Runners[0].Name != "r1" {
		t.Errorf("unexpected remaining runner: %+v", cfg.Runners[0])
	}
}

func TestRemoveRunner_notFound(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	err := RemoveRunner(cfgPath, "missing")
	if err == nil {
		t.Fatal("expected error for missing runner")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAddRunnerFull(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	opts := AddRunnerOpts{
		Name:      "r2",
		Org:       "my-org",
		Group:     "my-group",
		Host:      "h1",
		Count:     3,
		Labels:    []string{"self-hosted", "x64"},
		Ephemeral: true,
		Profile:   "agentic",
	}
	if err := AddRunnerFull(cfgPath, opts); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after AddRunnerFull: %v", err)
	}
	if len(cfg.Runners) != 2 {
		t.Fatalf("expected 2 runners, got %d", len(cfg.Runners))
	}
	if cfg.Runners[0].Name != "r1" {
		t.Errorf("existing runner disturbed: %+v", cfg.Runners[0])
	}
	got := cfg.Runners[1]
	if got.Name != "r2" || got.Org != "my-org" || got.Group != "my-group" ||
		got.Host != "h1" || got.Count != 3 || got.Ephemeral != true || got.Profile != "agentic" {
		t.Errorf("runner miswired: %+v", got)
	}
	// applyAgenticDefaults auto-appends "agentic" to agentic runners during Load.
	if len(got.Labels) != 3 || got.Labels[0] != "self-hosted" || got.Labels[1] != "x64" || got.Labels[2] != "agentic" {
		t.Errorf("labels = %v, want [self-hosted x64 agentic]", got.Labels)
	}
}

func TestAddRunnerFull_minimalOmitsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AddRunnerFull(cfgPath, AddRunnerOpts{Name: "r2", Repo: "o/r", Host: "h1"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after AddRunnerFull: %v", err)
	}
	if len(cfg.Runners) != 2 {
		t.Fatalf("expected 2 runners, got %d", len(cfg.Runners))
	}
	got := cfg.Runners[1]
	if got.Count != 1 {
		t.Errorf("count = %d, want 1 (Load normalizes omitted count)", got.Count)
	}
	if got.Org != "" || got.Group != "" || got.Ephemeral || got.Profile != "" || len(got.Labels) != 0 {
		t.Errorf("unset fields should stay zero: %+v", got)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	// Inspect the newly written runner node's key set, not the whole file: a
	// substring scan over the entire document also matches keys that belong
	// to other sections (a fixture's existing runner carrying org:/labels:/,
	// or a future writer emitting those keys elsewhere), which would fail
	// this test for reasons unrelated to the omission contract under test.
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parsing written config: %v", err)
	}
	runnersNode := findMapValue(root.Content[0], "runners")
	if runnersNode == nil || runnersNode.Kind != yaml.SequenceNode || len(runnersNode.Content) != 2 {
		t.Fatalf("expected 2 runners in the written config, got %+v", runnersNode)
	}
	added := runnersNode.Content[1]
	if added.Kind != yaml.MappingNode {
		t.Fatalf("expected the added runner to be a mapping, got kind %v", added.Kind)
	}
	written := make(map[string]bool, len(added.Content)/2)
	for i := 0; i+1 < len(added.Content); i += 2 {
		written[added.Content[i].Value] = true
	}
	for _, key := range []string{"count", "labels", "ephemeral", "profile", "org", "group"} {
		if written[key] {
			t.Errorf("default-valued key %q should not be written for the new runner; keys: %v", key, written)
		}
	}
	// The keys that carry a value must still be written.
	for _, key := range []string{"name", "repo", "host"} {
		if !written[key] {
			t.Errorf("key %q should be written for the new runner; keys: %v", key, written)
		}
	}
}

func TestAddRunnerFull_createsRunnersSection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AddRunnerFull(cfgPath, AddRunnerOpts{Name: "r1", Repo: "o/r", Host: "h1"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after AddRunnerFull: %v", err)
	}
	if len(cfg.Runners) != 1 {
		t.Fatalf("expected 1 runner, got %d", len(cfg.Runners))
	}
	if cfg.Runners[0].Name != "r1" || cfg.Runners[0].Host != "h1" {
		t.Errorf("runner miswired: %+v", cfg.Runners[0])
	}
}

func TestAddRunnerFull_errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		err := AddRunnerFull(filepath.Join(t.TempDir(), "runners.yml"), AddRunnerOpts{Name: "r1", Repo: "o/r", Host: "h1"})
		if err == nil || !strings.Contains(err.Error(), "reading config") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("runners not a sequence", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "runners.yml")
		initial := `hosts:
  h1:
    addr: a@b
runners:
  r1:
    host: h1
`
		if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
		err := AddRunnerFull(cfgPath, AddRunnerOpts{Name: "r2", Repo: "o/r", Host: "h1"})
		if err == nil || !strings.Contains(err.Error(), "runners is not a sequence") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid after modification", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "runners.yml")
		initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
		if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
		// The runner references a host that is missing from hosts:. The
		// mutation is rejected, and because writeYAMLBack validates before
		// it replaces the file, the config on disk is left exactly as it
		// was — still loadable, still holding the original runner.
		err := AddRunnerFull(cfgPath, AddRunnerOpts{Name: "r2", Repo: "o/r", Host: "nope"})
		if err == nil || !strings.Contains(err.Error(), "config invalid after modification") {
			t.Errorf("unexpected error: %v", err)
		}
		assertConfigUnchanged(t, cfgPath, initial)
	})

	t.Run("duplicate runner name", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "runners.yml")
		initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
		if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
		// A second runner with the same name on the same host collides on
		// the derived instance name ("r1-1"). AddRunnerFull has no
		// duplicate-name guard of its own — unlike AddHost — so the
		// collision is caught by the same post-mutation validation.
		err := AddRunnerFull(cfgPath, AddRunnerOpts{Name: "r1", Repo: "o/r", Host: "h1"})
		if err == nil || !strings.Contains(err.Error(), "config invalid after modification") {
			t.Errorf("unexpected error: %v", err)
		}
		assertConfigUnchanged(t, cfgPath, initial)
	})

	t.Run("group without org", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "runners.yml")
		initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
		if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
		// Runner groups are org-level, so a group without an org is
		// rejected by config validation — the third way a plain
		// `gh sr add runner` can fail.
		err := AddRunnerFull(cfgPath, AddRunnerOpts{Name: "r2", Repo: "o/r", Group: "g", Host: "h1"})
		if err == nil || !strings.Contains(err.Error(), "config invalid after modification") {
			t.Errorf("unexpected error: %v", err)
		}
		assertConfigUnchanged(t, cfgPath, initial)
	})
}

// assertConfigUnchanged asserts that a rejected mutation left the config
// file byte-for-byte as it was and still loadable. It is the post-state
// half of the "invalid after modification" cases: the error alone does not
// prove the user's config survived.
func assertConfigUnchanged(t *testing.T, cfgPath, want string) {
	t.Helper()

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config after rejected mutation: %v", err)
	}
	if string(data) != want {
		t.Errorf("rejected mutation changed the config on disk:\ngot:\n%s\nwant:\n%s", data, want)
	}
	if _, err := Load(cfgPath); err != nil {
		t.Errorf("config should still be loadable after a rejected mutation: %v", err)
	}
}

func TestAddHost_createsSectionAndOSArch(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AddHost(cfgPath, "h1", "u@h:22", "linux", "arm64"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("config should be loadable after AddHost: %v", err)
	}
	h, ok := cfg.Hosts["h1"]
	if !ok {
		t.Fatalf("h1 missing: %+v", cfg.Hosts)
	}
	if h.Addr != "u@h:22" || h.OS != "linux" || h.Arch != "arm64" {
		t.Errorf("host miswired: %+v", h)
	}
	if len(cfg.Runners) != 1 || cfg.Runners[0].Name != "r1" {
		t.Errorf("existing runner disturbed: %+v", cfg.Runners)
	}
}

func TestAddHost_errors(t *testing.T) {
	t.Run("hosts not a mapping", func(t *testing.T) {
		cfgPath := filepath.Join(t.TempDir(), "runners.yml")
		initial := `hosts: not-a-mapping
`
		if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
		err := AddHost(cfgPath, "h1", "u@h", "", "")
		if err == nil || !strings.Contains(err.Error(), "hosts is not a mapping") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoadYAMLRoot(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")
	initial := `hosts:
  h1:
    addr: a@b
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(cfgPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	top, err := loadYAMLRoot(cfgPath)
	if err != nil {
		t.Fatalf("loadYAMLRoot: %v", err)
	}
	if top.Kind != yaml.MappingNode {
		t.Fatalf("expected MappingNode, got %v", top.Kind)
	}
	if findMapValue(top, "hosts") == nil {
		t.Error("expected to find hosts mapping")
	}
	if findMapValue(top, "runners") == nil {
		t.Error("expected to find runners sequence")
	}
}

func TestLoadYAMLRoot_errors(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "runners.yml")

	// missing file
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for missing file")
	} else if !strings.Contains(err.Error(), "reading config") {
		t.Errorf("unexpected error: %v", err)
	}

	// malformed YAML
	if err := os.WriteFile(cfgPath, []byte(":\n  - : bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for malformed YAML")
	} else if !strings.Contains(err.Error(), "parsing config") {
		t.Errorf("unexpected error: %v", err)
	}

	// empty document
	if err := os.WriteFile(cfgPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for empty document")
	} else if !strings.Contains(err.Error(), "unexpected YAML structure") {
		t.Errorf("unexpected error: %v", err)
	}

	// root is a sequence, not a mapping
	if err := os.WriteFile(cfgPath, []byte("- a\n- b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadYAMLRoot(cfgPath); err == nil {
		t.Error("expected error for non-mapping root")
	} else if !strings.Contains(err.Error(), "config root is not a mapping") {
		t.Errorf("unexpected error: %v", err)
	}
}
