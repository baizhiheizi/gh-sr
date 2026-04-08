package config

import _ "embed"

//go:embed runners.yml.template
var RunnersYMLTemplate []byte

// EnvFileTemplate is the default contents for ~/.gh-sr/env when created by gh sr init.
const EnvFileTemplate = `# Optional dotenv-style variables for self-hosted runner manager (loaded before runners.yml).
# GitHub API auth is via gh auth login only; this file is for other tooling if needed.
`
