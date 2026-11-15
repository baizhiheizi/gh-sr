package runner

import (
	"sort"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// HostInstance is one runner instance present on a host: either configured
// (a runners.yml row expanded into instance names, Config pointing at the
// owning row) or orphan (a directory under ~/.gh-sr/runners no configured
// row claims, Config nil).
type HostInstance struct {
	Name   string
	Config *config.RunnerConfig
}

// HostInventory is the set of runner instances on one host — the configured
// instance names merged with the on-disk directories — plus the dir-listing
// error, surfaced separately so callers pick their own failure policy.
type HostInventory struct {
	// Instances holds every configured and (when includeOnDisk) on-disk
	// instance, sorted by name. Orphans carry Config == nil.
	Instances []HostInstance
	// ListErr is the ListRunnerInstanceDirs error, if any. The configured
	// half of Instances is still populated; callers choose between failing
	// the operation (ops disk/prune) and degrading to configured-only rows
	// (doctor).
	ListErr error
}

// HostInventoryFor answers "what runner instances exist on hostName" — the
// question doctor's disk rows, ops CollectDiskUsage, and ops PruneDisk each
// used to hand-roll (configured map ∪ ListRunnerInstanceDirs merge, with
// instance → owning RunnerConfig), in one place so the three answers cannot
// drift. includeOnDisk adds the SSH dir listing and classifies on-disk
// directories no configured row claims as orphans; without it the inventory
// is the configured set alone (prune-without-orphans must not pay a round
// trip it will not use).
func HostInventoryFor(h *host.Host, runners []config.RunnerConfig, hostName string, includeOnDisk bool) HostInventory {
	inv := HostInventory{}
	seen := make(map[string]struct{})
	for i := range runners {
		rc := &runners[i]
		if rc.Host != hostName {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			if _, ok := seen[inst]; ok {
				continue
			}
			seen[inst] = struct{}{}
			inv.Instances = append(inv.Instances, HostInstance{Name: inst, Config: rc})
		}
	}
	if !includeOnDisk {
		sortHostInstances(inv.Instances)
		return inv
	}
	dirs, err := ListRunnerInstanceDirs(h)
	if err != nil {
		// The configured half is still authoritative; the caller decides
		// whether a missing dir listing is fatal.
		inv.ListErr = err
		sortHostInstances(inv.Instances)
		return inv
	}
	for _, inst := range dirs {
		if _, ok := seen[inst]; ok {
			continue
		}
		seen[inst] = struct{}{}
		inv.Instances = append(inv.Instances, HostInstance{Name: inst})
	}
	sortHostInstances(inv.Instances)
	return inv
}

// Names returns the instance names in inventory order (sorted).
func (inv HostInventory) Names() []string {
	out := make([]string, len(inv.Instances))
	for i, in := range inv.Instances {
		out[i] = in.Name
	}
	return out
}

// ConfigByInstance maps instance name → owning config; orphans are absent,
// so a plain map lookup yields nil — the same shape MeasureDiskUsageBatch
// and PruneInstance consume, where nil config means orphan.
func (inv HostInventory) ConfigByInstance() map[string]*config.RunnerConfig {
	out := make(map[string]*config.RunnerConfig, len(inv.Instances))
	for _, in := range inv.Instances {
		if in.Config != nil {
			out[in.Name] = in.Config
		}
	}
	return out
}

func sortHostInstances(instances []HostInstance) {
	sort.Slice(instances, func(i, j int) bool { return instances[i].Name < instances[j].Name })
}
