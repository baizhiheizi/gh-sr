package ops

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/table"
)

// skippedHosts returns the names of hosts fanOutHosts could not connect to,
// sorted for deterministic error messages.
func skippedHosts[T any](results []fanoutResult[T]) []string {
	var skipped []string
	for _, r := range results {
		if r.ConnectErr != nil {
			skipped = append(skipped, r.Name)
		}
	}
	sort.Strings(skipped)
	return skipped
}

// connectError aggregates unreachable host names into the error surfaced by
// the disk orchestrators.
func connectError(skipped []string) error {
	return fmt.Errorf("cannot connect to host(s): %s", strings.Join(skipped, ", "))
}

// DiskPruneOptions configures PruneDisk.
type DiskPruneOptions struct {
	DryRun         bool
	PruneCache     bool
	IncludeOrphans bool
	// Force prunes configured instances when GitHub runner status is unknown.
	Force bool
}

func diskHostInstanceKey(hostName, instance string) string {
	return hostName + "\x00" + instance
}

type diskStatusMaps struct {
	busy        map[string]bool
	remote      map[string]string
	githubKnown map[string]bool
}

func diskStatusMapsFrom(statuses []runner.RunnerStatus) diskStatusMaps {
	m := diskStatusMaps{
		busy:        make(map[string]bool),
		remote:      make(map[string]string),
		githubKnown: make(map[string]bool),
	}
	for _, s := range statuses {
		key := diskHostInstanceKey(s.Host, s.Instance)
		m.busy[key] = s.Busy
		m.remote[key] = s.Remote
		if s.Remote != "" {
			m.githubKnown[key] = true
		}
	}
	return m
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

func configuredInstancesOnHost(runners []config.RunnerConfig) map[string]struct{} {
	out := make(map[string]struct{})
	for _, rc := range runners {
		for _, inst := range rc.InstanceNames() {
			out[inst] = struct{}{}
		}
	}
	return out
}

// CollectDiskUsage gathers disk usage for configured and orphan runner directories.
func CollectDiskUsage(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string) ([]runner.DiskUsageEntry, error) {
	runners, err := resolveAndFilter(w, cfg, filterHost, filterRepo, nameArgs)
	if err != nil {
		return nil, err
	}
	if len(runners) == 0 {
		return nil, fmt.Errorf("no runners matching the given filters")
	}

	statuses, err := CollectStatus(w, cfg, mgr, filterHost, filterRepo, nameArgs)
	if err != nil {
		return nil, err
	}
	statusMaps := diskStatusMapsFrom(statuses)
	groups := groupRunnersByHost(runners)

	results := fanOutHosts(w, cfg, groups, func(w io.Writer, h *host.Host, g hostGroup) ([]runner.DiskUsageEntry, error) {
		hcfg := cfg.Hosts[g.name]
		seen := make(map[string]struct{})
		configured := configuredInstancesOnHost(g.runners)
		rcByInstance := rcByInstanceForHost(g.runners, g.name)

		// Build the full set of instance names to measure (configured +
		// orphan). One batched SSH round-trip per host — replaces the
		// 1 ListRunnerInstanceDirs + len(instances) MeasureDiskUsage
		// pattern that previously paid N+1 SSH round-trips per host.
		instances := make([]string, 0, len(rcByInstance)+4)
		for inst := range configured {
			seen[inst] = struct{}{}
			instances = append(instances, inst)
		}

		diskDirs, err := runner.ListRunnerInstanceDirs(h)
		if err != nil {
			return nil, err
		}
		for _, inst := range diskDirs {
			if _, ok := seen[inst]; ok {
				continue
			}
			seen[inst] = struct{}{}
			instances = append(instances, inst)
		}

		entries := runner.MeasureDiskUsageBatch(h, g.name, instances, rcByInstance)
		var out []runner.DiskUsageEntry
		for _, inst := range instances {
			entry := entries[inst]
			key := diskHostInstanceKey(g.name, inst)
			entry.Busy = statusMaps.busy[key]
			entry.Remote = statusMaps.remote[key]
			out = append(out, entry)
		}

		// Host-level entry for the local cache server storage (Linux only —
		// the cache container is only deployed there).
		if s := cacheSettings(cfg); s != nil && hcfg.OS == "linux" {
			path, bytes, mErr := cache.MeasureStorage(h, *s)
			out = append(out, runner.DiskUsageEntry{
				Host:       g.name,
				Instance:   cache.ContainerName,
				Path:       path,
				Mode:       "cache",
				TotalBytes: bytes,
				Err:        mErr,
			})
		}
		return out, nil
	})

	if w != nil {
		// Historical contract: the skip list was only populated when a writer
		// was attached, so nil-writer callers get partial results instead of
		// an aggregated connect error. Preserve that gate.
		if skipped := skippedHosts(results); len(skipped) > 0 {
			return nil, connectError(skipped)
		}
	}

	var all []runner.DiskUsageEntry
	for _, r := range results {
		if r.Err != nil {
			return nil, r.Err
		}
		all = append(all, r.Val...)
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
func PruneDisk(w io.Writer, cfg *config.Config, mgr *runner.Manager, filterHost, filterRepo string, nameArgs []string, opts DiskPruneOptions) ([]runner.PruneResult, error) {
	runners, err := resolveAndFilter(w, cfg, filterHost, filterRepo, nameArgs)
	if err != nil {
		return nil, err
	}
	if len(runners) == 0 {
		return nil, fmt.Errorf("no runners matching the given filters")
	}

	statuses, err := CollectStatus(w, cfg, mgr, filterHost, filterRepo, nameArgs)
	if err != nil {
		return nil, err
	}
	statusMaps := diskStatusMapsFrom(statuses)
	groups := groupRunnersByHost(runners)

	runnerOpts := runner.PruneOptions{
		DryRun:         opts.DryRun,
		PruneCache:     opts.PruneCache,
		IncludeOrphans: opts.IncludeOrphans,
	}

	results := fanOutHosts(w, cfg, groups, func(w io.Writer, h *host.Host, g hostGroup) ([]runner.PruneResult, error) {
		configured := configuredInstancesOnHost(g.runners)
		rcByInstance := rcByInstanceForHost(g.runners, g.name)

		var targets []string
		for inst := range configured {
			targets = append(targets, inst)
		}
		if opts.IncludeOrphans {
			diskDirs, err := runner.ListRunnerInstanceDirs(h)
			if err != nil {
				return nil, err
			}
			for _, inst := range diskDirs {
				if _, ok := configured[inst]; !ok {
					targets = append(targets, inst)
				}
			}
		}
		sort.Strings(targets)

		var out []runner.PruneResult
		for _, inst := range targets {
			rc := rcByInstance[inst]
			key := diskHostInstanceKey(g.name, inst)
			busy := statusMaps.busy[key]
			if rc != nil && !statusMaps.githubKnown[key] && !opts.Force {
				res := runner.PruneResult{
					Instance: inst,
					Host:     g.name,
					Skipped:  true,
					Reason:   "GitHub status unknown (use --force)",
				}
				out = append(out, res)
				printPruneResult(w, res, opts.DryRun)
				continue
			}
			res := mgr.PruneInstance(h, g.name, inst, rc, busy, runnerOpts)
			out = append(out, res)
			printPruneResult(w, res, opts.DryRun)
		}
		return out, nil
	})

	if w != nil {
		// See CollectDiskUsage: nil-writer callers skip the aggregated
		// connect error (pre-existing contract).
		if skipped := skippedHosts(results); len(skipped) > 0 {
			return nil, connectError(skipped)
		}
	}

	var all []runner.PruneResult
	for _, r := range results {
		if r.Err != nil {
			return nil, r.Err
		}
		all = append(all, r.Val...)
	}
	if err := pruneResultsError(all); err != nil {
		return all, err
	}
	return all, nil
}

func pruneResultsError(results []runner.PruneResult) error {
	var parts []string
	for _, r := range results {
		if r.Err != nil {
			parts = append(parts, fmt.Sprintf("%s on %s: %v", r.Instance, r.Host, r.Err))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("disk prune failed for %d instance(s): %s", len(parts), strings.Join(parts, "; "))
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

// PrintDiskUsageTable prints disk usage entries to w.
func PrintDiskUsageTable(w io.Writer, entries []runner.DiskUsageEntry) {
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

	if !table.PrintPlain(w, table.Options{
		EmptyMsg: "No runner directories found.",
		Headers:  headers,
		Rows:     rows,
	}) {
		return
	}
	fmt.Fprintf(w, "\nTotal: %s across %d instance(s)\n", runner.FormatBytesHuman(totalBytes), len(entries))
}
