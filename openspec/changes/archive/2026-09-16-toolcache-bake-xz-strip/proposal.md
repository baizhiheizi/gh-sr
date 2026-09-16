# Proposal: toolcache-bake-xz-strip

## Why

The `container_runner_image.toolcache` bake (branch `feat/container-runner-toolcache-bake`, 45b7063) pre-extracts tool tarballs into the runner image at attended build time so cold containers skip flaky release-asset downloads — but it only handles gzip archives with no path remapping. Two real toolchains don't fit: Flutter stable ships **tar.xz** only, and Adoptium JDK tarballs carry a top-level version directory that breaks consumers probing `$TOOLCACHE/<dir>/bin/java` (they expect strip-components semantics). Meanwhile `main` has moved ~8 commits past the branch base, so the feature cannot land as-is.

## What Changes

- Rebase `feat/container-runner-toolcache-bake` onto current `main` and resolve conflicts.
- Extend the bake step to dispatch on archive extension: `tar.gz` (existing) and `tar.xz` (new; `xz-utils` joins `apt-packages-core.txt`).
- Add an optional `strip` field per entry (`--strip-components=N`, default 0), backward-compatible with existing 3-field lines; document that `strip: 1` turns a top-level-wrapped archive into a bare tool dir.
- Update config validation, Dockerfile bake loop, docs (configuration reference + agentic guide), CHANGELOG, and tests for the new field and format.
- No breaking changes: existing 3-column entries (Ruby/Node in production runners.yml) parse identically; the layout revision fingerprint folds in the bake list as before, so a changed list still triggers rebuild prompts.

Prerequisite-for / companion-to: baizhiheizi/enjoy_player change `linux-runner-toolchain-bake`, which consumes this feature to bake Flutter 3.44.0 (tar.xz) and a pinned Temurin JDK 17 (strip) into the org runner image.

## Capabilities

### New Capabilities
- `container-runner-image`: Declarative customization of the locally built `gh-sr/agentic-runner` image — extra apt packages and toolcache bake entries (archive formats, extraction layout, markers, validation, fingerprint/rebuild semantics).

### Modified Capabilities
<!-- none — container-host-docker covers host Docker provisioning, not image content -->

## Impact

- **Code**: `internal/config/config.go` (+tests), `internal/runner/container.go`, `internal/runner/runner.go`, `internal/runner/agentic-runner-image/Dockerfile`, `internal/ops/ops.go` (fingerprint), bench/layout tests.
- **Behavior**: image builds now require `xz-utils` in the build deps; bake failures fail the build (unchanged). Image tag/layout revision changes → existing deployments report BUILD `stale` until `gh sr rebuild`.
- **Ops**: after merge, the `gh` extension should be reinstalled from source (or a release tagged) so host binaries match; downstream runners.yml files may then adopt `tar.xz`/`strip` entries.
