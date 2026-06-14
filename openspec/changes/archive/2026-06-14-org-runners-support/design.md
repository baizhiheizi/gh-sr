## Context

gh-sr's runner model is already scope-aware. `RunnerConfig` exposes `org`, `group`, `repo` fields; `Scope()` / `ScopeTarget()` / `GitHubRegistrationURL()` resolve registration targets; `GitHubClient` uses `actionsURL(scope, target, ...)` for token/list/delete operations; native and container setup pass `--runnergroup` when `group` is set. CLI `gh sr runner add` accepts `--org` and `--group`.

The gap is user-facing: documentation, templates, and examples only show repo-scoped runners. Users operating GitHub orgs with many repositories naturally configure one runner block per repo, missing that a single `org:` entry can serve the entire org.

## Goals / Non-Goals

**Goals:**

- Make org-level runners discoverable through docs, templates, and CLI help
- Document authentication, runner groups, workflow targeting, and name-uniqueness rules
- Provide a migration path from per-repo to org-scoped pools
- Improve status/doctor output so scope (`org` vs `repo`) and permission failures are obvious

**Non-Goals:**

- Enterprise-level runner support (`/enterprises/{ent}/actions/runners`)
- Runner group CRUD via gh-sr (create/list/configure groups in GitHub)
- Changes to registration protocol, scoped API client, or config schema
- Automatic migration tooling (no `gh sr migrate-to-org` command)
- Workflow file generation or repo-wide `runs-on` updates

## Decisions

### 1. Docs-first, minimal code changes

**Decision:** Prioritize documentation and template updates over new features. Touch code only where UX is unclear (scope display, doctor error hints).

**Rationale:** Core org support is complete and tested (`config_test`, `github_test`). Adding features before users know `org:` exists would be premature.

**Alternative considered:** Build runner group management commands. Rejected — GitHub org settings UI already handles this; gh-sr's role is registration and lifecycle, not org policy administration.

### 2. New guide page for org runners

**Decision:** Add `docs/content/guides/org-runners.md` as the canonical org-runner guide; link from `configuration.md` and `authentication.md`.

**Rationale:** Org runners involve concepts (groups, access policies, workflow targeting, security) that don't fit cleanly in the config reference table alone.

**Alternative considered:** Expand `configuration.md` only. Rejected — would make an already long page harder to navigate.

### 3. Consistent scope display format

**Decision:** All user-facing output (status, TUI, ops progress) MUST display targets as:
- `org:<name>` for org-scoped runners
- `owner/repo` for repo-scoped runners
- Append `group=<name>` when `group` is set (TUI already does this)

**Rationale:** `internal/ops/ops.go` and `internal/runner/runner.go` already use `org:` prefix; `internal/tui/status.go` shows bare org name. Normalize to one format.

### 4. Doctor org permission hints

**Decision:** When `ListRunnersScoped("org", org)` fails with HTTP 403 or permission-related errors, doctor MUST append a hint: org owner or `admin:org` scope required via `gh auth login`.

**Rationale:** Authentication docs currently only show repo API paths; users hitting org runners will see opaque 403s without guidance.

**Alternative considered:** Proactively call runner-groups API to validate `group` exists. Rejected for this change — adds API surface and requires group-list permissions; document that groups must exist in GitHub org settings instead.

### 5. Template examples show both scopes

**Decision:** `config/runners.yml` and `internal/config/runners.yml.template` include commented examples for both repo-scoped and org-scoped runners, with `group` on the org example.

**Rationale:** `gh sr init` copies the template; this is the first thing new users see.

### 6. Name uniqueness documentation

**Decision:** Document that GitHub runner instance names (`name-1`, `name-2`, …) are unique within the registration scope — org-wide for `org:` runners, repo-wide for `repo:` runners. Update the existing "unique per repository" note in `configuration.md` to cover both scopes.

**Rationale:** Misunderstanding this causes registration conflicts when consolidating runners.

## Risks / Trade-offs

- **[Risk] Users expect gh-sr to create runner groups** → Mitigation: docs state groups are created in GitHub org settings; `group` only assigns at registration time
- **[Risk] Org runners increase security blast radius** → Mitigation: migration guide covers runner group access policies and public-repo defaults
- **[Risk] Display format change surprises scripts parsing status output** → Mitigation: `org:` prefix already used in ops output; TUI change is additive consistency, not a new format
- **[Risk] `gh auth login` default scopes lack `admin:org`** → Mitigation: authentication.md documents required scopes; doctor hints on failure

## Migration Plan

For users consolidating per-repo runners:

1. Create runner group in GitHub org settings (optional but recommended)
2. Add org-scoped runner block to `runners.yml` (`org:`, `group:`, `count:`, `labels:`)
3. Run `gh sr setup <name>` then `gh sr up <name>`
4. Update workflows in org repos to target labels or `runs-on: group:`
5. Stop and remove old repo-scoped runners (`gh sr down`, `gh sr cleanup`)
6. Delete repo-scoped entries from `runners.yml`

Rollback: re-add repo-scoped entries and re-register; org runners can coexist with repo runners during transition.

## Open Questions

- Should `gh sr init` print a one-line tip about `--org` when the user is authenticated to an org? (Defer — optional polish, not blocking)
- Should the main `configuration.md` example switch from all-repo to a mixed repo+org example? (Recommend: keep repo example, add separate org example block)
