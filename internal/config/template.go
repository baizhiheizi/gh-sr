package config

import _ "embed"

//go:embed runners.yml.template
var RunnersYMLTemplate []byte

// EnvFileTemplate is the default contents for ~/.gh-wm/env when created by gh wm init.
const EnvFileTemplate = `# Optional dotenv-style variables for GitHub Workflow Manager (loaded before runners.yml).
# GitHub API auth is via gh auth login only; this file is for other tooling if needed.
`
