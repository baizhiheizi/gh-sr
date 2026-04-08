package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/doctor"
	"github.com/an-lee/gh-sr/internal/editor"
	"github.com/an-lee/gh-sr/internal/ops"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/tui"
)

var version = "dev"

var (
	cfgFile    string
	filterHost string
	filterRepo string
)

// linuxSetupPrivilegesHelp is appended to root/setup/update Long text (non-interactive SSH + sudo behavior on Linux).
const linuxSetupPrivilegesHelp = `

Linux hosts: gh sr setup and update may run package installs and similar steps on the remote host. For a non-root
SSH user, gh sr uses sudo when the sudo binary exists on the remote PATH; SSH is non-interactive, so passwordless
sudo (or SSH as root) is usually required for those steps to succeed. For docker mode without Docker installed,
install Docker yourself or ensure sudo works; for native mode, pre-install curl/tar and runner OS dependencies
if you cannot use sudo. See the README section "Linux SSH user and privileges".`

// serviceLongHelp documents autostart behavior for the service subcommands.
const serviceLongHelp = `

Native runners do not survive host reboot until OS autostart is installed (gh sr service install). After install,
gh sr up and gh sr down start and stop the same supervisor (systemd, launchd, or a Windows scheduled task).

Linux user units (default) require loginctl enable-linger <user> on many headless servers so systemd --user
starts at boot without an interactive login. Use --system on Linux only for a system-wide unit in
/etc/systemd/system (needs passwordless sudo or root SSH).

Docker mode uses the container restart policy unless-stopped; gh sr service install skips docker runners.`

func main() {
	root := &cobra.Command{
		Use:   "sr",
		Short: "Manage self-hosted GitHub Actions runners across multiple hosts",
		Long: `Self-hosted runner manager for GitHub (gh sr) manages self-hosted GitHub Actions runners on any combination
of Linux, macOS, and Windows hosts — all from your laptop over SSH.

Define your hosts and runners in ~/.gh-sr/runners.yml (or set GH_SR_CONFIG / --config),
then use unified commands to setup, start, stop, and monitor everything.

With no subcommand, gh sr opens the interactive dashboard on a terminal; use gh sr --help for all commands.` + linuxSetupPrivilegesHelp,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown argument %q — use a subcommand or gh sr --help", args[0])
			}
			return runDashboard()
		},
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (empty = auto; use \"gh sr config path\" to print the resolved file)")
	root.PersistentFlags().StringVar(&filterHost, "host", "", "filter by host name")
	root.PersistentFlags().StringVar(&filterRepo, "repo", "", "filter by repo (owner/repo)")

	root.AddCommand(
		initCmd(),
		addCmd(),
		doctorCmd(),
		setupCmd(),
		upCmd(),
		downCmd(),
		restartCmd(),
		statusCmd(),
		logsCmd(),
		cleanupCmd(),
		updateCmd(),
		serviceCmd(),
		configCmd(),
		dashboardCmd(),
		hostsCmd(),
		versionCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runDashboard() error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprintln(os.Stderr, tui.NonTTYHint)
		return nil
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	cfgPath, err := config.ResolveConfigPath(cfgFile)
	if err != nil {
		return err
	}
	envPath, err := config.EnvFilePath()
	if err != nil {
		return err
	}
	return tui.RunDashboard(cfg, tui.DashboardOpts{
		ConfigPath: cfgPath,
		EnvPath:    envPath,
		FilterHost: filterHost,
		FilterRepo: filterRepo,
	})
}

func loadConfig() (*config.Config, error) {
	if err := config.BootstrapEnv(); err != nil {
		return nil, err
	}
	path, err := config.ResolveConfigPath(cfgFile)
	if err != nil {
		return nil, err
	}
	return config.LoadFromPath(path)
}

func newManager(cfg *config.Config, w io.Writer) (*runner.Manager, error) {
	tok, err := config.ResolveToken(cfg)
	if err != nil {
		return nil, err
	}
	m := runner.NewManager(tok)
	m.Out = w
	return m, nil
}

func initCmd() *cobra.Command {
	var force bool
	var quick bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create ~/.gh-sr with template runners.yml and env file",
		Long: `Create ~/.gh-sr directory with a template runners.yml and env file.

Use --quick for an interactive setup that asks for a repo and host address,
auto-detects everything else, and writes a working config ready for gh sr up.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.SrDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			runnersPath := filepath.Join(dir, "runners.yml")

			envPath, err := config.EnvFilePath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				if err := os.WriteFile(envPath, []byte(config.EnvFileTemplate), 0o600); err != nil {
					return err
				}
				fmt.Printf("Wrote %s\n", envPath)
			}

			if quick {
				return runQuickInit(runnersPath, force)
			}

			if _, err := os.Stat(runnersPath); err == nil && !force {
				fmt.Printf("Already exists (use --force to overwrite): %s\n", runnersPath)
			} else {
				if err := os.WriteFile(runnersPath, config.RunnersYMLTemplate, 0o600); err != nil {
					return err
				}
				fmt.Printf("Wrote %s\n", runnersPath)
			}
			fmt.Println("\nNext: edit ~/.gh-sr/runners.yml, then run `gh sr doctor` and `gh sr status`.")
			fmt.Println("Authentication: run `gh auth login`.")
			fmt.Println("Tip: use `gh sr init --quick` for interactive setup.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing ~/.gh-sr/runners.yml")
	cmd.Flags().BoolVar(&quick, "quick", false, "interactive setup: prompts for repo and host, auto-detects everything else")
	return cmd
}

func runQuickInit(runnersPath string, force bool) error {
	scanner := bufio.NewScanner(os.Stdin)
	prompt := func(label, defaultVal string) string {
		if defaultVal != "" {
			fmt.Printf("%s [%s]: ", label, defaultVal)
		} else {
			fmt.Printf("%s: ", label)
		}
		if scanner.Scan() {
			v := strings.TrimSpace(scanner.Text())
			if v != "" {
				return v
			}
		}
		return defaultVal
	}

	fmt.Println("=== gh sr quick setup ===")
	fmt.Println("This will create a working config. OS, arch, mode, and labels are all auto-detected.")
	fmt.Println()

	repo := prompt("GitHub repo (owner/repo)", "")
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	if !strings.Contains(repo, "/") {
		return fmt.Errorf("repo must be in owner/repo format")
	}

	addr := prompt("SSH address of runner host (user@host, or 'local' for this machine)", "local")

	hostName := "runner-host"
	if config.IsLocalAddr(addr) {
		hostName = "local"
	} else {
		parts := strings.SplitN(addr, "@", 2)
		if len(parts) == 2 {
			h := strings.Split(parts[1], ":")[0]
			h = strings.ReplaceAll(h, ".", "-")
			if h != "" {
				hostName = h
			}
		}
	}

	baseName := strings.ReplaceAll(strings.Split(repo, "/")[1], ".", "-")
	runnerName := baseName + "-" + hostName

	countStr := prompt("Number of runner instances", "1")
	count := 1
	if _, err := fmt.Sscanf(countStr, "%d", &count); err != nil || count < 1 {
		count = 1
	}

	agenticAnswer := prompt("Use GitHub Agentic Workflows (gh-aw) profile? (y/n)", "n")
	agentic := strings.ToLower(strings.TrimSpace(agenticAnswer)) == "y"

	var profileLine string
	if agentic {
		profileLine = "\n    profile: agentic"
	}

	seed := fmt.Sprintf(`github: {}
hosts:
  %s:
    addr: %s
runners:
  - name: %s
    repo: %s
    host: %s
    count: %d%s
`, hostName, addr, runnerName, repo, hostName, count, profileLine)

	if _, err := os.Stat(runnersPath); err == nil && !force {
		fmt.Printf("\n%s already exists. Overwrite? [y/N]: ", runnersPath)
		if scanner.Scan() {
			if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
				fmt.Println("Aborted.")
				return nil
			}
		}
	}

	if err := os.WriteFile(runnersPath, []byte(seed), 0o600); err != nil {
		return err
	}
	fmt.Printf("\nWrote %s\n", runnersPath)

	// Validate the generated config loads.
	if _, err := config.Load(runnersPath); err != nil {
		return fmt.Errorf("generated config is invalid: %w", err)
	}

	fmt.Println()
	fmt.Println("Config generated! Summary:")
	fmt.Printf("  Host:   %s (%s)\n", hostName, addr)
	fmt.Printf("  Runner: %s -> %s (x%d)\n", runnerName, repo, count)
	if agentic {
		fmt.Println("  Profile: agentic (docker, host network, NET_ADMIN, gh-aw label)")
	} else {
		fmt.Println("  Mode:   auto-detected at runtime")
	}
	fmt.Println("  Labels: auto-generated from host os/arch")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  gh sr doctor   # verify config, GitHub access, and host connectivity")
	fmt.Println("  gh sr up       # setup + start runners (all in one)")
	fmt.Println()
	fmt.Println("Authentication: run `gh auth login`.")

	return nil
}

func addCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a host or runner to the config",
	}
	cmd.AddCommand(addHostCmd(), addRunnerCmd())
	return cmd
}

func addHostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "host <name> <addr>",
		Short: "Add a host entry (os/arch auto-detected over SSH)",
		Long:  "Adds a host to runners.yml. Only name and SSH address are required; os and arch are auto-detected at runtime.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.ResolveConfigPath(cfgFile)
			if err != nil {
				return err
			}
			name, addr := args[0], args[1]
			if err := config.AddHost(cfgPath, name, addr, "", ""); err != nil {
				return err
			}
			fmt.Printf("Added host %q (%s) to %s\n", name, addr, cfgPath)
			return nil
		},
	}
}

func addRunnerCmd() *cobra.Command {
	var (
		repo      string
		org       string
		group     string
		host      string
		count     int
		labels    []string
		mode      string
		profile   string
		ephemeral bool
	)
	cmd := &cobra.Command{
		Use:   "runner <name>",
		Short: "Add a runner entry (labels auto-generated if omitted)",
		Long: `Adds a runner to runners.yml. Labels are auto-generated from host os/arch if not specified.

Use --profile agentic to auto-configure for GitHub Agentic Workflows (gh-aw):
sets docker mode, host networking, NET_ADMIN capability, and a gh-aw label.

Use --org instead of --repo for organization-level runners, and --group
to assign the runner to a runner group.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.ResolveConfigPath(cfgFile)
			if err != nil {
				return err
			}
			name := args[0]
			if repo == "" && org == "" {
				return fmt.Errorf("--repo or --org is required")
			}
			if repo != "" && org != "" {
				return fmt.Errorf("specify --repo or --org, not both")
			}
			if host == "" {
				return fmt.Errorf("--host is required")
			}
			opts := config.AddRunnerOpts{
				Name:      name,
				Repo:      repo,
				Org:       org,
				Group:     group,
				Host:      host,
				Count:     count,
				Labels:    labels,
				Mode:      mode,
				Profile:   profile,
				Ephemeral: ephemeral,
			}
			if err := config.AddRunnerFull(cfgPath, opts); err != nil {
				return err
			}
			target := repo
			if org != "" {
				target = "org:" + org
			}
			fmt.Printf("Added runner %q (%s, host=%s) to %s\n", name, target, host, cfgPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", "", "GitHub owner/repo (required unless --org is set)")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization (for org-level runners)")
	cmd.Flags().StringVar(&group, "group", "", "runner group name (org-level runners only)")
	cmd.Flags().StringVar(&host, "host", "", "host name from config (required)")
	cmd.Flags().IntVar(&count, "count", 1, "number of parallel instances")
	cmd.Flags().StringSliceVar(&labels, "labels", nil, "runner labels (auto-generated if empty)")
	cmd.Flags().StringVar(&mode, "mode", "", "runner mode: docker or native (auto-detected if empty)")
	cmd.Flags().StringVar(&profile, "profile", "", "runner profile: 'agentic' for GitHub Agentic Workflows")
	cmd.Flags().BoolVar(&ephemeral, "ephemeral", false, "register as ephemeral (one job then deregister)")
	return cmd
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect or edit configuration",
	}
	cmd.AddCommand(
		configPathCmd(),
		configShowCmd(),
		configEditCmd(),
		configEditEnvCmd(),
		configValidateCmd(),
	)
	return cmd
}

func doctorCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check config, GitHub API access, and host prerequisites",
		Long:  "Validates local paths, configuration, GitHub API access, and SSH targets (Docker or native tooling per runner mode). See README \"Host setup\" for steps gh sr cannot automate.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.BootstrapEnv(); err != nil {
				return err
			}
			cfgPath, err := config.ResolveConfigPath(cfgFile)
			if err != nil {
				return err
			}
			envPath, err := config.EnvFilePath()
			if err != nil {
				return err
			}

			cfg, cfgErr := config.LoadFromPath(cfgPath)
			var gh *runner.GitHubClient
			hasGitHubToken := false
			if cfg != nil {
				tok, tokErr := config.ResolveToken(cfg)
				if tokErr == nil {
					gh = runner.NewGitHubClient(tok)
					hasGitHubToken = true
				}
			}

			res := doctor.Run(cmd.OutOrStdout(), cfgPath, envPath, cfg, cfgErr, gh, hasGitHubToken, filterHost, filterRepo, strict)
			if code := doctor.ExitCode(res, strict); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "non-zero exit if any check is WARN (default: only FAIL fails the run)")
	return cmd
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print resolved config and env file paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolveConfigPath(cfgFile)
			if err != nil {
				return err
			}
			envPath, err := config.EnvFilePath()
			if err != nil {
				return err
			}
			envStatus := "not present"
			if _, err := os.Stat(envPath); err == nil {
				envStatus = "present"
			}
			fmt.Printf("Config file: %s\n", path)
			fmt.Printf("Env file:    %s (%s)\n", envPath, envStatus)
			return nil
		},
	}
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print resolved configuration (token source summarized)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			tui.PrintConfig(cfg)
			return nil
		},
	}
}

func configEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the resolved config file in $VISUAL or $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.ResolveConfigPath(cfgFile)
			if err != nil {
				return err
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return fmt.Errorf("config file does not exist: %s\nRun `gh sr init` to create it", path)
			}
			if err := editor.Open(path); err != nil {
				return err
			}
			if _, err := loadConfig(); err != nil {
				return fmt.Errorf("config invalid after edit: %w", err)
			}
			fmt.Println("Config is valid.")
			return nil
		},
	}
}

func configEditEnvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit-env",
		Short: "Open ~/.gh-sr/env in $VISUAL or $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.SrDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			envPath, err := config.EnvFilePath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				if err := os.WriteFile(envPath, []byte(config.EnvFileTemplate), 0o600); err != nil {
					return err
				}
			}
			if err := editor.Open(envPath); err != nil {
				return err
			}
			if _, err := loadConfig(); err != nil {
				return fmt.Errorf("config invalid after editing env: %w", err)
			}
			fmt.Println("Config is valid.")
			return nil
		},
	}
}

func configValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the resolved config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := loadConfig()
			if err != nil {
				return err
			}
			fmt.Println("OK")
			return nil
		},
	}
}

// --- Commands ---

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup [runner-names...]",
		Short: "Install runner prerequisites and configure runners on hosts",
		Long:  "Installs and configures runners on remote hosts over SSH." + linuxSetupPrivilegesHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			return ops.Setup(cmd.OutOrStdout(), cfg, mgr, filterHost, filterRepo, args)
		},
	}
}

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up [runner-names...]",
		Short: "Start runners (all, or filtered by name/host/repo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			return ops.Up(cmd.OutOrStdout(), cfg, mgr, filterHost, filterRepo, args)
		},
	}
}

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [runner-names...]",
		Short: "Stop runners",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			return ops.Down(cmd.OutOrStdout(), cfg, mgr, filterHost, filterRepo, args)
		},
	}
}

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [runner-names...]",
		Short: "Restart runners (stop then start)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			return ops.Restart(cmd.OutOrStdout(), cfg, mgr, filterHost, filterRepo, args)
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of all runners",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			allStatuses, err := ops.CollectStatus(cmd.OutOrStdout(), cfg, mgr, filterHost, filterRepo, args)
			if err != nil {
				return err
			}
			tui.PrintStatusTable(allStatuses)
			return nil
		},
	}
}

func logsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <runner-name>",
		Short: "Show recent logs from a runner",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			output, err := ops.Logs(cfg, mgr, filterHost, args[0])
			if err != nil {
				return err
			}
			fmt.Println(output)
			return nil
		},
	}
}

func cleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Remove offline/ghost runners from GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			_, err = ops.CleanupOffline(cmd.OutOrStdout(), cfg, mgr)
			return err
		},
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update [runner-names...]",
		Short: "Update runner binary on hosts (remove + setup + start)",
		Long:  "Removes each runner, runs setup again, then starts it. Re-runs the same remote install paths as gh sr setup." + linuxSetupPrivilegesHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr, err := newManager(cfg, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			return ops.Update(cmd.OutOrStdout(), cfg, mgr, filterHost, filterRepo, args)
		},
	}
}

func serviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Install or manage OS autostart for native runners",
		Long:  "Manage boot-time autostart for native-mode self-hosted runners." + serviceLongHelp,
	}
	var system bool
	install := &cobra.Command{
		Use:   "install [runner-names...]",
		Short: "Install autostart for native runners (all or filtered)",
		Long:  "Writes systemd user units (Linux), LaunchAgents (macOS), or a logon scheduled task (Windows), then enables and starts them." + serviceLongHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			return ops.ServiceInstall(cmd.OutOrStdout(), cfg, filterHost, filterRepo, args, system)
		},
	}
	install.Flags().BoolVar(&system, "system", false, "Linux only: install systemd unit under /etc/systemd/system (passwordless sudo or root SSH)")
	uninstall := &cobra.Command{
		Use:   "uninstall [runner-names...]",
		Short: "Remove autostart definitions installed by gh sr",
		Long:  "Stops and removes systemd units, LaunchAgents, or scheduled tasks created by gh sr service install." + serviceLongHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			return ops.ServiceUninstall(cmd.OutOrStdout(), cfg, filterHost, filterRepo, args)
		},
	}
	status := &cobra.Command{
		Use:   "status [runner-names...]",
		Short: "Show autostart state per runner instance",
		Long:  "Reports whether gh sr autostart is installed and the service state (native), or docker restart policy notes (docker)." + serviceLongHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			return ops.ServiceStatus(cmd.OutOrStdout(), cfg, filterHost, filterRepo, args)
		},
	}
	cmd.AddCommand(install, uninstall, status)
	return cmd
}

func hostsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "hosts",
		Short: "Show host resource usage (CPU, memory, disk, load, uptime)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			metrics := ops.CollectHostMetrics(cmd.OutOrStdout(), cfg, filterHost)
			tui.PrintHostMetricsTable(metrics)
			return nil
		},
	}
}

func dashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Launch interactive TUI dashboard",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDashboard()
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print gh sr version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
}
