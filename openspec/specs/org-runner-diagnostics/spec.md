# org-runner-diagnostics Specification

## Purpose
TBD - created by archiving change org-runners-support. Update Purpose after archive.
## Requirements
### Requirement: Consistent scope display in status output

All user-facing status and progress output MUST display runner targets with a consistent scope prefix.

#### Scenario: Org scope displays with prefix

- **WHEN** status or progress output shows an org-scoped runner
- **THEN** the target MUST display as `org:<org-name>` (e.g. `org:mycompany`)

#### Scenario: Repo scope displays as owner/repo

- **WHEN** status or progress output shows a repo-scoped runner
- **THEN** the target MUST display as `owner/repo` without a prefix

#### Scenario: Group appended when set

- **WHEN** an org-scoped runner has `group: ci-pool`
- **THEN** TUI and detailed status output MUST include `group=ci-pool` alongside the org target

### Requirement: Doctor checks org API access

`gh sr doctor` MUST verify GitHub API access for every unique `org` in the runner config, in parallel with existing repo checks.

#### Scenario: Org list succeeds

- **WHEN** doctor runs with org-scoped runners and the token has org permissions
- **THEN** doctor MUST report `org <name>: list runners OK (<n> registered)` for each org

#### Scenario: Org list fails with permission hint

- **WHEN** doctor's org runner list returns HTTP 403 or a permission-related error
- **THEN** the failure message MUST include a hint that org owner access or `admin:org` scope is required via `gh auth login`

### Requirement: Doctor skips org check when no org runners

Doctor MUST only perform org API checks when at least one configured runner has `org` set.

#### Scenario: Repo-only config

- **WHEN** all runners are repo-scoped
- **THEN** doctor MUST NOT perform org API list calls

