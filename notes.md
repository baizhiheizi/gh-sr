---
name: run-2026-07-03-28638387867
description: Run history entry for Repo Assist run 28638387867 (selected tasks 9, 4, 8) — archived summary
metadata:
  type: project
---

# Run 28638387867 (archived 2026-07-14)

Older run. Key outcomes preserved for reference:

- **Task 8** — `repo-assist/perf-loadstr-appendfloat-2026-07-03`, commit `977e1c5`: `internal/host/metrics.go` +21/−11. `LoadStr` switched from `strings.Builder + strconv.FormatFloat` to `strconv.AppendFloat + stack [40]byte`. **NOT submitted as PR** — maintainer explicitly reverted the working-tree version mid-run. Branch and commit preserved locally.
- **Task 9** — `repo-assist/test-editor-open-windows-default-2026-07-03`, commit `6db86e8`: `internal/editor/editor_test.go` +58/−3. Coverage `internal/editor`: 53.8% → 92.3%. **`create_pull_request` reported success**.
- **Task 4** — `repo-assist/eng-makefile-fmt-target-2026-07-03`, commit `9bfa21c`: `Makefile` +19/−2. Adds `make fmt`, `make tidy`, `make ci`. **`create_pull_request` reported success**.
- **Task 11** — `update_issue` to #309.

## Verified
- `go build ./...` OK
- `go vet ./...` clean
- `gofmt -l .` clean
- `make ci` 14/14 OK
- `go test ./... -race -count=1` 14/14 OK
- `BenchmarkLoadStr` 5 reps: 284→257 ns/op (-9.5%), 24→16 B/op (-33%)

## Discovered contracts (for future reference)
- **`os/exec` missing-binary error chain** wraps `exec.ErrNotFound`, NOT `fs.ErrNotExist`.
- **`t.Setenv` is per-test isolation** — safe to use across subtests without teardown leaks.
- **`make ci` ergonomics**: vet + fmt + test, in that order, exit on first failure.

## LoadStr commit status
- The `977e1c5` commit survives on `repo-assist/perf-loadstr-appendfloat-2026-07-03`. Working tree at `main` reverts to the `strings.Builder` version. (Later superseded by PR #355 / commit b43ab41 which landed the same win-class permanently.)