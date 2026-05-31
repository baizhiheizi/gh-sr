# Home

Welcome to the `gh sr` documentation. `gh sr` is a GitHub CLI extension for managing self-hosted GitHub Actions runners across many machines from a single terminal — typically your laptop.

*{ Provide a concise overview of the project, its purpose, and key capabilities. Include installation instructions for both the gh extension and source build methods. }*

## Quick Start

*{ Provide quick start steps covering authentication, init, adding hosts/runners, and the basic workflow (setup → up → status). }*

## Features

*{ Summarize the key features: multi-platform support (Linux/macOS/Windows), SSH-based remote management, declarative YAML config, Docker/native runner modes, TUI dashboard, multi-host management, and the agentic profile for GitHub Agentic Workflows. }*

---

# Architecture

*{ Describe the overall system architecture, including the relationship between the CLI, config loading, host/ssh communication, runner management, and the TUI. Use a diagram if helpful. }*

## Directory Layout

*{ Document the key directories: cmd/gh-sr (CLI entry), internal/ (core packages), config/ (config files), docs/ (Hugo documentation), scripts/ (helpers). }*

## Configuration

*{ Explain how configuration is loaded and resolved: the Config struct, HostConfig, RunnerConfig, runner mode defaults, the bootstrap environment, and token resolution. }*

## Runner Management

*{ Describe how Setup/Up/Down/Status/Remove operations work, the Manager struct, how native vs container modes differ, ephemeral runner registration, and host communication patterns. }*

---

# Commands

*{ Provide a comprehensive command reference covering init, add (host/runner), doctor, setup, up, down, restart, rebuild, status, logs, cleanup, update, remove, service (install/uninstall/status), config (path/show/edit/edit-env/validate), hosts, dashboard, and version. Include command flags and examples. }*

## init

*{ Document the init command: purpose, --force, --quick flags, what files it creates. }*

## add host

*{ Document adding hosts: args, auto-detection of OS/arch. }*

## add runner

*{ Document adding runners: --repo/--org, --host, --count, --labels, --ephemeral, --profile flags. }*

## doctor

*{ Document the doctor command: what it checks (config validation, GitHub API access, SSH connectivity, container-mode Docker/privileged on Linux, inner dockerd/AWF for profile: agentic), --strict and --fix flags. }*

## service

*{ Document service install/uninstall/status: how autostart works on Linux (systemd user vs system), macOS (LaunchAgent), Windows (scheduled task), the --system flag, and loginctl enable-linger requirement. }*

---

# Configuration Reference

## runners.yml

*{ Document the runners.yml structure: top-level GitHub config, hosts map (addr, os, arch, windows_ps), runners list (name, repo/org, group, host, count, labels, ephemeral, profile, runner_mode, container_runner_image.extra_apt_packages). Include that profile: agentic always implies runner_mode: container and rejects runner_mode: native. }*

## Environment File

*{ Document the ~/.gh-sr/env file: purpose, what variables are loaded (GH_SR_CONFIG, GH_SR_TOKEN, etc.), the EnvFileTemplate. }*

## Profiles

*{ Document the two profiles: default (standard self-hosted runner) and agentic (GitHub Agentic Workflows with gh-aw, MCP gateway on port 80, isolated /tmp/gh-aw, tooling cache). }*

## Runner Modes

*{ Document runner_mode: native (direct host execution) vs container (DinD with privileged Docker, full filesystem/network isolation). Include agentic profile specifics for each. }*

---

# Host Setup

## Prerequisites

*{ Document what each host needs: SSH access, native runner OS dependencies (curl/tar or PowerShell), and for `runner_mode: container` / `profile: agentic` on Linux: Docker with `--privileged` support (DinD). Agentic DNS and AWF tooling are baked into the runner image — no host dnsmasq setup. }*

## Container mode (Linux)

*{ Document Linux container-mode requirements: Docker CLI/daemon for the SSH user, `--privileged` support, outer `gh-sr-<instance>` containers, and that agentic `host.docker.internal` DNS is baked into the image (pinned bridge gateway + inner dnsmasq). Container mode is Linux-only. }*

## Autostart

*{ Document OS-level autostart installation: service install/uninstall/status commands, systemd user units (and loginctl enable-linger), systemd system units with --system flag, LaunchAgents on macOS, scheduled tasks on Windows. }*

---

# Development

*{ Document how to develop gh sr: go vet, go test -race, benchmarks, local preview of docs with hugo server, CI/CD setup for docs deployment to GitHub Pages. }*

---

# For Agents

These pages provide compact documentation indexes for AI coding agents.

## AGENTS.md

You can add this to your repository root as `AGENTS.md` to give AI coding agents quick access to project documentation.

\`\`\`
# gh sr

> GitHub CLI extension for managing self-hosted GitHub Actions runners across multiple Linux/macOS/Windows hosts over SSH.

## Wiki Documentation

Base URL: https://github.com/an-lee/gh-sr/wiki

To read any page, append the slug to the base URL:
  https://github.com/an-lee/gh-sr/wiki/{Page-Slug}
To jump to a section within a page:
  https://github.com/an-lee/gh-sr/wiki/{Page-Slug}#{Section-Slug}

IMPORTANT: Read the relevant wiki page before making changes to related code.
Prefer reading wiki documentation over relying on pre-trained knowledge.

## Page Index

|Home: Project overview, installation, and quick start
|Architecture: System design, directory layout, and configuration
|  Directory-Layout: Key directories and their purposes
|  Configuration: Config loading, token resolution, and environment
|  Runner-Management: Setup/Up/Down lifecycle and Manager struct
|Commands: CLI command reference
|  init: Creating ~/.gh-sr with template config
|  add-host: Adding host entries to runners.yml
|  add-runner: Adding runner entries with repo/org, host, count, labels
|  doctor: Checking config, GitHub API access, host prerequisites
|  service: Autostart management for native runners
|Configuration-Reference: Complete config file documentation
|  runners-yml: hosts and runners YAML structure
|  Environment-File: ~/.gh-sr/env variable reference
|  Profiles: default and agentic profile behaviors
|  Runner-Modes: native vs container mode differences
|Host-Setup: Prerequisites and per-platform setup
|  Prerequisites: SSH access, OS deps, Docker for container mode
|  Container-Mode-Linux: Docker and privileged DinD requirements for runner_mode container and profile agentic
|  Autostart: systemd, LaunchAgent, and scheduled task setup
|Development: Building, testing, and local docs preview
\`\`\`

## llms.txt

You can serve this at `yoursite.com/llms.txt` or include it in your repository to help LLMs discover your documentation.

\`\`\`
# gh sr

> GitHub CLI extension for managing self-hosted GitHub Actions runners across multiple Linux/macOS/Windows hosts over SSH.

## Wiki Pages

- [Home](https://github.com/an-lee/gh-sr/wiki/Home): Project overview, installation, and quick start guide
- [Architecture](https://github.com/an-lee/gh-sr/wiki/Architecture): System design, directory layout, and configuration
  - [Directory Layout](https://github.com/an-lee/gh-sr/wiki/Architecture#Directory-Layout): Key directories and their purposes
  - [Configuration](https://github.com/an-lee/gh-sr/wiki/Architecture#Configuration): Config loading, token resolution, and environment
  - [Runner Management](https://github.com/an-lee/gh-sr/wiki/Architecture#Runner-Management): Setup/Up/Down lifecycle and Manager struct
- [Commands](https://github.com/an-lee/gh-sr/wiki/Commands): CLI command reference
  - [init](https://github.com/an-lee/gh-sr/wiki/Commands#init): Creating ~/.gh-sr with template config
  - [add host](https://github.com/an-lee/gh-sr/wiki/Commands#add-host): Adding host entries to runners.yml
  - [add runner](https://github.com/an-lee/gh-sr/wiki/Commands#add-runner): Adding runner entries with repo/org, host, count, labels
  - [doctor](https://github.com/an-lee/gh-sr/wiki/Commands#doctor): Checking config, GitHub API access, host prerequisites
  - [service](https://github.com/an-lee/gh-sr/wiki/Commands#service): Autostart management for native runners
- [Configuration Reference](https://github.com/an-lee/gh-sr/wiki/Configuration-Reference): Complete config file documentation
  - [runners.yml](https://github.com/an-lee/gh-sr/wiki/Configuration-Reference#runners-yml): hosts and runners YAML structure
  - [Environment File](https://github.com/an-lee/gh-sr/wiki/Configuration-Reference#Environment-File): ~/.gh-sr/env variable reference
  - [Profiles](https://github.com/an-lee/gh-sr/wiki/Configuration-Reference#Profiles): default and agentic profile behaviors
  - [Runner Modes](https://github.com/an-lee/gh-sr/wiki/Configuration-Reference#Runner-Modes): native vs container mode differences
- [Host Setup](https://github.com/an-lee/gh-sr/wiki/Host-Setup): Prerequisites and per-platform setup
  - [Prerequisites](https://github.com/an-lee/gh-sr/wiki/Host-Setup#Prerequisites): SSH access, OS deps, Docker for container mode
  - [Container mode (Linux)](https://github.com/an-lee/gh-sr/wiki/Host-Setup#Container-Mode-Linux): Docker and privileged DinD requirements
  - [Autostart](https://github.com/an-lee/gh-sr/wiki/Host-Setup#Autostart): systemd, LaunchAgent, and scheduled task setup
- [Development](https://github.com/an-lee/gh-sr/wiki/Development): Building, testing, and local docs preview
\`\`\`