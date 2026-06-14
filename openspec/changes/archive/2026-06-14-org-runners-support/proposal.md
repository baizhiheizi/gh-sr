## Why

gh-sr already registers and manages organization-level self-hosted runners (`org:`, `group:`, scoped GitHub API), but every example, template, and auth doc path is repo-centric. Users with many repos under one GitHub org duplicate runner config per repository when a single org-scoped pool would suffice. Making org runners visible and well-documented reduces operational overhead and aligns gh-sr with how GitHub orgs are meant to share runner infrastructure.

## What Changes

- Add org-level runner examples to `docs/content/configuration.md`, `config/runners.yml`, and `internal/config/runners.yml.template`
- Document org-level authentication requirements (`admin:org`, org owner) in `docs/content/authentication.md`
- Add a short migration guide: consolidating per-repo runners into one org-scoped pool
- Improve operational UX: clearer scope display in status/TUI (`org:` vs `owner/repo`), doctor messaging for org API permission failures
- Document org-wide runner name uniqueness (names are unique per org, not per repo)
- Add Hugo docs page or section covering runner groups and workflow targeting (`runs-on: group:` / labels)

No breaking changes to config schema or CLI flags. Core registration/API code already exists; this change is primarily discoverability, documentation, and light UX polish.

## Capabilities

### New Capabilities

- `org-runner-configuration`: Requirements for org-scoped runner config (`org`, `group`), validation rules, CLI (`gh sr runner add --org`), and registration behavior
- `org-runner-documentation`: User-facing docs, examples, migration guidance, and workflow targeting for org runners and runner groups
- `org-runner-diagnostics`: Doctor and status/TUI requirements for org scope visibility and actionable permission errors

### Modified Capabilities

<!-- No existing openspec/specs/ baseline — all requirements are new -->

## Impact

- **Docs**: `docs/content/configuration.md`, `docs/content/authentication.md`, new or extended guide under `docs/content/guides/`
- **Templates**: `config/runners.yml`, `internal/config/runners.yml.template`
- **Code (light)**: `internal/doctor/doctor.go`, `internal/tui/status.go`, `internal/ops/ops.go` (scope display consistency)
- **Tests**: Doctor and config tests if error messages or display strings change
- **Risk**: Low — no changes to registration protocol or GitHub API integration
