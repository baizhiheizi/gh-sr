## 1. Documentation — configuration and templates

- [x] 1.1 Add org-scoped runner example block to `docs/content/configuration.md` (with `org:`, `group:`, `count:`, `labels:`)
- [x] 1.2 Update name-uniqueness note in `configuration.md` to cover both repo-wide and org-wide scopes
- [x] 1.3 Add commented org-scoped runner example to `config/runners.yml`
- [x] 1.4 Add commented org-scoped runner example to `internal/config/runners.yml.template`
- [x] 1.5 Link to org runners guide from `configuration.md` config reference section

## 2. Documentation — authentication and org guide

- [x] 2.1 Add org-level permission requirements and API paths to `docs/content/authentication.md`
- [x] 2.2 Create `docs/content/guides/org-runners.md` covering org runners, runner groups, workflow targeting, security, and migration steps
- [x] 2.3 Link org runners guide from `authentication.md`
- [x] 2.4 Register guide in Hugo nav/weight if the docs theme requires explicit menu entry

## 3. Diagnostics — scope display consistency

- [x] 3.1 Normalize TUI status target display to `org:<name>` prefix in `internal/tui/status.go` (match ops/runner output)
- [x] 3.2 Audit `internal/ops/ops.go` and `internal/runner/runner.go` display paths; fix any inconsistent scope formatting
- [x] 3.3 Ensure `group=<name>` appears in TUI/status when `group` is set on org runners

## 4. Diagnostics — doctor org permission hints

- [x] 4.1 Enhance doctor org API failure messages in `internal/doctor/doctor.go` with `admin:org` / org owner hint on 403 or permission errors
- [x] 4.2 Add or update doctor test covering org permission failure message content

## 5. Verification

- [x] 5.1 Run `go build ./...` and `go test ./...`
- [x] 5.2 Manually verify `gh sr runner add --help` mentions `--org` and `--group` (confirm existing text meets spec; adjust if needed)
- [x] 5.3 Review docs site renders org guide and examples correctly (if Hugo build is available locally)
