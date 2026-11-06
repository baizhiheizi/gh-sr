package runner

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

// referenceEnrichFromScopeRunners is the original implementation this file's
// production code replaced: it materialised the per-instance `name-N` keys and
// looked instances up by exact match. It is kept here as the executable
// specification of the lookup rule — the owner of instance `B-N` is the last
// RunnerConfig named `B` whose count reaches N.
//
// The production lookup avoids building those keys (one string allocation per
// instance on every enrich tick) by splitting the `-N` suffix off the
// instance name instead. This reference exists so that equivalence is tested
// rather than assumed.
func referenceEnrichFromScopeRunners(statuses []RunnerStatus, cfg *config.Config, scopeRunners map[scopeKey][]GitHubRunner) {
	rcByInstance := make(map[string]*config.RunnerConfig)
	for i := range cfg.Runners {
		rc := &cfg.Runners[i]
		count := rc.Count
		if count < 1 {
			count = 1
		}
		name := rc.Name
		for j := 1; j <= count; j++ {
			rcByInstance[name+"-"+strconv.Itoa(j)] = rc
		}
	}

	for i := range statuses {
		hcfg, ok := cfg.Hosts[statuses[i].Host]
		if !ok {
			continue
		}
		rc := rcByInstance[statuses[i].Instance]
		if rc == nil {
			continue
		}
		exp := expectedGitHubRunnerOS(hcfg.OS)
		key := scopeKey{rc.Scope(), rc.ScopeTarget()}
		for _, gr := range scopeRunners[key] {
			if gr.Name != statuses[i].Instance {
				continue
			}
			if exp != "" && gr.OS != "" && !strings.EqualFold(gr.OS, exp) {
				continue
			}
			statuses[i].Remote = gr.Status
			statuses[i].Busy = gr.Busy
			break
		}
	}
}

// TestEnrichFromScopeRunners_MatchesInstanceKeyMap asserts the base-name
// lookup resolves every status exactly like the original per-instance key
// map, over randomised configs that share base names across hosts, use base
// names containing dashes and digits, and carry instance names that are not
// valid `name-N` forms.
func TestEnrichFromScopeRunners_MatchesInstanceKeyMap(t *testing.T) {
	names := []string{"ci", "web", "my-runner", "web-2", "runner-0", "a-b-c", "x", "web"}
	hosts := []string{"h1", "h2", "h3"}
	repos := []string{"o/r1", "o/r2", "o/r3"}
	counts := []int{0, 1, 2, 3, 5}

	rng := rand.New(rand.NewSource(20240607))
	m := &Manager{}

	for iter := 0; iter < 300; iter++ {
		cfg := &config.Config{
			Hosts: map[string]config.HostConfig{
				"h1": {Addr: "a@b", OS: "linux", Arch: "amd64"},
				"h2": {Addr: "a@b", OS: "windows", Arch: "amd64"},
				"h3": {Addr: "a@b", OS: "linux", Arch: "arm64"},
			},
		}
		nRunners := 1 + rng.Intn(8)
		for i := 0; i < nRunners; i++ {
			rc := config.RunnerConfig{
				Name:  names[rng.Intn(len(names))],
				Repo:  repos[rng.Intn(len(repos))],
				Host:  hosts[rng.Intn(len(hosts))],
				Count: counts[rng.Intn(len(counts))],
			}
			if rng.Intn(4) == 0 {
				rc.Org = "acme"
				rc.Repo = ""
			}
			cfg.Runners = append(cfg.Runners, rc)
		}

		// GitHub-side runners: real instance names for the scopes the config
		// targets, plus decoys with similar names.
		scopeRunners := make(map[scopeKey][]GitHubRunner)
		for _, rc := range cfg.Runners {
			key := scopeKey{rc.Scope(), rc.ScopeTarget()}
			for _, inst := range rc.InstanceNames() {
				scopeRunners[key] = append(scopeRunners[key], GitHubRunner{
					Name:   inst,
					Status: []string{"online", "offline"}[rng.Intn(2)],
					Busy:   rng.Intn(2) == 0,
					OS:     []string{"Linux", "Windows", "macOS", ""}[rng.Intn(4)],
				})
			}
		}
		for _, key := range []scopeKey{{"repo", "o/r1"}, {"repo", "o/r2"}, {"repo", "o/r3"}, {"org", "acme"}} {
			for _, decoy := range []string{"web-foo", "web-02", "web-0", "web", "web-1-2", "ci-1-1"} {
				scopeRunners[key] = append(scopeRunners[key], GitHubRunner{
					Name:   decoy,
					Status: "online",
					OS:     "Linux",
				})
			}
		}

		// Statuses: one per configured instance, plus instance names that do
		// not follow the `name-N` convention at all.
		var statuses []RunnerStatus
		for _, rc := range cfg.Runners {
			for _, inst := range rc.InstanceNames() {
				statuses = append(statuses, RunnerStatus{Instance: inst, Host: rc.Host})
			}
		}
		for _, noise := range []string{"web-foo", "web-02", "web-0", "web", "web-", "-1", "ci-+1", "web-999999999999", "web-1-2", "nope"} {
			statuses = append(statuses, RunnerStatus{Instance: noise, Host: hosts[rng.Intn(len(hosts))]})
		}
		// A status for an unknown host must stay untouched.
		statuses = append(statuses, RunnerStatus{Instance: "ci-1", Host: "unknown-host"})

		want := make([]RunnerStatus, len(statuses))
		copy(want, statuses)
		referenceEnrichFromScopeRunners(want, cfg, scopeRunners)

		got := make([]RunnerStatus, len(statuses))
		copy(got, statuses)
		m.enrichFromScopeRunners(got, cfg, scopeRunners)

		for i := range want {
			if got[i].Remote != want[i].Remote || got[i].Busy != want[i].Busy {
				t.Fatalf("iter %d status %d (instance %q host %q): got Remote=%q Busy=%v, want Remote=%q Busy=%v",
					iter, i, statuses[i].Instance, statuses[i].Host, got[i].Remote, got[i].Busy, want[i].Remote, want[i].Busy)
			}
		}
	}
}

// TestEnrichFromScopeRunners_SharedBaseNameAcrossHosts pins the regression
// this lookup has to preserve: two runners may share a base name on different
// hosts (config validation only enforces instance-name uniqueness per host),
// and the instance `web-2` belongs to the runner whose count reaches 2 — not
// to whichever runner was configured last.
func TestEnrichFromScopeRunners_SharedBaseNameAcrossHosts(t *testing.T) {
	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "a@b", OS: "linux", Arch: "amd64"},
			"h2": {Addr: "a@b", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "web", Repo: "o/r1", Host: "h1", Count: 3},
			{Name: "web", Repo: "o/r2", Host: "h2", Count: 1},
		},
	}
	scopeRunners := map[scopeKey][]GitHubRunner{
		{"repo", "o/r1"}: {{Name: "web-1", Status: "online", OS: "Linux"}, {Name: "web-2", Status: "online", OS: "Linux"}, {Name: "web-3", Status: "online", OS: "Linux"}},
		{"repo", "o/r2"}: {{Name: "web-1", Status: "online", OS: "Linux"}},
	}
	statuses := []RunnerStatus{
		{Instance: "web-1", Host: "h1"},
		{Instance: "web-2", Host: "h1"},
		{Instance: "web-3", Host: "h1"},
		{Instance: "web-1", Host: "h2"},
	}

	m := &Manager{}
	m.enrichFromScopeRunners(statuses, cfg, scopeRunners)

	for i := range statuses {
		if statuses[i].Remote != "online" {
			t.Fatalf("status %d (instance %q host %q): Remote=%q, want %q", i, statuses[i].Instance, statuses[i].Host, statuses[i].Remote, "online")
		}
	}
}

// TestEnrichFromScopeRunners_SharedBaseNameResolvesToOwningScope asserts the
// owner of a shared instance name determines the scope that is searched, so
// a runner can only ever match the GitHub scope it was configured for.
func TestEnrichFromScopeRunners_SharedBaseNameResolvesToOwningScope(t *testing.T) {
	cfg := &config.Config{
		Hosts: map[string]config.HostConfig{
			"h1": {Addr: "a@b", OS: "linux", Arch: "amd64"},
		},
		Runners: []config.RunnerConfig{
			{Name: "web", Repo: "o/r1", Host: "h1", Count: 3},
			{Name: "web", Repo: "o/r2", Host: "h1", Count: 1},
		},
	}
	// `web-2` only exists in o/r1; o/r2 holds a runner with the same name so
	// a scope mix-up would be visible.
	scopeRunners := map[scopeKey][]GitHubRunner{
		{"repo", "o/r1"}: {{Name: "web-2", Status: "online", OS: "Linux"}},
		{"repo", "o/r2"}: {{Name: "web-2", Status: "offline", OS: "Linux"}},
	}
	statuses := []RunnerStatus{{Instance: "web-2", Host: "h1"}}

	m := &Manager{}
	m.enrichFromScopeRunners(statuses, cfg, scopeRunners)

	// The last config entry (o/r2) owns `web-1` but not `web-2`; `web-2`
	// stays with the o/r1 runner, whose count reaches 2.
	if statuses[0].Remote != "online" {
		t.Fatalf("Remote=%q, want %q (instance must resolve to the runner whose count covers it)", statuses[0].Remote, "online")
	}
}

func TestSplitInstanceIndex(t *testing.T) {
	cases := []struct {
		instance string
		base     string
		index    int
		ok       bool
	}{
		{"ci-1", "ci", 1, true},
		{"ci-12", "ci", 12, true},
		{"my-runner-3", "my-runner", 3, true},
		{"runner-0-10", "runner-0", 10, true},
		{"ci-0", "", 0, false},
		{"ci-02", "", 0, false},
		{"ci-+1", "", 0, false},
		{"ci-1 ", "", 0, false},
		{"ci-x", "", 0, false},
		{"ci-", "", 0, false},
		{"ci", "", 0, false},
		{"-1", "", 0, false},
		{"", "", 0, false},
		{"ci-999999999999", "", 0, false},
	}
	for _, tc := range cases {
		base, index, ok := splitInstanceIndex(tc.instance)
		if ok != tc.ok || base != tc.base || index != tc.index {
			t.Errorf("splitInstanceIndex(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.instance, base, index, ok, tc.base, tc.index, tc.ok)
		}
	}
}
