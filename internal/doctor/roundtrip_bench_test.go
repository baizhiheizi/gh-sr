package doctor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/registry"
	"github.com/an-lee/gh-sr/internal/sharedcache"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// The benchmark in this file measures the SSH round trips one full doctor
// host pass costs. Doctor is the heaviest unmeasured remote path in gh-sr:
// unlike Status (one batch per host) it walks a dozen checks, most of which
// still pay one remote invocation per probe, so the per-host total is the
// network/energy cost operators feel on every `gh sr doctor` run.
//
// Scope: what is measured is the runHostChecks host-check pass. It excludes
// the pre-pass OS detection that ensureDoctorHostOS runs before the checks
// (host.DetectOS issues one `uname -s` when runners.yml omits `os:` for a
// remote host). The fixture pins OS on the host config, so that round trip
// is not in the numbers below — a remote host configured without `os:` costs
// one round trip more than the counts here (29 and 10 instead of 28 and 9).
// The counts therefore measure the checks themselves, the part the folds
// target, and are not a claim about the whole `gh sr doctor` cost for a host.
//
// The mock below answers every command the healthy path issues with the
// reply its parser consumes — one-shot scripts get their tagged sections,
// batch scripts get one row per instance, plain probes get their OK token.
// The companion test pins the exact counts and asserts every check reached
// its healthy OK line (no short-circuit from a mock gap), which is what
// makes the counts meaningful: a check that bailed early would print a WARN
// and drop its round trips from the total.
//
// The auto-bind settings are deliberate: cache.bind_addr and
// registry_mirror.bind_addr unset means dockerutil.EffectiveBind and both
// RunnerURL calls resolve the docker0 gateway over SSH — the shape of the
// default runners.yml, and the shape future folds (resolving the gateway
// once per pass) must beat.

// doctorHealthyBatchReply renders the registry mirror one-shot batch answer
// for one healthy mirror.
func doctorHealthyBatchReply() string {
	return "@@m:1\n" +
		"@@st:1|running|registry:2\n" +
		"@@health:1|ok\n" +
		"@@du:1|1048576\n"
}

// doctorCacheOneShotReply renders the cache Inspect one-shot answer for a
// healthy deployed cache: running container, home, gateway, healthy probe,
// storage bytes.
func doctorCacheOneShotReply() string {
	return "S:running|ghcr.io/example/gh-sr-cache:1\n" +
		"H:/home/runner\n" +
		"G:" + gatewayIP("172.17.0.1") +
		"K:1\n" +
		"D:1048576\n"
}

// doctorDockerChainReply renders the docker CLI → daemon → privileged chain
// answer with all three probes passing.
func doctorDockerChainReply() string {
	return "#docker-cli:0\n#docker-daemon:0\n#docker-privileged:0\n"
}

// doctorAgenticInnerReply renders the agentic inner-hygiene batch answer for
// one running instance: no orphans, no networks, every fanout probe clean.
// cacheEnabled adds the cache-env fanout tag the OK wording reflects.
func doctorAgenticInnerReply(cacheEnabled bool) string {
	fanout := "#container-inner-host-docker-internal:0\n" +
		"#container-node-npm:0\n" +
		"#container-docker-socket-user:0\n" +
		"#container-zstd:0"
	if cacheEnabled {
		fanout += "\n#container-cache-env:0"
	}
	sec := "@@begin:1\n" +
		"@@state:1|running\n" +
		"@@hyg:1|0\n@@hygend:1|0\n" +
		"@@hyg:1|1\n@@hygend:1|1\n" +
		fanout + "\n" +
		"@@end:1"
	return sec
}

// batchScriptInstances extracts the instance names a batch script walks:
// the word list between `for <var> in` and the closing `; do`. Both batch
// shapes (container probe `for i in`, disk sizes `for inst in`) embed the
// names verbatim, so the mock answers exactly the rows the script asks for
// and stays correct regardless of scenario instance count.
func batchScriptInstances(cmd, forToken string) []string {
	start := strings.Index(cmd, forToken)
	if start < 0 {
		return nil
	}
	rest := cmd[start+len(forToken):]
	end := strings.Index(rest, "; do")
	if end < 0 {
		return nil
	}
	fields := strings.Fields(rest[:end])
	// Both builders quote each word (quotePOSIXWord / strconv.Quote), so a
	// plain name arrives as `"c-1"` — strip the wrapper back off.
	for i, f := range fields {
		fields[i] = strings.Trim(f, `"`)
	}
	return fields
}

// doctorRunFn answers every remote command one full healthy doctor host
// pass issues for a host with the given configured instance names (the
// on-disk and autostart listings must mirror runners.yml or the orphan
// check warns). Route order matters: the tagged batch scripts
// (@@begin / @@m) also contain substrings of the plain-probe routes, and
// the cache one-shot embeds the bare gateway probe, so scripts route before
// the probes they embed.
func doctorRunFn(instances []string) func(cmd string) (string, error) {
	onDisk := strings.Join(instances, "\n")
	return func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@begin:"):
			return doctorAgenticInnerReply(true), nil
		case strings.Contains(cmd, "@@m:"):
			return doctorHealthyBatchReply(), nil
		case strings.Contains(cmd, "{{.State.Status}}|{{.Config.Image}}"):
			return doctorCacheOneShotReply(), nil
		case strings.Contains(cmd, "# ghsr-batch-disk-v1"):
			var b strings.Builder
			for _, inst := range batchScriptInstances(cmd, "for inst in") {
				fmt.Fprintf(&b, "%s\t4096\t1024\t512\t2048\n", inst)
			}
			return b.String(), nil
		case strings.Contains(cmd, "for i in"):
			var b strings.Builder
			for _, inst := range batchScriptInstances(cmd, "for i in") {
				fmt.Fprintf(&b, "running|yes|yes|no|%s\n", inst)
			}
			return b.String(), nil
		case strings.Contains(cmd, "#docker-privileged:"):
			return doctorDockerChainReply(), nil
		case strings.Contains(cmd, "ip -o route get 1.1.1.1"):
			return "1500\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gatewayIP("172.17.0.1"), nil
		case strings.Contains(cmd, "getent hosts api.github.com"):
			return "ok", nil
		case strings.Contains(cmd, "curl -fsS"):
			// probeFromRunnerContainer: the cache /health probe answers
			// "healthy", the /v2/ mirror probes answer the HTTP code "200".
			if strings.Contains(cmd, "/v2/") {
				return "200", nil
			}
			return "healthy", nil
		case strings.Contains(cmd, "command -v curl"):
			return "ok", nil
		case strings.Contains(cmd, "sudo -n true"):
			return "ok", nil
		case strings.Contains(cmd, "id -u"):
			return "1000", nil
		case strings.Contains(cmd, "grep -c '^GH_SR_NPM_REGISTRY='"):
			return "1", nil
		case strings.Contains(cmd, "grep -c '^GH_SR_REGISTRY_MIRROR='"):
			return "1", nil
		case strings.Contains(cmd, "range .Mounts"):
			return "gh-sr-npm-cache /home/runner/.npm\n", nil
		case strings.Contains(cmd, "docker ps -a --format '{{.Names}}'"):
			return "gh-sr-registry\n" + onDisk + "\n", nil
		case strings.Contains(cmd, "ghsr-runner-"):
			return onDisk + "\n", nil
		case strings.Contains(cmd, `ls -1 "$HOME/.gh-sr/runners"`):
			return onDisk + "\n", nil
		case strings.Contains(cmd, "echo yes || echo no"):
			return "yes", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/home/runner\n", nil
		}
		return "", nil
	}
}

// doctorRoundTripScenario is one runHostChecks configuration under
// measurement.
type doctorRoundTripScenario struct {
	runners  []config.RunnerConfig
	cacheSet *cache.Settings
	mirror   *registry.Settings
	extras   containerExtras
}

// runDoctorRoundTrips drives one full host pass through runHostChecks and
// returns the round-trip count, the rendered output, and the result
// counters.
func runDoctorRoundTrips(sc doctorRoundTripScenario) (int, string, Result) {
	var instances []string
	for i := range sc.runners {
		if sc.runners[i].Host == "h" {
			instances = append(instances, sc.runners[i].InstanceNames()...)
		}
	}
	mock := &testutil.RoundTripCounter{RunFn: doctorRunFn(instances)}
	h := host.NewHost("h", config.HostConfig{OS: "linux", Arch: "amd64", Addr: "local"})
	h.SetConn(mock)
	var buf strings.Builder
	var res Result
	runHostChecks(&buf, "h", h, sc.runners, &res, sc.cacheSet, sc.cacheSet != nil, sc.mirror, sc.extras)
	return mock.RoundTrips, buf.String(), res
}

// mixedDoctorScenario is the busy-host shape: native + container + agentic
// runners, cache and mirror on the default auto-bind path, npm-registry and
// shared-cache extras configured.
func mixedDoctorScenario() doctorRoundTripScenario {
	return doctorRoundTripScenario{
		runners: []config.RunnerConfig{
			{Name: "n", Host: "h", Repo: "o/r", Count: 2},
			{Name: "c", Host: "h", Repo: "o/r", Count: 2, RunnerMode: config.RunnerModeContainer},
			{Name: "aw", Host: "h", Repo: "o/r", Count: 1, Profile: "agentic"},
		},
		cacheSet: &cache.Settings{Enabled: true},
		mirror: &registry.Settings{
			Enabled: true,
			Mirrors: []registry.MirrorSettings{{Name: "dockerhub", Port: 5577}},
		},
		extras: containerExtras{
			NPMRegistry: "http://registry.example",
			SharedCaches: []sharedcache.Cache{
				{Key: "npm", Volume: "gh-sr-npm-cache", Mount: "/home/runner/.npm"},
			},
		},
	}
}

// nativeOnlyDoctorScenario is the quiet-host shape: one native runner, no
// container services, so only the host-level checks run.
func nativeOnlyDoctorScenario() doctorRoundTripScenario {
	return doctorRoundTripScenario{
		runners: []config.RunnerConfig{
			{Name: "n", Host: "h", Repo: "o/r", Count: 1},
		},
	}
}

// BenchmarkDoctorHostRoundTripsMixed5 measures one full doctor host pass on
// the busy-host shape (2 native + 2 container + 1 agentic instance, cache +
// mirror + extras on the auto-bind path): 28 round trips today, every one a
// network hop plus a remote shell start. The companion test pins the count
// and the healthy-output condition.
//
// The count covers the runHostChecks pass only — see the scope note at the
// top of this file for the pre-pass OS detection it excludes.
func BenchmarkDoctorHostRoundTripsMixed5(b *testing.B) {
	sc := mixedDoctorScenario()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt, _, res := runDoctorRoundTrips(sc)
		if res.Warn != 0 || res.Fail != 0 {
			b.Fatalf("unhealthy pass: %+v", res)
		}
		b.StopTimer()
		b.ReportMetric(float64(rt), "round-trips/op")
		b.StartTimer()
	}
}

// BenchmarkDoctorHostRoundTripsNativeOnly1 measures one full doctor host
// pass for a single native runner: 9 round trips today. The count covers the
// runHostChecks pass only — see the scope note at the top of this file.
func BenchmarkDoctorHostRoundTripsNativeOnly1(b *testing.B) {
	sc := nativeOnlyDoctorScenario()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt, _, res := runDoctorRoundTrips(sc)
		if res.Warn != 0 || res.Fail != 0 {
			b.Fatalf("unhealthy pass: %+v", res)
		}
		b.StopTimer()
		b.ReportMetric(float64(rt), "round-trips/op")
		b.StartTimer()
	}
}

// TestDoctorHostRoundTripsExact pins the healthy-pass round-trip counts and
// the integrity condition behind them: every check reaches its OK line, no
// check short-circuits into a WARN/FAIL (which would silently drop its
// round trips from the total), and the silent-healthy checks (orphans, disk)
// stay silent. These counts are the baseline the round-trip folds target.
func TestDoctorHostRoundTripsExact(t *testing.T) {
	t.Run("Mixed5", func(t *testing.T) {
		rt, out, res := runDoctorRoundTrips(mixedDoctorScenario())
		if res.Warn != 0 || res.Fail != 0 {
			t.Fatalf("unhealthy pass: %+v\noutput:\n%s", res, out)
		}
		for _, want := range []string{
			"dns: resolves api.github.com",
			"native: curl and tar present",
			"native: instance n-1 installed",
			"native: instance n-2 installed",
			"linux: non-root user has passwordless sudo",
			"container: host Docker available and --privileged supported",
			"container: instance c-1 — inner dockerd healthy",
			"container: instance c-2 — inner dockerd healthy",
			"container: instance aw-1 — inner dockerd healthy",
			"container: instance c-1 — registered (.runner present",
			"container(agent): inner Docker clean, host-gateway alias resolvable, node/npm + zstd on PATH, socket user ok, cache URL wired (gh-sr-aw-1)",
			"cache: healthy at http://172.17.0.1:",
			"cache: reachable from runner container",
			"registry: mirror healthy at http://172.17.0.1:5577",
			"registry: reachable from runner container",
			"extras: npm registry + shared caches wired into runner containers",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in output:\n%s", want, out)
			}
		}
		for _, unwanted := range []string{
			"[WARN]",
			"[FAIL]",
			"orphan runners:",
			"disk: instance",
			"bound to 0.0.0.0",
			"not deployed",
			"probe result returned",
		} {
			if strings.Contains(out, unwanted) {
				t.Errorf("unexpected %q in output:\n%s", unwanted, out)
			}
		}
		if rt != 28 {
			t.Errorf("round trips = %d, want 28 (update the benchmark comment and fold baselines when this moves — down is progress)", rt)
		}
	})

	t.Run("NativeOnly1", func(t *testing.T) {
		rt, out, res := runDoctorRoundTrips(nativeOnlyDoctorScenario())
		if res.Warn != 0 || res.Fail != 0 {
			t.Fatalf("unhealthy pass: %+v\noutput:\n%s", res, out)
		}
		for _, want := range []string{
			"dns: resolves api.github.com",
			"native: curl and tar present",
			"native: instance n-1 installed",
			"linux: non-root user has passwordless sudo",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in output:\n%s", want, out)
			}
		}
		for _, unwanted := range []string{
			"[WARN]",
			"[FAIL]",
			"orphan runners:",
			"disk: instance",
			"container:",
		} {
			if strings.Contains(out, unwanted) {
				t.Errorf("unexpected %q in output:\n%s", unwanted, out)
			}
		}
		if rt != 9 {
			t.Errorf("round trips = %d, want 9 (update the benchmark comment and fold baselines when this moves — down is progress)", rt)
		}
	})
}
