package config

import _ "embed"

//go:embed runners.yml.template
var RunnersYMLTemplate []byte

// EnvFileTemplate is the default contents for ~/.ghr/env when created by ghr init.
const EnvFileTemplate = `# Secrets for ghr (loaded before runners.yml).
# If you use gh CLI (gh auth login), you can skip this file entirely.
# Otherwise set your PAT here and use github.pat: env:GITHUB_PAT in runners.yml.
# GITHUB_PAT=
`
