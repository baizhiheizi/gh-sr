package config

import (
	"fmt"

	ghAuth "github.com/cli/go-gh/v2/pkg/auth"
)

// ResolveToken returns a GitHub API token from the gh CLI (go-gh reads gh's config/keyring).
// Config must be validated first: legacy github.pat is rejected in Config.Validate.
func ResolveToken(_ *Config) (string, error) {
	token, err := ghAuth.TokenForHost("github.com")
	if token != "" {
		return token, nil
	}
	return "", fmt.Errorf("no GitHub token found: %v; run `gh auth login`", err)
}
