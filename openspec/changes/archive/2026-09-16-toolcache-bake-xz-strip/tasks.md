# Tasks: toolcache-bake-xz-strip

## 1. Rebase the feature branch

- [x] 1.1 Recreate the local branch from `origin/feat/container-runner-toolcache-bake` and rebase onto `origin/main`, resolving conflicts in `internal/config/config.go`, the Dockerfile, and CHANGELOG in favor of main's plumbing. Verify: `git rebase` completes; `go build ./...` succeeds.
- [x] 1.2 Run the branch's test suite against rebased code. Verify: `go test ./internal/config/... ./internal/runner/...` passes; `make check` (vet + race tests) clean.

## 2. xz support

- [x] 2.1 Add `xz-utils` to `internal/runner/agentic-runner-image/apt-packages-core.txt`. Verify: file lists it; manifest parse test (if any) passes.
- [x] 2.2 Extend the Dockerfile bake RUN loop to select `tar -xJf` for `.tar.xz` URLs (and `tar -xzf` otherwise), keeping the `|| exit 1` fail-fast semantics. Verify: render the build context for a sample list and inspect the generated loop; `docker build` smoke on a scratch context with a tiny xz fixture.

## 3. strip support

- [x] 3.1 Add `Strip int` (`yaml:"strip,omitempty"`) to `ContainerToolcacheEntry`; validation bounds 0–16 and rejects negatives with an entry-indexed error. Verify: `go test ./internal/config/...` covering valid defaults, explicit 1, and rejected -1/17.
- [x] 3.2 Thread `strip` through the bake file writer (4th field) and the Dockerfile loop (`--strip-components=${strip:-0}`), preserving 3-field-line compatibility. Verify: unit test on the writer output for both 3-field and 4-field entries; generated loop handles empty 4th field.
- [x] 3.3 Update `internal/runner/container_test.go` / layout bench expectations for the new serialization. Verify: `go test ./internal/runner/...` passes.

## 4. Docs, CHANGELOG, fingerprint

- [x] 4.1 Update `docs/content/configuration.md` (toolcache entry fields incl. `strip`) and the agentic guide's toolcache mentions. Verify: Hugo build or doc lint passes.
- [x] 4.2 CHANGELOG entry under Unreleased covering the bake feature + this extension. Verify: section renders in preview.
- [x] 4.3 Confirm the image fingerprint still folds the serialized bake list (now including strip) and `gh sr status` reports stale after a bake-list edit. Verify: layout test asserting fingerprint sensitivity to the new field.

## 5. Ship

- [x] 5.1 Push the branch, open/merge PR to main, tag a release. Verify: CI green; release artifact published.
- [x] 5.2 Rebuild/install the `gh sr` extension from merged main on the runner host and confirm `gh sr doctor` accepts a config using `tar.xz` + `strip` entries (schema-only check, no rebuild yet — that belongs to the consuming change). Verify: `gh extension list` shows the new build; doctor exit 0.
