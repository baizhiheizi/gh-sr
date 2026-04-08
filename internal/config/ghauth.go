package config

import (
	"fmt"
	"os"

	ghAuth "github.com/cli/go-gh/v2/pkg/auth"
)

// TokenSourcePAT means the token came from the config PAT field or GITHUB_PAT env var.
const TokenSourcePAT = "pat"

// TokenSourceGH means the token came from the gh CLI's stored credentials.
const TokenSourceGH = "gh"

// ResolvedToken holds a GitHub token and its origin so callers can report how auth was obtained.
type ResolvedToken struct {
	Token  string
	Source string // TokenSourcePAT or TokenSourceGH
}

// ResolveToken finds a usable GitHub token using the following priority:
//  1. Explicit PAT in config (github.pat or env: reference)
//  2. GITHUB_PAT / GITHUB_TOKEN environment variable (direct, not via env: prefix)
//  3. gh CLI auth (via go-gh library — reads gh's config/keyring)
//
// Returns an error only when no source provides a token.
func ResolveToken(cfg *Config) (ResolvedToken, error) {
	if cfg.GitHub.PAT != "" {
		return ResolvedToken{Token: cfg.GitHub.PAT, Source: TokenSourcePAT}, nil
	}

	for _, env := range []string{"GITHUB_PAT", "GITHUB_TOKEN"} {
		if t := os.Getenv(env); t != "" {
			return ResolvedToken{Token: t, Source: TokenSourcePAT}, nil
		}
	}

	token, _ := ghAuth.TokenForHost("github.com")
	if token != "" {
		return ResolvedToken{Token: token, Source: TokenSourceGH}, nil
	}

	return ResolvedToken{}, fmt.Errorf("no GitHub token found; either set github.pat in runners.yml, export GITHUB_PAT, or run `gh auth login`")
}
