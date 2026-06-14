# org-runner-configuration Specification

## Purpose
TBD - created by archiving change org-runners-support. Update Purpose after archive.
## Requirements
### Requirement: Org-scoped runner config fields

A runner configuration entry MUST support `org` as an alternative to `repo` for GitHub registration scope. When `org` is set, the runner MUST register against `https://github.com/<org>` and use org-scoped GitHub API endpoints.

#### Scenario: Org field selects org scope

- **WHEN** a runner block has `org: mycompany` and no `repo` field
- **THEN** `Scope()` MUST return `"org"` and `ScopeTarget()` MUST return `"mycompany"`

#### Scenario: Repo field selects repo scope

- **WHEN** a runner block has `repo: mycompany/app` and no `org` field
- **THEN** `Scope()` MUST return `"repo"` and `ScopeTarget()` MUST return `"mycompany/app"`

#### Scenario: Repo or org required

- **WHEN** a runner block has neither `repo` nor `org`
- **THEN** config validation MUST fail with an error naming the runner

#### Scenario: Repo and org are mutually exclusive

- **WHEN** a runner block has both `repo` and `org` set
- **THEN** config validation MUST fail with an error indicating only one may be set

### Requirement: Runner group requires org scope

The `group` field MUST only be valid on org-scoped runners. It MUST be passed as `--runnergroup` during native and container registration.

#### Scenario: Group without org is rejected

- **WHEN** a runner block has `group: ci-pool` but no `org` field
- **THEN** config validation MUST fail indicating group requires org

#### Scenario: Group is passed at registration

- **WHEN** an org-scoped runner with `group: ci-pool` is set up
- **THEN** registration MUST include `--runnergroup ci-pool` (native `config.sh`/`config.cmd` and container entrypoint)

#### Scenario: Empty group uses GitHub default

- **WHEN** an org-scoped runner has no `group` field
- **THEN** registration MUST NOT pass `--runnergroup` and GitHub MUST assign the runner to the org's Default group

### Requirement: CLI supports org runner creation

`gh sr runner add` MUST accept `--org` as an alternative to `--repo`, and `--group` for org-level runner groups.

#### Scenario: Add org runner via CLI

- **WHEN** user runs `gh sr runner add myorg-ci --org mycompany --host vps-1`
- **THEN** the runner entry MUST be written to config with `org: mycompany` and no `repo` field

#### Scenario: CLI rejects both repo and org

- **WHEN** user runs `gh sr runner add x --repo o/r --org mycompany --host h1`
- **THEN** the command MUST fail before writing config

#### Scenario: CLI requires one target

- **WHEN** user runs `gh sr runner add x --host h1` without `--repo` or `--org`
- **THEN** the command MUST fail indicating `--repo` or `--org` is required

### Requirement: Org-wide runner name uniqueness

GitHub enforces runner instance name uniqueness within the registration scope. For org-scoped runners, instance names (`name-1`, `name-2`, …) MUST be unique across the entire organization, not per repository.

#### Scenario: Documentation states org-wide uniqueness

- **WHEN** a user reads org runner documentation
- **THEN** it MUST state that instance names are unique org-wide for `org:` runners

