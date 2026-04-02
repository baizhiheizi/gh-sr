package config

import _ "embed"

//go:embed runners.yml.template
var RunnersYMLTemplate []byte

// EnvFileTemplate is the default contents for ~/.ghr/env when created by ghr init.
const EnvFileTemplate = `# Secrets for ghr (loaded before runners.yml). Use with github.pat: env:GITHUB_PAT in runners.yml.
# GITHUB_PAT=
`
