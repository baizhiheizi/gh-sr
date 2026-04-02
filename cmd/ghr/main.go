package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
	"github.com/an-lee/ghr/internal/runner"
	"github.com/an-lee/ghr/internal/tui"
)

var (
	cfgFile    string
	filterHost string
	filterRepo string
)

func main() {
	root := &cobra.Command{
		Use:   "ghr",
		Short: "Manage self-hosted GitHub Actions runners across multiple hosts",
		Long: `ghr manages self-hosted GitHub Actions runners on any combination
of Linux, macOS, and Windows hosts — all from your laptop over SSH.

Define your hosts and runners in config/runners.yml, then use unified
commands to setup, start, stop, and monitor everything.`,
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&cfgFile, "config", "c", config.DefaultPath(), "config file path")
	root.PersistentFlags().StringVar(&filterHost, "host", "", "filter by host name")
	root.PersistentFlags().StringVar(&filterRepo, "repo", "", "filter by repo (owner/repo)")

	root.AddCommand(
		setupCmd(),
		upCmd(),
		downCmd(),
		restartCmd(),
		statusCmd(),
		logsCmd(),
		cleanupCmd(),
		updateCmd(),
		configCmd(),
		dashboardCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	return config.Load(cfgFile)
}

func newManager(cfg *config.Config) *runner.Manager {
	return runner.NewManager(cfg.GitHub.PAT)
}

func connectHost(name string, cfg config.HostConfig) (*host.Host, error) {
	h := host.NewHost(name, cfg)
	if err := h.Connect(); err != nil {
		return nil, err
	}
	return h, nil
}

// --- Commands ---

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup [runner-names...]",
		Short: "Install runner prerequisites and configure runners on hosts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr := newManager(cfg)
			runners := config.FilterRunners(cfg, filterHost, filterRepo, args)

			hostsDone := map[string]bool{}
			for _, rc := range runners {
				hcfg := cfg.Hosts[rc.Host]
				if hostsDone[rc.Host] && rc.EffectiveMode(hcfg.OS) == "docker" {
					continue
				}

				fmt.Printf("Setting up on %s (%s)...\n", rc.Host, hcfg.Addr)
				h, err := connectHost(rc.Host, hcfg)
				if err != nil {
					return err
				}
				defer h.Close()

				if err := mgr.Setup(h, rc); err != nil {
					return err
				}
				hostsDone[rc.Host] = true
			}

			fmt.Println("\nSetup complete.")
			return nil
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
			mgr := newManager(cfg)
			runners := config.FilterRunners(cfg, filterHost, filterRepo, args)

			for _, rc := range runners {
				hcfg := cfg.Hosts[rc.Host]
				fmt.Printf("Starting %s on %s...\n", rc.Name, rc.Host)
				h, err := connectHost(rc.Host, hcfg)
				if err != nil {
					return err
				}
				defer h.Close()

				if err := mgr.Start(h, rc); err != nil {
					return err
				}
			}

			return nil
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
			mgr := newManager(cfg)
			runners := config.FilterRunners(cfg, filterHost, filterRepo, args)

			for _, rc := range runners {
				hcfg := cfg.Hosts[rc.Host]
				fmt.Printf("Stopping %s on %s...\n", rc.Name, rc.Host)
				h, err := connectHost(rc.Host, hcfg)
				if err != nil {
					return err
				}
				defer h.Close()

				if err := mgr.Stop(h, rc); err != nil {
					return err
				}
			}

			return nil
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
			mgr := newManager(cfg)
			runners := config.FilterRunners(cfg, filterHost, filterRepo, args)

			for _, rc := range runners {
				hcfg := cfg.Hosts[rc.Host]
				fmt.Printf("Restarting %s on %s...\n", rc.Name, rc.Host)
				h, err := connectHost(rc.Host, hcfg)
				if err != nil {
					return err
				}
				defer h.Close()

				_ = mgr.Stop(h, rc)
				if err := mgr.Start(h, rc); err != nil {
					return err
				}
			}

			return nil
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
			mgr := newManager(cfg)
			runners := config.FilterRunners(cfg, filterHost, filterRepo, args)

			var allStatuses []runner.RunnerStatus
			for _, rc := range runners {
				hcfg := cfg.Hosts[rc.Host]
				h, err := connectHost(rc.Host, hcfg)
				if err != nil {
					fmt.Printf("Warning: cannot connect to %s: %v\n", rc.Host, err)
					for _, name := range rc.InstanceNames() {
						allStatuses = append(allStatuses, runner.RunnerStatus{
							Instance: name,
							Host:     rc.Host,
							Repo:     rc.Repo,
							Mode:     rc.EffectiveMode(hcfg.OS),
							Local:    "unreachable",
						})
					}
					continue
				}
				defer h.Close()

				statuses, err := mgr.Status(h, rc)
				if err != nil {
					return err
				}
				allStatuses = append(allStatuses, statuses...)
			}

			mgr.EnrichWithGitHubStatus(allStatuses, cfg)
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

			target := args[0]
			rc, found := cfg.FindRunner(target)
			if !found {
				return fmt.Errorf("runner %q not found in config", target)
			}

			hcfg := cfg.Hosts[rc.Host]
			h, err := connectHost(rc.Host, hcfg)
			if err != nil {
				return err
			}
			defer h.Close()

			mgr := newManager(cfg)
			output, err := mgr.Logs(h, *rc, target)
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
			mgr := newManager(cfg)

			fmt.Println("Cleaning up offline runners...")
			removed, err := mgr.CleanupOffline(cfg)
			if err != nil {
				return err
			}
			if removed == 0 {
				fmt.Println("No offline runners found.")
			} else {
				fmt.Printf("Removed %d offline runner(s).\n", removed)
			}
			return nil
		},
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update [runner-names...]",
		Short: "Update runner binary on hosts (remove + setup + start)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mgr := newManager(cfg)
			runners := config.FilterRunners(cfg, filterHost, filterRepo, args)

			for _, rc := range runners {
				hcfg := cfg.Hosts[rc.Host]
				h, err := connectHost(rc.Host, hcfg)
				if err != nil {
					return err
				}
				defer h.Close()

				fmt.Printf("Updating %s on %s...\n", rc.Name, rc.Host)
				_ = mgr.Remove(h, rc)
				if err := mgr.Setup(h, rc); err != nil {
					return err
				}
				if err := mgr.Start(h, rc); err != nil {
					return err
				}
			}

			fmt.Println("\nUpdate complete.")
			return nil
		},
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print resolved configuration",
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

func dashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Launch interactive TUI dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			return tui.RunDashboard(cfg)
		},
	}
}
