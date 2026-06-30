---
name: run-2026-06-30-28473873742
description: Run history entry for Repo Assist run 28473873742 (selected tasks 6, 9, 1)
metadata:
  type: project
---

# Run 28473873742 — 2026-06-30 20:45 UTC

## Selected tasks
- Task 1 (Labelling) — fallback to Task 2 (all 12 open issues already labelled).
- Task 6 (Maintain Repo Assist PRs) — verified PR #298 (`mergeable_state: unstable` is a CI-status artifact, not a real failure). Local build/vet/tests/race (14/14) all green; `git merge --no-commit origin/main` says "Already up to date" → no actual conflict. PR is healthy and awaiting maintainer review.
- Task 9 (Testing Improvements) — branch `repo-assist/test-listinstalled-os-paths-2026-06-30`, commit `4d8e291`: 5 new test functions / 13 sub-tests in `internal/autostart/cleanup_test.go` (`TestListInstalledDarwin`, `TestListInstalledWindows`, `TestListInstalledDispatch`, `TestParseInstanceLines`, `equalInstances` helper). **Phantom PR** — safeoutputs `create_pull_request` returned success but no PR opened remotely. Patch (8.8 KB) + bundle (4.5 KB) preserved at `/tmp/gh-aw/aw-repo-assist-test-listinstalled-os-paths-2026-06-30.{patch,bundle}`. Local branch preserved.
- Task 11 — `add_comment` on #100 with full Suggested Actions + Run History entry succeeded (`aw_nl3wnkwa`); `update_issue` 5th silent failure on #100 — recovered via `add_comment` per established pattern.

## Verified
- `go build ./...` OK; `go vet ./...` clean; `gofmt -l .` clean.
- `go test ./... -count=1 -race` 14/14 OK.
- New tests in `internal/autostart/cleanup_test.go`: 5 functions / 13 sub-tests, race-clean.
- Coverage delta: `ListInstalled` 40%→100%, `listInstalledDarwin` 0%→100%, `listInstalledWindows` 0%→100%, `parseInstanceLines` 0%→100%; package 43.7%→51.9% (+8.2 pp).
- Diff: 1 file, +190 / −0.

## phantom-PR pattern (8th+ occurrence)
- Run 28436621798 (RenderPlain refactor) — landed as PR #297.
- Run 28350202697 (gofmt drift fix) — landed as PR #292.
- Run 28368951549 (renderRowWith refactor) — branch not pushed.
- Run 28302041922 (Perf Improver consolidate-autostart-detect) — reported incomplete.
- #273 (run 28231593128) — landed later.
- Run 28355199319 (orchestrator-connect-errors) — visible as PR #293.
- Run 28455022510 (SplitSeq cleanup) — landed as PR #298 despite phantom report.
- **Run 28473873742 (this, test-listinstalled-os-paths)** — branch not pushed. Recovery: `git am /tmp/gh-aw/aw-repo-assist-test-listinstalled-os-paths-2026-06-30.patch && git push origin repo-assist/test-listinstalled-os-paths-2026-06-30 && gh pr create --base main --head repo-assist/test-listinstalled-os-paths-2026-06-30 --draft`.

## Discovered contracts (for future reference)
- **`internal/autostart.ListInstalled(h)`** dispatches Linux/Darwin/Windows/unsupported-OS via switch — all 4 arms now have direct test coverage.
- **`internal/autostart.parseInstanceLines(out)`** preserves input order, skips blank lines and lines without `ghsr-runner-` prefix, trims whitespace per line. Dedup is the caller's responsibility via `dedupeInstances`. Important for the upcoming SplitSeq refactor in PR #298.
- **`internal/host.Host.RunShell`** on Windows hosts with non-local `Addr` encodes the script via `powershell.exe -EncodedCommand`. For tests with local Addr, wrapCommand is a no-op and the mock sees the raw script.
- **`listInstalledDarwin`** uses inline parsing with `com.github.ghsr.runner.` prefix; `listInstalledWindows` uses `instanceFromServiceBasename` helper.