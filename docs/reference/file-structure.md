# File structure

Layout of this repository (module `github.com/an-lee/ghr`):

```text
ghr/
  cmd/
    ghr/
      main.go               # CLI entry point
  internal/
    config/
      config.go             # YAML config parsing and validation
      envfile.go            # ~/.ghr/env dotenv loader
      paths.go              # Config path resolution, ~/.ghr helpers
      load.go               # LoadFromPath with missing-file hints
      template.go           # Embedded template for ghr init
      runners.yml.template  # Default runners.yml content
    editor/
      editor.go             # $VISUAL / $EDITOR / platform default
    host/
      host.go               # Host abstraction
      connection.go         # SSH connection management (Executor interface)
      local.go              # Local command execution (addr: local)
    doctor/
      doctor.go             # ghr doctor diagnostics
    ops/
      ops.go                # Shared setup/up/down/restart/update/status/logs/cleanup (CLI + TUI)
      service.go            # ghr service install/uninstall/status
    autostart/
      autostart.go          # systemd / launchd / Windows task install and Detect/Start/Stop
      active.go             # Supervisor active check for ghr status
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
    runners.yml             # Example YAML (not auto-loaded; use GHR_CONFIG or -c)
  docs/                     # MkDocs source (this site)
  go.mod
  go.sum
```
