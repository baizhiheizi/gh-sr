---
name: run-history
description: Test-improver run history
metadata:
  type: project
---

## 2026-06-08 12:52 UTC — Run 27138355567

- Branch: `test-assist/awf-hygiene-validators`
- PR: draft via `create_pull_request` safeoutputs
- Patch: `/tmp/gh-aw/aw-test-assist-awf-hygiene-validators.patch` (18383 B / 488 lines)
- Bundle: `/tmp/gh-aw/aw-test-assist-awf-hygiene-validators.bundle` (4728 B)
- Coverage delta: `internal/agentic` 60.5% → 83.9% (+23.4 pp); both `ValidateAWFHygiene` and `ValidateAWFHygieneInner` 0% → 100%
- New file: `internal/agentic/agentic_awf_hygiene_test.go` (435 lines, 13 subtests, all parallel)
- Status: build ✅, vet ✅, `go test ./... -race -count=1` ✅, `gofmt` ✅
- Note: First-cut test failed because inner iptables probe drops `sudo -n` (DinD inner daemon runs as root) — exact-string mock caught the divergence, fixed by separating `awfHygieneIptablesCmd` and `awfHygieneIptablesInnerCmd` constants

## 2026-06-06 11:46 UTC — Run 27061368592

- Branch: `test-assist/agentic-validation-helpers`
- PR: draft via `create_pull_request` safeoutputs (subsequently closed as duplicate #107/#108)
- Coverage: `internal/agentic` 44.8% → 60.5% (+15.7 pp)
- Five `Validate*` helpers 0% → 100%
- Build/vet/race: all pass
- Backlog items completed: 1 (agentic validation set)
- Backlog remaining: 4 high-value

## Previous test-improver PRs (most recent first)

- **#68** (merged 2026-05-31) — `IsServiceActive`, `absRunnerDir`, `resolveAbsoluteRunnerDir`, `containerRunnerPresent`, `containerImageExists`
- **#58** (merged 2026-04-29) — `DetectOS`/`DetectArch`/`DetectDockerAvailable` + mock infra
- **#54** (merged) — `nativeConfigURL`, `powerShellSingleQuoted`, `windowsDeleteRunnerConfig`
- **#53** (merged) — `containerRunnerImageExtraSorted`
- **#51** (merged 2026-04-24) — `applyContainerImageExtras` + `lockedWriter`
- **#40** (merged 2026-04-19) — `posixSingleQuote` + `editor.Preferred`/`Command`
- **#34** (merged) — `normalizeArch` + `SanitizeInstance` table-driven
- **#31** (merged 2026-04-17) — `HasBlockingFailures`, `FormatRemediation`, `FormatAllRemediations`
- **#11** (merged) — `shellSingleQuote`, `dockerRunnerEntryScript`, `docker_pre_setup`

Monthly Activity issues: #4 (April 2026, closed 2026-05-31), #69 (May 2026, closed 2026-05-31), #109 (June 2026, active).

[[wip]] [[backlog]]
