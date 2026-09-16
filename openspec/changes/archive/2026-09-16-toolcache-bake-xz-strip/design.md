# Design: toolcache-bake-xz-strip

## Context

Branch `feat/container-runner-toolcache-bake` (45b7063) already implements `container_runner_image.toolcache` end to end: config struct + validation (HTTPS, ≤64 entries, traversal-free dirs, duplicate-dir rejection), a `toolcache-extra.txt` COPY + RUN loop in the agentic-runner Dockerfile (`curl --retry 8` → `tar -xzf` → `touch <complete>` → chown), fingerprint folding, docs, and tests. It is based on a main that is now ~8 commits behind (`use github app`, go-version bump, gh-aw upgrade, PRs #484/#486). Consumers (enjoy_player `linux-runner-toolchain-bake`) need two format extensions: xz archives (Flutter) and strip-components (Adoptium JDK).

## Goals / Non-Goals

**Goals:**
- Land the existing branch on current main with minimal semantic drift.
- Add `.tar.xz` support and per-entry `strip`, backward-compatible.
- Keep validation, docs, tests, and CHANGELOG coherent with the branch's established style.

**Non-Goals:**
- Caching/reusing downloaded tarballs across builds (build cache handles layer reuse).
- Checksum pinning of bake URLs (deferred; URLs are operator-supplied HTTPS today).
- Any change to runtime tool installs or the `RUNNER_TOOL_CACHE` layout.

## Decisions

1. **Rebase, don't reimplement.** The branch is complete and reviewed in situ (amended once, pushed); rebasing preserves its tests and docs. Conflicts expected only where main touched `config.go` validation plumbing, the Dockerfile preamble, or CHANGELOG top sections.

2. **Extension-based decompressor dispatch in the Dockerfile RUN loop.** Case the temp archive name on its suffix: `.tar.xz` → `tar -xJf`, else `tar -xzf` (existing). `xz-utils` is added to `apt-packages-core.txt` (Ubuntu 24.04 base ships it in the seed set; explicit entry keeps the manifest self-describing). The temp filename stays fixed (`/tmp/toolcache.tar.gz`) — cosmetic only — or is renamed per-entry if trivial.

3. **`strip` as a fourth whitespace-separated field.** The bake file is read with `read -r url dir complete strip` and `--strip-components=${strip:-0}`, so legacy 3-field lines parse with strip 0 — no format version bump, existing production entries untouched. `ContainerToolcacheEntry` gains `Strip int` with `yaml:"strip,omitempty"`; validation bounds it (0–16) and rejects negative values. Docs describe the canonical wrapped-archive case (Adoptium `jdk-<v>+<b>/` → `strip: 1`, `dir: jdk-temurin-17`).

4. **Fingerprint unchanged in mechanism.** The branch already folds the serialized bake list into the image tag/layout revision; the new field serializes too, so adding `strip` naturally changes fingerprints. No `internal/ops` changes beyond what the rebase requires.

5. **Extension install path.** After merge, rebuild/install the extension from source for the consuming host (`go build` → replace `~/.local/share/gh/extensions/gh-sr/gh-sr`) and tag a release so the state is pinned; the downstream enjoy_player change gates on this.

## Risks / Trade-offs

- [Rebase conflicts in config validation plumbing (main's "use github app" touched nearby code)] → resolve in favor of main's plumbing, re-run the branch's config tests; `go test ./internal/config/... ./internal/runner/...` gates.
- [xz decompression adds a build dependency] → `xz-utils` is tiny and already present transitively; explicit core-manifest entry makes it auditable.
- [Strip masking a genuinely flat archive] → harmless (strip 1 on a flat tree drops only the top-level files' path prefix); docs steer strip use to wrapped archives; validation bounds prevent nonsense values.

## Migration Plan

1. `git fetch origin && git rebase origin/main feat/container-runner-toolcache-bake` (recreate local branch from the remote ref), resolve conflicts.
2. Implement xz + strip (Dockerfile, config struct + validation + tests, docs, CHANGELOG).
3. `make check` (vet + race tests); build binary; verify with a scratch `runners.yml` containing a tar.xz + strip entry that the generated build context renders the expected loop.
4. Push branch, merge to main, tag/release, reinstall the extension on the runner host.
5. Rollback: the feature is additive and config-gated; reverting the runners.yml bake list restores pre-bake behavior without code rollback.

## Open Questions

- None blocking; exact Adoptium version pin belongs to the consuming change.
