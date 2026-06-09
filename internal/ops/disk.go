package ops

import (
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

// CollectDiskUsage gathers disk usage for configured and orphan runner directories.
func CollectDiskUsage(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) ([]runner.DiskUsageEntry, error) {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return nil, err
	}

	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	statuses, err := CollectStatus(w, cfg, mgr, filterHost, filterRepo, nameArgs)
	if err != nil {
		return nil, err
	}
	busyByInstance := map[string]bool{}
	remoteByInstance := map[string]string{}
	for _, s := range statuses {
		busyByInstance[s.Instance] = s.Busy
		remoteByInstance[s.Instance] = s.Remote
	}

	configuredByHost := configuredInstancesByHost(cfg, runners)

	type hostResult struct {
		entries []runner.DiskUsageEntry
		err     error
	}

	hostNames := uniqueHostNamesFromRunners(runners)
	if len(hostNames) == 0 {
		return nil, nil
	}

	results := make([]hostResult, len(hostNames))
	var wg sync.WaitGroup
	var wMu sync.Mutex

	for i, hostName := range hostNames {
		wg.Add(1)
		go func(i int, hostName string) {
			defer wg.Done()
			hcfg := cfg.Hosts[hostName]
			h, err := ConnectHost(hostName, hcfg)
			if err != nil {
				if w != nil {
					wMu.Lock()
					fmt.Fprintf(w, "Warning: cannot connect to %s: %v\n", hostName, err)
					wMu.Unlock()
				}
				return
			}
			defer h.Close()

			seen := make(map[string]struct{})
			configured := configuredByHost[hostName]
			rcByInstance := rcByInstanceForHost(runners, hostName)

			for inst := range configured {
				seen[inst] = struct{}{}
				rc := rcByInstance[inst]
				entry := runner.MeasureDiskUsage(h, hostName, inst, rc)
				entry.Busy = busyByInstance[inst]
				entry.Remote = remoteByInstance[inst]
				results[i].entries = append(results[i].entries, entry)
			}

			diskDirs, listErr := runner.ListRunnerInstanceDirs(h)
			if listErr != nil {
				results[i].err = listErr
				return
			}
			for _, inst := range diskDirs {
				if _, ok := seen[inst]; ok {
					continue
				}
				entry := runner.MeasureDiskUsage(h, hostName, inst, nil)
				entry.Orphan = true
				results[i].entries = append(results[i].entries, entry)
			}
		}(i, hostName)
	}
	wg.Wait()

	var all []runner.DiskUsageEntry
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		all = append(all, r.entries...)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Host != all[j].Host {
			return all[i].Host < all[j].Host
		}
		return all[i].Instance < all[j].Instance
	})
	return all, nil
}

// PruneDisk reclaims disk space on idle runner instances.
func PruneDisk(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string, opts runner.PruneOptions) ([]runner.PruneResult, error) {
	if err := ResolveHostInfo(w, cfg); err != nil {
		return nil, err
	}

	runners := config.FilterRunners(cfg, filterHost, filterRepo, nameArgs)
	statuses, err := CollectStatus(w, cfg, mgr, filterHost, filterRepo, nameArgs)
	if err != nil && !opts.Force {
		return nil, err
	}

	busyByInstance := map[string]bool{}
	githubKnown := map[string]bool{}
	for _, s := range statuses {
		busyByInstance[s.Instance] = s.Busy
		if s.Remote != "" {
			githubKnown[s.Instance] = true
		}
	}

	configuredByHost := configuredInstancesByHost(cfg, runners)

	type hostResult struct {
		results []runner.PruneResult
		err     error
	}

	hostNames := uniqueHostNamesFromRunners(runners)
	if len(hostNames) == 0 {
		return nil, fmt.Errorf("no runners matching the given filters")
	}

	out := make([]hostResult, len(hostNames))
	var wg sync.WaitGroup
	var wMu sync.Mutex

	for i, hostName := range hostNames {
		wg.Add(1)
		go func(i int, hostName string) {
			defer wg.Done()
			hcfg := cfg.Hosts[hostName]
			h, err := ConnectHost(hostName, hcfg)
			if err != nil {
				if w != nil {
					wMu.Lock()
					fmt.Fprintf(w, "Warning: cannot connect to %s: %v\n", hostName, err)
					wMu.Unlock()
				}
				return
			}
			defer h.Close()

			configured := configuredByHost[hostName]
			rcByInstance := rcByInstanceForHost(runners, hostName)

			var targets []string
			for inst := range configured {
				targets = append(targets, inst)
			}
			if opts.IncludeOrphans {
				diskDirs, listErr := runner.ListRunnerInstanceDirs(h)
				if listErr != nil {
					out[i].err = listErr
					return
				}
				for _, inst := range diskDirs {
					if _, ok := configured[inst]; !ok {
						targets = append(targets, inst)
					}
				}
			}
			sort.Strings(targets)

			for _, inst := range targets {
				rc := rcByInstance[inst]
				busy := busyByInstance[inst]
				if rc != nil && !githubKnown[inst] && !opts.Force {
					out[i].results = append(out[i].results, runner.PruneResult{
						Instance: inst,
						Host:     hostName,
						Skipped:  true,
						Reason:   "GitHub status unknown (use --force)",
					})
					if w != nil {
						wMu.Lock()
						printPruneResult(w, out[i].results[len(out[i].results)-1], opts.DryRun)
						wMu.Unlock()
					}
					continue
				}
				res := mgr.PruneInstance(h, hostName, inst, rc, busy, opts)
				out[i].results = append(out[i].results, res)
				if w != nil {
					wMu.Lock()
					printPruneResult(w, res, opts.DryRun)
					wMu.Unlock()
				}
			}
		}(i, hostName)
	}
	wg.Wait()

	var all []runner.PruneResult
	for _, r := range out {
		if r.err != nil {
			return nil, r.err
		}
		all = append(all, r.results...)
	}
	return all, nil
}

func printPruneResult(w io.Writer, res runner.PruneResult, dryRun bool) {
	prefix := "  "
	if dryRun {
		prefix = "  [dry-run] "
	}
	if res.Err != nil {
		fmt.Fprintf(w, "%s%s on %s: error: %v\n", prefix, res.Instance, res.Host, res.Err)
		return
	}
	if res.Skipped {
		fmt.Fprintf(w, "%s%s on %s: skipped (%s)\n", prefix, res.Instance, res.Host, res.Reason)
		return
	}
	for _, a := range res.Actions {
		fmt.Fprintf(w, "%s%s on %s: %s\n", prefix, res.Instance, res.Host, a)
	}
}

func configuredInstancesByHost(cfg *config.Config, runners []config.RunnerConfig) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{})
	for _, rc := range runners {
		if out[rc.Host] == nil {
			out[rc.Host] = make(map[string]struct{})
		}
		for _, inst := range rc.InstanceNames() {
			out[rc.Host][inst] = struct{}{}
		}
	}
	return out
}

func rcByInstanceForHost(runners []config.RunnerConfig, hostName string) map[string]*config.RunnerConfig {
	out := make(map[string]*config.RunnerConfig)
	for i := range runners {
		rc := &runners[i]
		if rc.Host != hostName {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			out[inst] = rc
		}
	}
	return out
}

func uniqueHostNamesFromRunners(runners []config.RunnerConfig) []string {
	seen := make(map[string]struct{})
	for _, rc := range runners {
		seen[rc.Host] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// PrintDiskUsageTable prints disk usage entries to w.
func PrintDiskUsageTable(w io.Writer, entries []runner.DiskUsageEntry) {
	if len(entries) == 0 {
		fmt.Fprintln(w, "No runner directories found.")
		return
	}

	headers := []string{"HOST", "INSTANCE", "MODE", "TOTAL", "WORK", "TEMP", "DOCKER-DATA", "OTHER", "BUSY", "ORPHAN"}
	rows := make([][]string, len(entries))
	var totalBytes int64
	for i, e := range entries {
		if e.Err != nil {
			rows[i] = []string{e.Host, e.Instance, e.Mode, "error", e.Err.Error(), "", "", "", "", ""}
			continue
		}
		totalBytes += e.TotalBytes
		busy := "-"
		if e.Busy {
			busy = "yes"
		} else if e.Remote == "online" {
			busy = "no"
		}
		orphan := "no"
		if e.Orphan {
			orphan = "yes"
		}
		rows[i] = []string{
			e.Host,
			e.Instance,
			e.Mode,
			runner.FormatBytesHuman(e.TotalBytes),
			runner.FormatBytesHuman(e.WorkBytes),
			runner.FormatBytesHuman(e.TempBytes),
			runner.FormatBytesHuman(e.DockerDataBytes),
			runner.FormatBytesHuman(e.OtherBytes),
			busy,
			orphan,
		}
	}

	widths := columnWidths(headers, rows)
	printRow(w, headers, widths, true)
	for _, row := range rows {
		printRow(w, row, widths, false)
	}
	fmt.Fprintf(w, "\nTotal: %s across %d instance(s)\n", runner.FormatBytesHuman(totalBytes), len(entries))
}

func columnWidths(headers []string, rows [][]string) []int {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for j, cell := range row {
			if j < len(widths) && len(cell) > widths[j] {
				widths[j] = len(cell)
			}
		}
	}
	return widths
}

func printRow(w io.Writer, cells []string, widths []int, header bool) {
	for i, cell := range cells {
		if i >= len(widths) {
			break
		}
		if header {
			fmt.Fprintf(w, "%-*s  ", widths[i], cell)
		} else {
			fmt.Fprintf(w, "%-*s  ", widths[i], cell)
		}
	}
	fmt.Fprintln(w)
}
