## 1. Core Docker ensure helper

- [x] 1.1 Add `internal/runner/docker.go` with `EnsureHostDocker(h *host.Host, w io.Writer) error` and a sentinel error (e.g. `ErrDockerGroupPending`) for Option B early exit
- [x] 1.2 Implement detection: `docker --version` (CLI) then `docker info` (daemon + permissions); branch into install / start-daemon / permission-denied / OK paths per design
- [x] 1.3 Implement curl bootstrap (apt/yum/apk) before get.docker.com, reusing the same pattern as `setupNative` in `native.go`
- [x] 1.4 Implement get.docker.com install via `sudoPrelude()` + `curl -fsSL https://get.docker.com | $SUDO sh` + `systemctl enable --now docker`
- [x] 1.5 After fresh install, run `usermod -aG docker <h.SSHUser()>` when SSH user is known; return `ErrDockerGroupPending` with re-run message (skip re-run requirement for root SSH)

## 2. Integration

- [x] 2.1 Call `EnsureHostDocker` at the start of `setupContainer` in `container.go`; propagate `ErrDockerGroupPending` as a clean user-facing exit (not wrapped as image-build failure)
- [x] 2.2 Handle daemon-only recovery: when CLI exists but `docker info` fails, try `systemctl enable --now docker` and re-check (do not invoke get.docker.com)
- [x] 2.3 Handle permission-denied on existing install: fail with docker group guidance, no reinstall

## 3. Doctor and docs alignment

- [x] 3.1 Update `ValidateContainerPrereqs` docker-cli remediation in `internal/agentic/agentic.go` to mention `gh sr setup` auto-install on Linux and possible second run after group membership
- [x] 3.2 Update `docs/content/host-setup.md` to describe the two-run setup flow when Docker is auto-installed (Option B)

## 4. Tests

- [x] 4.1 Add `internal/runner/docker_test.go` with mock executor cases: Docker OK, CLI missing → install script invoked, fresh install → group pending error, CLI present + daemon down → systemctl start, permission denied → no reinstall, root SSH → no group-pending
- [x] 4.2 Add or extend `container_test.go` to verify `setupContainer` calls ensure before image build
- [x] 4.3 Run `go test ./internal/runner/... ./internal/agentic/...` and fix any regressions

## 5. Verification

- [x] 5.1 Manual smoke: mock host missing Docker → first setup prints install + re-run message, second setup proceeds (or document if manual VPS test deferred)
- [x] 5.2 Confirm existing hosts with Docker pre-installed show no behavior change
