package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/cache"
	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// gatewayIP returns a docker0 `ip -4 -o addr show` line for the given gateway.
// The exact field order matches what cache.parseGatewayIPOutput walks.
func gatewayIP(ip string) string {
	return "2: docker0    inet " + ip + "/16 brd 172.17.255.255 scope global docker0\n"
}

// TestEffectiveBind pins the four-branch resolver: explicit BindAddr wins;
// otherwise the auto path returns the docker0 gateway from cache.ResolveGatewayIP;
// a failed/empty gateway lookup falls back to 0.0.0.0.
func TestEffectiveBind(t *testing.T) {
	t.Parallel()
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(&testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "ip -4 -o addr show docker0") {
				return gatewayIP("172.17.0.1"), nil
			}
			return "", nil
		},
	})

	cases := []struct {
		name     string
		settings cache.Settings
		want     string
	}{
		{"explicit bind addr wins", cache.Settings{BindAddr: "192.168.5.10"}, "192.168.5.10"},
		{"auto resolves docker0 gateway", cache.Settings{}, "172.17.0.1"},
		{"0.0.0.0 bind returns verbatim", cache.Settings{BindAddr: "0.0.0.0"}, "0.0.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := effectiveBind(&tc.settings, h); got != tc.want {
				t.Errorf("effectiveBind = %q; want %q", got, tc.want)
			}
		})
	}

	t.Run("gateway lookup failure falls back to 0.0.0.0", func(t *testing.T) {
		t.Parallel()
		fail := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		fail.SetConn(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "ip -4 -o addr show docker0") {
					return "", errors.New("no docker0")
				}
				return "", nil
			},
		})
		if got := effectiveBind(&cache.Settings{}, fail); got != "0.0.0.0" {
			t.Errorf("fallback = %q; want 0.0.0.0", got)
		}
	})

	t.Run("empty gateway output falls back to 0.0.0.0", func(t *testing.T) {
		t.Parallel()
		empty := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		empty.SetConn(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "ip -4 -o addr show docker0") {
					return "   \n", nil
				}
				return "", nil
			},
		})
		if got := effectiveBind(&cache.Settings{}, empty); got != "0.0.0.0" {
			t.Errorf("fallback = %q; want 0.0.0.0", got)
		}
	})
}

// TestProbeCacheFromRunner covers the five branches of the inner-container
// reachability probe:
//   - no container-mode instance configured for the host → early return (no warn)
//   - RunnerURL cannot be resolved → early return (no warn)
//   - h.Run returns an error → warn
//   - h.Run returns "PROBE_FAIL" → fail with the host-firewall remediation hint
//   - clean /health answer → ok
func TestProbeCacheFromRunner(t *testing.T) {
	t.Parallel()

	containerRunners := []config.RunnerConfig{
		{Name: "ci", Host: "h1", Repo: "o/r", Count: 1, RunnerMode: config.RunnerModeContainer},
	}
	nativeRunners := []config.RunnerConfig{
		{Name: "native", Host: "h1", Repo: "o/r", Count: 1},
	}

	t.Run("no container-mode instance on host returns silently", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		var r Result
		probeCacheFromRunner(&buf, "h1", nil, nativeRunners, &cache.Settings{Enabled: true, BindAddr: "172.17.0.1"}, &r)
		if buf.Len() != 0 {
			t.Errorf("expected no output, got:\n%s", buf.String())
		}
		if r.Warn != 0 || r.Fail != 0 {
			t.Errorf("expected no warn/fail, got %+v", r)
		}
	})

	t.Run("unresolvable RunnerURL returns silently", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "ip -4 -o addr show docker0") {
					return "", errors.New("no gateway")
				}
				return "", nil
			},
		})
		var buf strings.Builder
		var r Result
		probeCacheFromRunner(&buf, "h1", h, containerRunners, &cache.Settings{Enabled: true}, &r)
		if buf.Len() != 0 {
			t.Errorf("expected no output, got:\n%s", buf.String())
		}
		if r.Warn != 0 || r.Fail != 0 {
			t.Errorf("expected no warn/fail, got %+v", r)
		}
	})

	t.Run("docker exec error warns", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		sentinel := errors.New("docker exec failed")
		h.SetConn(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "docker exec") && strings.Contains(cmd, "curl -fsS") {
					return "boom", sentinel
				}
				return "", nil
			},
		})
		var buf strings.Builder
		var r Result
		probeCacheFromRunner(&buf, "h1", h, containerRunners, &cache.Settings{Enabled: true, BindAddr: "172.17.0.1"}, &r)
		out := buf.String()
		if !strings.Contains(out, "WARN") {
			t.Errorf("expected WARN severity, got:\n%s", out)
		}
		if !strings.Contains(out, "could not probe from runner container") {
			t.Errorf("expected warn message, got:\n%s", out)
		}
		if r.Warn != 1 || r.Fail != 0 {
			t.Errorf("expected 1 warn / 0 fail, got %+v", r)
		}
	})

	t.Run("PROBE_FAIL indicates host firewall blocking container traffic", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "docker exec") && strings.Contains(cmd, "curl -fsS") {
					return "PROBE_FAIL\n", nil
				}
				return "", nil
			},
		})
		var buf strings.Builder
		var r Result
		s := &cache.Settings{Enabled: true, BindAddr: "172.17.0.1", Port: 3001}
		probeCacheFromRunner(&buf, "h1", h, containerRunners, s, &r)
		out := buf.String()
		if !strings.Contains(out, "FAIL") {
			t.Errorf("expected FAIL severity, got:\n%s", out)
		}
		if !strings.Contains(out, "INPUT firewall is blocking container traffic") {
			t.Errorf("expected firewall remediation, got:\n%s", out)
		}
		if !strings.Contains(out, "ufw allow") {
			t.Errorf("expected ufw remediation hint, got:\n%s", out)
		}
		if r.Fail != 1 || r.Warn != 0 {
			t.Errorf("expected 1 fail / 0 warn, got %+v", r)
		}
	})

	t.Run("clean /health response prints OK", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "docker exec") && strings.Contains(cmd, "curl -fsS") {
					return "healthy\n", nil
				}
				return "", nil
			},
		})
		var buf strings.Builder
		var r Result
		probeCacheFromRunner(&buf, "h1", h, containerRunners, &cache.Settings{Enabled: true, BindAddr: "172.17.0.1"}, &r)
		out := buf.String()
		if !strings.Contains(out, "OK") {
			t.Errorf("expected OK severity, got:\n%s", out)
		}
		if !strings.Contains(out, "reachable from runner container") {
			t.Errorf("expected OK message, got:\n%s", out)
		}
		if r.Warn != 0 || r.Fail != 0 {
			t.Errorf("expected no warn/fail, got %+v", r)
		}
	})

	t.Run("first container-mode runner instance is selected", func(t *testing.T) {
		t.Parallel()
		runners := []config.RunnerConfig{
			{Name: "native", Host: "h1", Repo: "o/r", Count: 1},
			{Name: "ci", Host: "h1", Repo: "o/r", Count: 2, RunnerMode: config.RunnerModeContainer},
		}
		var observedCmd string
		h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "docker exec") && strings.Contains(cmd, "curl -fsS") {
					observedCmd = cmd
					return "healthy\n", nil
				}
				return "", nil
			},
		})
		wantCName := runner.ContainerDockerName("ci-1")
		var buf strings.Builder
		var r Result
		probeCacheFromRunner(&buf, "h1", h, runners, &cache.Settings{Enabled: true, BindAddr: "172.17.0.1"}, &r)
		if !strings.Contains(observedCmd, wantCName) {
			t.Errorf("expected probe to use first container instance %q, got cmd:\n%s", wantCName, observedCmd)
		}
	})
}

// TestCheckCacheReachability drives checkCacheReachability through its branches.
// cache.Inspect emits up to four SSH calls (docker inspect + du + curl + ip),
// so the mock answers each by command substring. Together the cases cover nil
// settings, Inspect failure, not-deployed, unhealthy, healthy, and the 0.0.0.0
// bind warning.
func TestCheckCacheReachability(t *testing.T) {
	t.Parallel()

	t.Run("nil settings returns silently", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		var r Result
		checkCacheReachability(&buf, "h1", nil, nil, nil, &r)
		if buf.Len() != 0 {
			t.Errorf("expected no output, got:\n%s", buf.String())
		}
		if r.Warn != 0 || r.Fail != 0 {
			t.Errorf("expected no warn/fail, got %+v", r)
		}
	})

	t.Run("cache.Inspect error warns", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("inspect failed")
		h := reachabilityHost(&testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, "docker inspect") {
					return "", sentinel
				}
				return "", nil
			},
		})
		var buf strings.Builder
		var r Result
		checkCacheReachability(&buf, "h1", h, nil, &cache.Settings{Enabled: true}, &r)
		out := buf.String()
		if !strings.Contains(out, "WARN") || !strings.Contains(out, "inspect failed") {
			t.Errorf("expected inspect-failed warn, got:\n%s", out)
		}
		if r.Warn != 1 || r.Fail != 0 {
			t.Errorf("expected 1 warn / 0 fail, got %+v", r)
		}
	})

	t.Run("not deployed warns and suggests gh sr cache deploy", func(t *testing.T) {
		t.Parallel()
		h := reachabilityHost(reachabilityMock("", "", "", ""))
		var buf strings.Builder
		var r Result
		checkCacheReachability(&buf, "h1", h, nil, &cache.Settings{Enabled: true, StoragePath: "/srv/cache"}, &r)
		out := buf.String()
		if !strings.Contains(out, "not deployed") {
			t.Errorf("expected not-deployed message, got:\n%s", out)
		}
		if !strings.Contains(out, "gh sr cache deploy") {
			t.Errorf("expected deploy hint, got:\n%s", out)
		}
		if r.Warn != 1 || r.Fail != 0 {
			t.Errorf("expected 1 warn / 0 fail, got %+v", r)
		}
	})

	t.Run("unhealthy inspect fails with docker logs hint", func(t *testing.T) {
		t.Parallel()
		// running container, /health probe errors → Healthy=false
		h := reachabilityHost(reachabilityMock("running|example.com/cache:v1", "", "", "no docker0"))
		var buf strings.Builder
		var r Result
		checkCacheReachability(&buf, "h1", h, nil, &cache.Settings{Enabled: true, StoragePath: "/srv/cache"}, &r)
		out := buf.String()
		if !strings.Contains(out, "FAIL") {
			t.Errorf("expected FAIL severity, got:\n%s", out)
		}
		if !strings.Contains(out, "server not healthy") {
			t.Errorf("expected unhealthy message, got:\n%s", out)
		}
		if !strings.Contains(out, "docker logs") {
			t.Errorf("expected docker logs hint, got:\n%s", out)
		}
		if r.Fail != 1 || r.Warn != 0 {
			t.Errorf("expected 1 fail / 0 warn, got %+v", r)
		}
	})

	t.Run("healthy cache prints OK without 0.0.0.0 warning when BindAddr is set", func(t *testing.T) {
		t.Parallel()
		h := reachabilityHost(reachabilityMock("running|example.com/cache:v1", "/srv/cache", "healthy\n", gatewayIP("172.17.0.1")))
		var buf strings.Builder
		var r Result
		checkCacheReachability(&buf, "h1", h, nil, &cache.Settings{Enabled: true, BindAddr: "172.17.0.1", StoragePath: "/srv/cache"}, &r)
		out := buf.String()
		if !strings.Contains(out, "OK") || !strings.Contains(out, "healthy at") {
			t.Errorf("expected healthy OK, got:\n%s", out)
		}
		if strings.Contains(out, "0.0.0.0") {
			t.Errorf("did not expect 0.0.0.0 warning for explicit BindAddr, got:\n%s", out)
		}
		if r.Warn != 0 || r.Fail != 0 {
			t.Errorf("expected no warn/fail, got %+v", r)
		}
	})

	t.Run("healthy cache warns when effective bind is 0.0.0.0", func(t *testing.T) {
		t.Parallel()
		// BindAddr="0.0.0.0" → effectiveBind returns "0.0.0.0" verbatim, but
		// localURL still falls through to the docker0 gateway so the /health
		// probe can succeed. The 0.0.0.0 warning only fires when the cache is
		// actually healthy.
		h := reachabilityHost(reachabilityMock("running|example.com/cache:v1", "/srv/cache", "healthy\n", gatewayIP("172.17.0.1")))
		var buf strings.Builder
		var r Result
		checkCacheReachability(&buf, "h1", h, nil, &cache.Settings{Enabled: true, BindAddr: "0.0.0.0", StoragePath: "/srv/cache"}, &r)
		out := buf.String()
		if !strings.Contains(out, "OK") {
			t.Errorf("expected healthy OK, got:\n%s", out)
		}
		if !strings.Contains(out, "WARN") || !strings.Contains(out, "0.0.0.0") {
			t.Errorf("expected 0.0.0.0 warning, got:\n%s", out)
		}
		if !strings.Contains(out, "LAN") {
			t.Errorf("expected LAN warning text, got:\n%s", out)
		}
		if r.Warn != 1 || r.Fail != 0 {
			t.Errorf("expected 1 warn / 0 fail, got %+v", r)
		}
	})
}

// reachabilityMock returns a MockExecutor that responds to the four SSH
// commands cache.Inspect may issue:
//   - dockerInspectReply: docker inspect --format='{{.State.Status}}|{{.Config.Image}}' gh-sr-cache ...
//   - duReply: du -sb <storage> ... | cut -f1
//   - curlReply: curl -fsS -m 3 <url>/health ...
//   - ipReply: ip -4 -o addr show docker0
//
// Pass "no docker0" for ipReply to simulate a gateway lookup failure (the
// helper returns an error so cache.ResolveGatewayIP returns ("", err)).
// Other commands (notably probeCacheFromRunner's docker exec curl) get
// healthy/empty so the function does not contribute extra warns/fails in
// tests that don't intend to exercise the probe.
func reachabilityMock(dockerInspectReply, duReply, curlReply, ipReply string) *testutil.MockExecutor {
	return &testutil.MockExecutor{
		RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "docker inspect --format='{{.State.Status}}|{{.Config.Image}}'"):
				return dockerInspectReply, nil
			case strings.Contains(cmd, "du -sb"):
				return duReply, nil
			case strings.Contains(cmd, "curl -fsS -m 3") && strings.Contains(cmd, "/health"):
				if ipReply == "no docker0" {
					return "", errors.New("connect: connection refused")
				}
				return curlReply, nil
			case strings.Contains(cmd, "ip -4 -o addr show docker0"):
				if ipReply == "no docker0" {
					return "", errors.New("no docker0")
				}
				return ipReply, nil
			case strings.Contains(cmd, "docker exec") && strings.Contains(cmd, "curl -fsS"):
				return "healthy\n", nil
			default:
				return "", nil
			}
		},
	}
}

// reachabilityHost wraps a MockExecutor in a Linux host with the local addres
// so wrapCommand is a no-op and the mock sees the literal commands.
func reachabilityHost(mock *testutil.MockExecutor) *host.Host {
	h := host.NewHost("linux", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	return h
}
