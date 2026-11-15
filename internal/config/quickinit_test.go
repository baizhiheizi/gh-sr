package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestQuickInitSeed_pinsSeed pins the quick-init seed contract: the file
// written through AddHost/AddRunnerFull loads, the host and runner round-trip
// exactly, the profile propagates, and re-seeding overwrites rather than
// appending (the caller owns the overwrite confirmation).
func TestQuickInitSeed_pinsSeed(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "runners.yml")

	err := QuickInitSeed(path, "runner-host", "user@10.0.0.1", AddRunnerOpts{
		Name:  "gh-sr-runner-host",
		Repo:  "owner/repo",
		Host:  "runner-host",
		Count: 3,
	})
	if err != nil {
		t.Fatalf("seed must load: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	hcfg, ok := cfg.Hosts["runner-host"]
	if !ok {
		t.Fatalf("host missing: %+v", cfg.Hosts)
	}
	if hcfg.Addr != "user@10.0.0.1" {
		t.Errorf("addr = %q, want user@10.0.0.1", hcfg.Addr)
	}
	if len(cfg.Runners) != 1 {
		t.Fatalf("runners = %d, want 1", len(cfg.Runners))
	}
	rc := cfg.Runners[0]
	if rc.Name != "gh-sr-runner-host" || rc.Repo != "owner/repo" || rc.Host != "runner-host" || rc.Count != 3 {
		t.Errorf("runner miswired: %+v", rc)
	}
	if rc.Profile == "agentic" {
		t.Error("empty profile must not become agentic")
	}

	// Overwrite semantics: seeding again replaces the file wholesale.
	err = QuickInitSeed(path, "local", "local", AddRunnerOpts{
		Name:    "repo-local",
		Repo:    "owner/other",
		Host:    "local",
		Count:   1,
		Profile: "agentic",
	})
	if err != nil {
		t.Fatalf("re-seed must load: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload after re-seed: %v", err)
	}
	if _, ok := cfg.Hosts["runner-host"]; ok {
		t.Error("re-seed must overwrite, not append (old host still present)")
	}
	if len(cfg.Runners) != 1 || cfg.Runners[0].Profile != "agentic" {
		t.Errorf("re-seed runner = %+v, want one agentic runner", cfg.Runners)
	}

	// The seed lives on disk where the caller's editor flow expects it.
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Errorf("seed file missing or empty: %v", err)
	}
}
