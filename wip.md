---
name: wip
description: Current test-improver work in progress
metadata:
  type: project
---

## Run #32514567250 (2026-08-21 — current run)

- Branch `test-assist/environment-orchestrators`, commit `93b4217`.
- Draft PR intent accepted; patch + bundle at `/tmp/gh-aw/aw-test-assist-environment-orchestrators.{patch,bundle}`.
- 11 tests in `internal/runner/environment_orchestrators_test.go` covering `ContainerEnvironment.Provision` / `Start` / `Reset` / `Destroy` orchestrator methods (0% → 100% each).
- New helpers: `combinedGitHubStub` (single httptest server for both releases/latest + registration-token endpoints, mirrors production `GitHubClient.apiBase`), `envTestRig`, `envTestRigVersionFail`.
- Coverage: `internal/runner` **81.2% → 82.1% (+0.9 pp)**.
- Verified: focused tests (11/11 PASS with `-race`), build, vet, gofmt, full race suite (all 16 packages pass).
- Prior run (2026-08-14) PR #410 (`native-windows-branches-v2`) merged 2026-08-19 by `an-lee`.
- `containerAwaitHealthy` (52.4%, the timeout-driven polling loop in environment.go) is the next candidate inside `environment.go`; requires fake clock for deadline coverage. Deferred.

## Run #31831183885 (2026-08-14)

- Branch `test-assist/native-windows-branches-v2`, commit `be279d5`.
- PR #410 merged 2026-08-19.
- 11 tests in `internal/runner/native_windows_branch_test.go` covering Windows branches (0% → 100%) of `setupNative` / `startNativeOnce` / `handleStaleRegistration` / `stopNative` / `removeNative` / `removeNativeDirectory`.
- Coverage: `internal/runner` 76.4% → 80.9% (+4.5 pp).

[[backlog]] [[run-history]]