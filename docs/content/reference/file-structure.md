---
title: "File Structure"
weight: 10
---

# File structure

Layout of this repository (module `github.com/an-lee/gh-sr`):

```text
gh-sr/                      # repository root (GitHub: gh-sr)
  cmd/
    gh-sr/
      main.go               # CLI entry point (binary name: gh-sr; command: gh sr)
  internal/
    config/
      config.go             # YAML config parsing and validation
      envfile.go            # ~/.gh-sr/env dotenv loader
      paths.go              # Config path resolution, ~/.gh-sr helpers
      load.go               # LoadFromPath with missing-file hints
      template.go           # Embedded template for gh sr init
      runners.yml.template  # Default runners.yml content
    editor/
      editor.go             # $VISUAL / $EDITOR / platform default
    host/
      host.go               # Host abstraction
      connection.go         # SSH connection management (Executor interface)
      local.go              # Local command execution (addr: local)
    doctor/
      doctor.go             # gh sr doctor diagnostics
    ops/
      ops.go                # Shared setup/up/down/restart/update/status/logs/cleanup (CLI + TUI)
      service.go            # gh sr service install/uninstall/status
    autostart/
      autostart.go          # systemd / launchd / Windows task install and Detect/Start/Stop
      active.go             # Supervisor active check for gh sr status
      generate.go           # Unit and plist text generation
      sanitize.go           # Safe names for unit files and tasks
    runner/
      runner.go             # Runner lifecycle orchestration
      native.go             # Native runner management (mac/win/linux)
      docker.go             # Docker runner management
      github.go             # GitHub API client
    tui/
      dashboard.go          # Interactive TUI dashboard (model + update)
      dashboard_view.go     # TUI views and layout
      status.go             # Status table rendering
      styles.go             # Lipgloss styles
  config/
    runners.yml             # Example YAML (not auto-loaded; use GH_SR_CONFIG or -c)
  docs/                     # Hugo source (this site)
  go.mod
  go.sum
```
