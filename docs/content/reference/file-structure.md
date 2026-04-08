---
title: "File Structure"
weight: 10
---

# File structure

Layout of this repository (module `github.com/an-lee/gh-wm`):

```text
gh-wm/                      # repository root (GitHub: gh-wm)
  cmd/
    gh-wm/
      main.go               # CLI entry point (binary name: gh-wm; command: gh wm)
  internal/
    config/
      config.go             # YAML config parsing and validation
      envfile.go            # ~/.gh-wm/env dotenv loader
      paths.go              # Config path resolution, ~/.gh-wm helpers
      load.go               # LoadFromPath with missing-file hints
      template.go           # Embedded template for gh wm init
      runners.yml.template  # Default runners.yml content
    editor/
      editor.go             # $VISUAL / $EDITOR / platform default
    host/
      host.go               # Host abstraction
      connection.go         # SSH connection management (Executor interface)
      local.go              # Local command execution (addr: local)
    doctor/
      doctor.go             # gh wm doctor diagnostics
    ops/
      ops.go                # Shared setup/up/down/restart/update/status/logs/cleanup (CLI + TUI)
      service.go            # gh wm service install/uninstall/status
    autostart/
      autostart.go          # systemd / launchd / Windows task install and Detect/Start/Stop
      active.go             # Supervisor active check for gh wm status
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
    runners.yml             # Example YAML (not auto-loaded; use GH_WM_CONFIG or -c)
  docs/                     # Hugo source (this site)
  go.mod
  go.sum
```
