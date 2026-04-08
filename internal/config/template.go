package config

import _ "embed"

//go:embed runners.yml.template
var RunnersYMLTemplate []byte

// EnvFileTemplate is the default contents for ~/.gh-wm/env when created by gh wm init.
const EnvFileTemplate = `# Secrets for GitHub Workflow Manager (loaded before runners.yml).
# If you use gh CLI (gh auth login), you can skip this file entirely.
# Otherwise set your PAT here and use github.pat: env:GITHUB_PAT in runners.yml.
# GITHUB_PAT=
`
