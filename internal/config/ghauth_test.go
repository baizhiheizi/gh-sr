package config

import (
	"testing"
)

func TestResolveToken_FromGhOrErrors(t *testing.T) {
	t.Setenv("GITHUB_PAT", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	cfg := &Config{}
	tok, err := ResolveToken(cfg)
	if err != nil {
		// No gh auth configured — expected error path.
		if tok != "" {
			t.Errorf("token should be empty on error, got %q", tok)
		}
		return
	}
	if tok == "" {
		t.Fatal("token from gh should not be empty")
	}
}
