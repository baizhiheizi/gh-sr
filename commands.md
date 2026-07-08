---
name: commands
description: Validated build/test/coverage commands for baizhiheizi/gh-sr
metadata:
  type: reference
---

Validated on `test-assist/diskschedule-command-seams` 2026-07-08 against `.github/workflows/ci.yml`.

```bash
go build -o gh-sr ./cmd/gh-sr/        # build binary
go build ./...                         # full-repo build
go vet ./...                           # static analysis (CI runs)
gofmt -l .                             # CI format check; use gofmt -w <files>
go test ./... -race -count=1          # CI parity (race + fresh cache)
go test ./... -cover                   # per-package coverage summary
go test ./... -coverprofile=cov.out && go tool cover -html=cov.out
go test ./... -run='^$' -bench=. -benchmem -count=1  # benchmarks
```

Makefile aliases: `make build`, `make test`, `make vet`, `make check`.

Sandbox note: Go proxy may be blocked on first invocation; subsequent runs are cached. Safeoutputs CLI is the path to GitHub writes.

[[repo]] [[testing-notes]]
