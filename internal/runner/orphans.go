package runner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// OrphanCleanupPlan describes autostart and/or directory cleanup for one instance.
type OrphanCleanupPlan struct {
	Instance  string
	Autostart bool
	Directory bool
}

// ConfiguredInstanceSet returns instance names from runners assigned to hostName.
func ConfiguredInstanceSet(runners []config.RunnerConfig, hostName string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, rc := range runners {
		if rc.Host != hostName {
			continue
		}
		for _, inst := range rc.InstanceNames() {
			set[inst] = struct{}{}
		}
	}
	return set
}

// OrphanInstances returns runner instance names on h that are not in configured
// (from runners.yml) but have a gh-sr autostart unit and/or directory under ~/.gh-sr/runners.
func OrphanInstances(h *host.Host, configured map[string]struct{}) ([]string, error) {
	dirs, err := ListRunnerInstanceDirs(h)
	if err != nil {
		return nil, err
	}
	installed, err := autostart.ListInstalled(h)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var orphans []string
	add := func(inst string) {
		if _, ok := configured[inst]; ok {
			return
		}
		if err := SafeRunnerInstanceName(inst); err != nil {
			return
		}
		if _, ok := seen[inst]; ok {
			return
		}
		seen[inst] = struct{}{}
		orphans = append(orphans, inst)
	}
	for _, inst := range dirs {
		add(inst)
	}
	for _, inst := range installed {
		add(inst)
	}
	sort.Strings(orphans)
	return orphans, nil
}

// PlanOrphanCleanup reports what would be removed for an orphan instance.
func (m *Manager) PlanOrphanCleanup(h *host.Host, instance string) (OrphanCleanupPlan, error) {
	plan := OrphanCleanupPlan{Instance: instance}
	if err := SafeRunnerInstanceName(instance); err != nil {
		return plan, err
	}
	kind, err := autostart.Detect(h, instance)
	if err != nil {
		return plan, err
	}
	plan.Autostart = kind != autostart.KindNone
	if h.OS == "linux" && svcShPresent(h, instance) {
		plan.Autostart = true
	}
	exists, err := instanceDirectoryExists(h, instance)
	if err != nil {
		return plan, err
	}
	plan.Directory = exists
	return plan, nil
}

// CleanupOrphanInstance removes gh-sr autostart and/or the runner directory for an instance
// that is not in runners.yml. When dryRun is true, nothing is removed.
func (m *Manager) CleanupOrphanInstance(h *host.Host, instance string, dryRun bool) (OrphanCleanupPlan, error) {
	plan, err := m.PlanOrphanCleanup(h, instance)
	if err != nil {
		return plan, err
	}
	if !plan.Autostart && !plan.Directory {
		return plan, nil
	}
	if dryRun {
		if plan.Autostart {
			fmt.Fprintf(m.out(), "  %s: would remove autostart\n", instance)
		}
		if plan.Directory {
			fmt.Fprintf(m.out(), "  %s: would remove orphan directory\n", instance)
		}
		return plan, nil
	}

	if plan.Autostart {
		m.removeNativeServices(h, instance)
	}
	if plan.Directory {
		if err := removeDirTree(h, instance); err != nil {
			return plan, fmt.Errorf("removing orphan directory: %w", err)
		}
		fmt.Fprintf(m.out(), "  %s: orphan directory removed\n", instance)
	}
	return plan, nil
}

func instanceDirectoryExists(h *host.Host, instance string) (bool, error) {
	dir := h.RunnerDir(instance)
	if h.OS == "windows" {
		out, err := h.RunShell(fmt.Sprintf("if (Test-Path -LiteralPath %s -PathType Container) { 'yes' } else { 'no' }", h.RunnerDirPS(instance)))
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(out) == "yes", nil
	}
	out, err := h.Run(fmt.Sprintf(`test -d %s && echo yes || echo no`, dir))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}
