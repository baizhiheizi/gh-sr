package config

import (
	"testing"
)

func TestResolveToken_ExplicitPAT(t *testing.T) {
	t.Parallel()
	cfg := &Config{GitHub: GitHubConfig{PAT: "my-pat-token"}}
	tok, err := ResolveToken(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != "my-pat-token" {
		t.Errorf("token: got %q", tok.Token)
	}
	if tok.Source != TokenSourcePAT {
		t.Errorf("source: got %q want %q", tok.Source, TokenSourcePAT)
	}
}

func TestResolveToken_EnvGITHUB_PAT(t *testing.T) {
	t.Setenv("GITHUB_PAT", "env-pat")
	cfg := &Config{}
	tok, err := ResolveToken(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != "env-pat" {
		t.Errorf("token: got %q", tok.Token)
	}
	if tok.Source != TokenSourcePAT {
		t.Errorf("source: got %q", tok.Source)
	}
}

func TestResolveToken_EnvGITHUB_TOKEN(t *testing.T) {
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("GITHUB_TOKEN", "env-token")
	cfg := &Config{}
	tok, err := ResolveToken(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != "env-token" {
		t.Errorf("token: got %q", tok.Token)
	}
}

func TestResolveToken_PATTakesPriority(t *testing.T) {
	t.Setenv("GITHUB_PAT", "env-pat")
	cfg := &Config{GitHub: GitHubConfig{PAT: "explicit-pat"}}
	tok, err := ResolveToken(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token != "explicit-pat" {
		t.Errorf("explicit PAT should win over env: got %q", tok.Token)
	}
}

func TestResolveToken_FallsBackToGhOrErrors(t *testing.T) {
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	cfg := &Config{}
	tok, err := ResolveToken(cfg)
	if err != nil {
		// No gh auth configured — expected error path.
		return
	}
	// gh auth is configured in this environment — token should come from gh.
	if tok.Source != TokenSourceGH {
		t.Errorf("expected gh source, got %q", tok.Source)
	}
	if tok.Token == "" {
		t.Error("token from gh should not be empty")
	}
}
