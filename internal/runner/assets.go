package runner

import _ "embed"

//go:embed svc.sh
var SVCShContent string

//go:embed actions.runner.service.template
var ServiceTemplateContent string

//go:embed agentic-runner-image/Dockerfile
var agenticRunnerDockerfile string

//go:embed agentic-runner-image/apt-packages-core.txt
var agenticRunnerAptPackagesCore string

//go:embed agentic-runner-image/entrypoint.sh
var agenticRunnerEntrypoint string

//go:embed agentic-runner-image/docker-wrapper.sh
var agenticRunnerDockerWrapper string

//go:embed agentic-runner-image/daemon.json
var agenticRunnerDaemonJSON string

//go:embed agentic-runner-image/dnsmasq-gh-sr.conf
var agenticRunnerDnsmasqConf string

//go:embed agentic-runner-image/hooks/job-started.sh
var agenticRunnerJobStartedHook string

//go:embed agentic-runner-image/hooks/job-completed.sh
var agenticRunnerJobCompletedHook string
