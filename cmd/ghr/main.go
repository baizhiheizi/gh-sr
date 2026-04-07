package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/doctor"
	"github.com/an-lee/ghr/internal/editor"
	"github.com/an-lee/ghr/internal/ops"
	"github.com/an-lee/ghr/internal/runner"
	"github.com/an-lee/ghr/internal/tui"
)

var version = "dev"

var (
	cfgFile    string
	filterHost string
	filterRepo string
)

// linuxSetupPrivilegesHelp is appended to root/setup/update Long text (non-interactive SSH + sudo behavior on Linux).
const linuxSetupPrivilegesHelp = `

Linux hosts: ghr setup and update may run package installs and similar steps on the remote host. For a non-root
SSH user, ghr uses sudo when the sudo binary exists on the remote PATH; SSH is non-interactive, so passwordless
sudo (or SSH as root) is usually required for those steps to succeed. For docker mode without Docker installed,
install Docker yourself or ensure sudo works; for native mode, pre-install curl/tar and runner OS dependencies
if you cannot use sudo. See the README section "Linux SSH user and privileges".`

// serviceLongHelp documents autostart behavior for the service subcommands.
const serviceLongHelp = `

Native runners do not survive host reboot until OS autostart is installed (ghr service install). After install,
ghr up and ghr down start and stop the same supervisor (systemd, launchd, or a Windows scheduled task).

Linux user units (default) require loginctl enable-linger <user> on many headless servers so systemd --user
starts at boot without an interactive login. Use --system on Linux only for a system-wide unit in
/etc/systemd/system (needs passwordless sudo or root SSH).

Docker mode uses the container restart policy unless-stopped; ghr service install skips docker runners.`

func main() {
	root := &cobra.Command{
		Use:   "ghr",
		Short: "Manage self-hosted GitHub Actions runners across multiple hosts",
		Long: `ghr manages self-hosted GitHub Actions runners on any combination
of Linux, macOS, and Windows hosts — all from your laptop over SSH.

Define your hosts and runners in ~/.ghr/runners.yml (or set GHR_CONFIG / --config),
then use unified commands to setup, start, stop, and monitor everything.

With no subcommand, ghr opens the interactive dashboard on a terminal; use ghr --help for all commands.` + linuxSetupPrivilegesHelp,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown argument %q — use a subcommand or ghr --help", args[0])
			}
			return runDashboard()
		},
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path (empty = auto; use \"ghr config path\" to print the resolved file)")
	root.PersistentFlags().StringVar(&filterHost, "host", "", "filter by host name")
	root.PersistentFlags().StringVar(&filterRepo, "repo", "", "filter by repo (owner/repo)")

	root.AddCommand(
		initCmd(),
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
		helpCmd(root),
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

func newManager(cfg *config.Config, w io.Writer) *runner.Manager {
	m := runner.NewManager(cfg.GitHub.PAT)
	m.Out = w
	return m
}

func initCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create ~/.ghr with template runners.yml and env file",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.GhrDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			runnersPath := filepath.Join(dir, "runners.yml")
			if _, err := os.Stat(runnersPath); err == nil && !force {
				fmt.Printf("Already exists (use --force to overwrite): %s\n", runnersPath)
			} else {
				if err := os.WriteFile(runnersPath, config.RunnersYMLTemplate, 0o600); err != nil {
					return err
				}
				fmt.Printf("Wrote %s\n", runnersPath)
			}
			envPath, err := config.EnvFilePath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				if err := os.WriteFile(envPath, []byte(config.EnvFileTemplate), 0o600); err != nil {
					return err
				}
				fmt.Printf("Wrote %s\n", envPath)
			} else {
				fmt.Printf("Unchanged (already exists): %s\n", envPath)
			}
			fmt.Println("\nNext: edit ~/.ghr/runners.yml, set GITHUB_PAT in ~/.ghr/env, then run `ghr config validate`, `ghr doctor`, and `ghr status`.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing ~/.ghr/runners.yml")
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
		Long:  "Validates local paths, configuration, PAT access to the GitHub API, and SSH targets (Docker or native tooling per runner mode). See README \"Host setup\" for steps ghr cannot automate.",
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
			if cfg != nil {
				gh = runner.NewGitHubClient(cfg.GitHub.PAT)
			}

			res := doctor.Run(cmd.OutOrStdout(), cfgPath, envPath, cfg, cfgErr, gh, filterHost, filterRepo, strict)
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
		Short: "Print resolved configuration (PAT redacted)",
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
				return fmt.Errorf("config file does not exist: %s\nRun `ghr init` to create it", path)
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
		Short: "Open ~/.ghr/env in $VISUAL or $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.GhrDir()
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
			mgr := newManager(cfg, cmd.OutOrStdout())
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
			mgr := newManager(cfg, cmd.OutOrStdout())
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
			mgr := newManager(cfg, cmd.OutOrStdout())
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
			mgr := newManager(cfg, cmd.OutOrStdout())
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
			mgr := newManager(cfg, cmd.OutOrStdout())
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
			mgr := newManager(cfg, cmd.OutOrStdout())
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
			mgr := newManager(cfg, cmd.OutOrStdout())
			_, err = ops.CleanupOffline(cmd.OutOrStdout(), cfg, mgr)
			return err
		},
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update [runner-names...]",
		Short: "Update runner binary on hosts (remove + setup + start)",
		Long:  "Removes each runner, runs setup again, then starts it. Re-runs the same remote install paths as ghr setup." + linuxSetupPrivilegesHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr := newManager(cfg, cmd.OutOrStdout())
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
		Short: "Remove autostart definitions installed by ghr",
		Long:  "Stops and removes systemd units, LaunchAgents, or scheduled tasks created by ghr service install." + serviceLongHelp,
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
		Long:  "Reports whether ghr autostart is installed and the service state (native), or docker restart policy notes (docker)." + serviceLongHelp,
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
		Short: "Print ghr version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
}

func helpCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:               "help [command]",
		Short:             "Show help for a command",
		Args:              cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			target, _, _ := root.Find(args)
			if target == nil {
				target = root
			}
			target.Help()
		},
	}
}
