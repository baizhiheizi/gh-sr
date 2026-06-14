# org-runner-documentation Specification

## Purpose
TBD - created by archiving change org-runners-support. Update Purpose after archive.
## Requirements
### Requirement: Configuration docs include org runner example

The configuration reference MUST include a complete org-scoped runner example alongside existing repo-scoped examples.

#### Scenario: Org example in configuration reference

- **WHEN** a user reads `docs/content/configuration.md`
- **THEN** they MUST find a `runners[]` example using `org:`, optional `group:`, `host:`, `count:`, and `labels:`

#### Scenario: Config templates include org example

- **WHEN** a user reads `config/runners.yml` or `internal/config/runners.yml.template`
- **THEN** they MUST find a commented org-scoped runner example with `group` noted as org-only

### Requirement: Authentication docs cover org permissions

Authentication documentation MUST describe permissions required for org-level runner management, distinct from repo admin access.

#### Scenario: Org permission requirements documented

- **WHEN** a user reads `docs/content/authentication.md`
- **THEN** it MUST state that org-scoped runners require org owner access or `admin:org` scope (classic PAT) / equivalent fine-grained org permission
- **THEN** it MUST list org-scoped API paths (`/orgs/{org}/actions/runners/...`) alongside existing repo paths

### Requirement: Org runners guide exists

A dedicated guide MUST explain org-level runners, runner groups, workflow targeting, and security considerations.

#### Scenario: Guide page is reachable

- **WHEN** a user browses gh-sr documentation
- **THEN** a guide at `docs/content/guides/org-runners.md` MUST exist and be linked from configuration and authentication pages

#### Scenario: Guide covers workflow targeting

- **WHEN** a user reads the org runners guide
- **THEN** it MUST explain targeting via labels (`runs-on: [self-hosted, Linux]`) and runner groups (`runs-on: group: ci-pool`)
- **THEN** it MUST explain that runner groups control which org repos may use the runners

#### Scenario: Guide covers runner groups setup

- **WHEN** a user reads the org runners guide
- **THEN** it MUST explain that runner groups are created in GitHub org settings before referencing them in `group:` config
- **THEN** it MUST note that public repos cannot use org runners by default unless the group policy allows it

### Requirement: Migration guide for repo to org consolidation

Documentation MUST describe how to consolidate multiple per-repo runner entries into one org-scoped pool.

#### Scenario: Migration steps documented

- **WHEN** a user reads the org runners guide
- **THEN** it MUST provide step-by-step migration: create org runner → setup/up → update workflows → remove repo runners → cleanup config
- **THEN** it MUST note that org and repo runners can coexist during transition

### Requirement: CLI help references org runners

`gh sr runner add --help` MUST mention `--org` and `--group` with brief usage context.

#### Scenario: Help text mentions org scope

- **WHEN** a user runs `gh sr runner add --help`
- **THEN** the long description MUST mention `--org` for organization-level runners and `--group` for runner groups

