## ADDED Requirements

### Requirement: Canonical MockExecutor for host.Executor tests

The codebase SHALL provide `internal/testutil.MockExecutor` implementing `host.Executor` with support for: static output/error, a `RunFn` override, sequential canned responses, and recording of executed commands in `Calls`.

#### Scenario: Package tests use shared mock instead of local duplicate

- **WHEN** a test in `internal/autostart`, `internal/runner`, or `internal/doctor` needs to stub remote command execution
- **THEN** it MUST use `testutil.MockExecutor` rather than a package-local `mockExecutor` struct

#### Scenario: RunFn override takes precedence

- **WHEN** `MockExecutor.RunFn` is set
- **THEN** `Run(cmd)` MUST invoke `RunFn` and MUST NOT return static `Output`/`RunErr`

#### Scenario: Sequential responses advance index

- **WHEN** `MockExecutor.Responses` is populated and `RunFn` is nil
- **THEN** each `Run` call MUST return the next response in order until exhausted

### Requirement: Local mock executor duplicates are removed

After migration, there MUST be no standalone `mockExecutor`, `diskMockExecutor`, `containerMockExecutor`, or `diskDoctorMock` type definitions outside `internal/testutil` (except re-exports or aliases documented in `host` tests if needed for package-local ergonomics).

#### Scenario: Grep finds no duplicate mock types

- **WHEN** searching for `type mockExecutor struct` or `type containerMockExecutor struct` in the repository
- **THEN** matches MUST appear only in `internal/testutil` (or be absent entirely if renamed to `MockExecutor`)
