package dockerutil

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

const gwOutput = "2: docker0    inet 172.17.0.1/16 brd 172.17.255.255 scope global docker0\\       valid_lft forever preferred_lft forever\n"

func newUtilHost(t *testing.T, mock *testutil.MockExecutor) *host.Host {
	t.Helper()
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	return h
}

func TestParseGatewayIPOutput(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{gwOutput, "172.17.0.1"},
		{"2: docker0    inet 10.0.0.1/16 scope global docker0\n", "10.0.0.1"},
		{"2: docker0    inet6 fe80::1/64 scope link\n", ""},
		{"", ""},
		{"command not found\n", ""},
	}
	for _, tc := range cases {
		if got := ParseGatewayIPOutput(tc.in); got != tc.want {
			t.Errorf("ParseGatewayIPOutput(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveGatewayIP(t *testing.T) {
	t.Parallel()
	got, err := ResolveGatewayIP(newUtilHost(t, &testutil.MockExecutor{Output: gwOutput}))
	if err != nil || got != "172.17.0.1" {
		t.Errorf("gateway: got %q err %v", got, err)
	}
	if _, err := ResolveGatewayIP(newUtilHost(t, &testutil.MockExecutor{RunErr: io.EOF})); err == nil {
		t.Error("transport failure should error")
	}
}

// TestEffectiveBind pins the four-branch resolver: explicit bind wins;
// otherwise the auto path returns the docker0 gateway; a failed/empty gateway
// lookup falls back to 0.0.0.0. 0.0.0.0 itself is returned verbatim (it is a
// real choice, not the fallback).
func TestEffectiveBind(t *testing.T) {
	t.Parallel()
	h := newUtilHost(t, &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "ip -4 -o addr show docker0") {
			return gwOutput, nil
		}
		return "", nil
	}})
	cases := []struct {
		name string
		bind string
		want string
	}{
		{"explicit bind addr wins", "192.168.5.10", "192.168.5.10"},
		{"0.0.0.0 bind returns verbatim", "0.0.0.0", "0.0.0.0"},
		{"auto resolves docker0 gateway", "", "172.17.0.1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := EffectiveBind(h, tc.bind); got != tc.want {
				t.Errorf("EffectiveBind(%q) = %q; want %q", tc.bind, got, tc.want)
			}
		})
	}

	t.Run("gateway lookup failure falls back to 0.0.0.0", func(t *testing.T) {
		t.Parallel()
		fail := newUtilHost(t, &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "ip -4 -o addr show docker0") {
				return "", errors.New("no docker0")
			}
			return "", nil
		}})
		if got := EffectiveBind(fail, ""); got != "0.0.0.0" {
			t.Errorf("fallback = %q; want 0.0.0.0", got)
		}
	})

	t.Run("empty gateway output falls back to 0.0.0.0", func(t *testing.T) {
		t.Parallel()
		empty := newUtilHost(t, &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "ip -4 -o addr show docker0") {
				return "   \n", nil
			}
			return "", nil
		}})
		if got := EffectiveBind(empty, ""); got != "0.0.0.0" {
			t.Errorf("fallback = %q; want 0.0.0.0", got)
		}
	})
}

func TestResolveBind(t *testing.T) {
	t.Parallel()

	t.Run("explicit bind skips gateway lookup and warning", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		bind, gw := ResolveBind(&buf, newUtilHost(t, &testutil.MockExecutor{
			RunFn: func(cmd string) (string, error) {
				t.Error("explicit bind must not resolve the gateway")
				return gwOutput, nil
			},
		}), "cache", "10.0.0.5", "cache API is exposed on every host interface")
		if bind != "10.0.0.5" || gw != "" {
			t.Errorf("bind=%q gw=%q", bind, gw)
		}
		if strings.Contains(buf.String(), "warning") {
			t.Errorf("explicit bind must not warn: %q", buf.String())
		}
	})

	t.Run("auto path binds gateway and returns it", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		bind, gw := ResolveBind(&buf, newUtilHost(t, &testutil.MockExecutor{Output: gwOutput}), "registry", "", "the mirror is exposed on every host interface")
		if bind != "172.17.0.1" || gw != "172.17.0.1" {
			t.Errorf("bind=%q gw=%q", bind, gw)
		}
		if strings.Contains(buf.String(), "warning") {
			t.Errorf("gateway found must not warn: %q", buf.String())
		}
	})

	t.Run("missing gateway warns and binds 0.0.0.0", func(t *testing.T) {
		t.Parallel()
		var buf strings.Builder
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, "ip -4 -o addr show docker0") {
				return "device not found\n", nil
			}
			return "", nil
		}}
		bind, gw := ResolveBind(&buf, newUtilHost(t, mock), "cache", "", "cache API is exposed on every host interface")
		if bind != "0.0.0.0" || gw != "" {
			t.Errorf("bind=%q gw=%q", bind, gw)
		}
		out := buf.String()
		if !strings.Contains(out, "docker0 gateway not found") || !strings.Contains(out, "binding 0.0.0.0") {
			t.Errorf("expected fallback warning, got: %q", out)
		}
	})
}

func TestPullImage_bestEffort(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "docker pull") {
			return "", errors.New("no network")
		}
		return "", nil
	}}
	PullImage(&buf, newUtilHost(t, mock), "cache", "example/img:1")
	if !strings.Contains(buf.String(), "  cache: pulling example/img:1...\n") {
		t.Errorf("expected pull progress line, got: %q", buf.String())
	}
	last := mock.Calls[len(mock.Calls)-1]
	if !strings.Contains(last, "docker pull 'example/img:1'") {
		t.Errorf("image must be quoted: %q", last)
	}
	// Best-effort by design: PullImage signals nothing — it compiles as a
	// statement without assignment.
}

func TestContainerState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		want string
	}{
		{"running", "running\n", "running"},
		{"exists but stopped", "exited\n", "exists"},
		{"created but never started", "created\n", "exists"},
		{"absent", "", ""},
		{"whitespace only", "  \n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
				if strings.Contains(cmd, ".State.Status") {
					return tc.out, nil
				}
				return "", nil
			}}
			got, err := ContainerState(newUtilHost(t, mock), "gh-sr-cache")
			if err != nil || got != tc.want {
				t.Errorf("state = %q err %v; want %q", got, err, tc.want)
			}
		})
	}

	t.Run("transport failure wraps", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("ssh gone")
		mock := &testutil.MockExecutor{RunFn: func(string) (string, error) { return "", sentinel }}
		_, err := ContainerState(newUtilHost(t, mock), "gh-sr-cache")
		if !errors.Is(err, sentinel) {
			t.Errorf("want wrapped sentinel, got %v", err)
		}
	})
}

// TestLayoutCurrent pins exact-label equality: a container stamped with the
// current revision is current; one with an older revision, a prefix collision
// ("v33" vs rev "v3"), or no label at all is stale. The cache implementation
// this helper replaces matched on a substring, so "v33" reported current —
// the shared helper must not.
func TestLayoutCurrent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"current revision", "v3\n", true},
		{"older revision is stale", "v2\n", false},
		{"prefix collision is stale", "v33\n", false},
		{"no label is stale", "<no value>\n", false},
		{"absent output is stale", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cmd string
			mock := &testutil.MockExecutor{RunFn: func(c string) (string, error) {
				if strings.Contains(c, "Config.Labels") {
					cmd = c
					return tc.out, nil
				}
				return "", nil
			}}
			got, err := LayoutCurrent(newUtilHost(t, mock), "gh-sr-cache", "gh-sr.cache-layout", "v3")
			if err != nil || got != tc.want {
				t.Errorf("current = %v err %v; want %v", got, err, tc.want)
			}
			if !strings.Contains(cmd, `{{index .Config.Labels "gh-sr.cache-layout"}}`) {
				t.Errorf("label must be queried by name, got: %q", cmd)
			}
		})
	}
}

func TestConverge(t *testing.T) {
	t.Parallel()

	// convergedMock answers the two inspects so the container reports running
	// with the current layout — the no-op path.
	convergedMock := func() *testutil.MockExecutor {
		return &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				return "v3\n", nil
			case strings.Contains(cmd, ".State.Status"):
				return "running\n", nil
			}
			return "", nil
		}}
	}

	opts := func(deploy func() error, afterRemove func()) ConvergeOpts {
		return ConvergeOpts{
			Container: "gh-sr-cache", LayoutLabel: "gh-sr.cache-layout", LayoutRev: "v3",
			Service: "cache", Noun: "cache container",
			Deploy: deploy, AfterRemove: afterRemove,
		}
	}

	t.Run("running with current layout is a no-op", func(t *testing.T) {
		t.Parallel()
		mock := convergedMock()
		deployed := false
		if err := Converge(io.Discard, newUtilHost(t, mock), opts(func() error {
			deployed = true
			return nil
		}, nil)); err != nil {
			t.Fatalf("Converge: %v", err)
		}
		if deployed {
			t.Error("running container with current layout must not redeploy")
		}
		for _, c := range mock.Calls {
			if strings.Contains(c, "docker start") || strings.Contains(c, "docker rm") || strings.Contains(c, "docker run") {
				t.Fatalf("no-op converge must not touch the container, calls: %v", mock.Calls)
			}
		}
	})

	t.Run("stopped container is started", func(t *testing.T) {
		t.Parallel()
		deployed := false
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				return "v3\n", nil
			case strings.Contains(cmd, ".State.Status"):
				return "exited\n", nil
			}
			return "", nil
		}}
		if err := Converge(io.Discard, newUtilHost(t, mock), opts(func() error {
			deployed = true
			return nil
		}, nil)); err != nil {
			t.Fatalf("Converge: %v", err)
		}
		if deployed {
			t.Error("stopped current-layout container must start, not redeploy")
		}
		last := mock.Calls[len(mock.Calls)-1]
		if !strings.Contains(last, "docker start gh-sr-cache") {
			t.Errorf("expected docker start, got %q", last)
		}
	})

	t.Run("missing container is deployed", func(t *testing.T) {
		t.Parallel()
		deployed := false
		mock := &testutil.MockExecutor{}
		if err := Converge(io.Discard, newUtilHost(t, mock), opts(func() error {
			deployed = true
			return nil
		}, nil)); err != nil {
			t.Fatalf("Converge: %v", err)
		}
		if !deployed {
			t.Error("missing container must reach Deploy")
		}
		// Absent container: only the state inspect runs, no layout inspect.
		if len(mock.Calls) != 1 {
			t.Errorf("expected 1 call (state inspect), got %v", mock.Calls)
		}
	})

	t.Run("stale layout is recreated via Deploy with AfterRemove hook", func(t *testing.T) {
		t.Parallel()
		deployed, hookRan := false, false
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				return "<no value>\n", nil
			case strings.Contains(cmd, ".State.Status"):
				return "running\n", nil
			case strings.Contains(cmd, "docker rm -f"):
				return "", nil
			}
			return "", nil
		}}
		var buf strings.Builder
		o := opts(func() error { deployed = true; return nil }, func() { hookRan = true })
		if err := Converge(&buf, newUtilHost(t, mock), o); err != nil {
			t.Fatalf("Converge: %v", err)
		}
		if !deployed || !hookRan {
			t.Errorf("stale container must be recreated (deployed=%v hook=%v)", deployed, hookRan)
		}
		if !strings.Contains(buf.String(), "was deployed by an older gh-sr layout; recreating") {
			t.Errorf("expected recreate message, got: %q", buf.String())
		}
	})

	t.Run("stale layout remove failure is wrapped with the noun", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				return "<no value>\n", nil
			case strings.Contains(cmd, ".State.Status"):
				return "running\n", nil
			case strings.Contains(cmd, "docker rm -f"):
				return "", errors.New("rm denied")
			}
			return "", nil
		}}
		err := Converge(io.Discard, newUtilHost(t, mock), opts(nil, nil))
		if err == nil || !strings.Contains(err.Error(), "removing outdated cache container") {
			t.Errorf("expected wrapped rm error, got %v", err)
		}
	})

	t.Run("start failure is wrapped with the noun", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				return "v3\n", nil
			case strings.Contains(cmd, ".State.Status"):
				return "exited\n", nil
			case strings.Contains(cmd, "docker start"):
				return "", errors.New("start denied")
			}
			return "", nil
		}}
		err := Converge(io.Discard, newUtilHost(t, mock), opts(nil, nil))
		if err == nil || !strings.Contains(err.Error(), "starting cache container") {
			t.Errorf("expected wrapped start error, got %v", err)
		}
	})

	t.Run("deploy error propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("deploy failed")
		mock := &testutil.MockExecutor{}
		if err := Converge(io.Discard, newUtilHost(t, mock), opts(func() error { return sentinel }, nil)); !errors.Is(err, sentinel) {
			t.Errorf("want sentinel, got %v", err)
		}
	})

	t.Run("state inspect failure propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("inspect failed")
		deployed := false
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			if strings.Contains(cmd, ".State.Status") {
				return "", sentinel
			}
			return "", nil
		}}
		if err := Converge(io.Discard, newUtilHost(t, mock), opts(func() error {
			deployed = true
			return nil
		}, nil)); !errors.Is(err, sentinel) {
			t.Errorf("want sentinel, got %v", err)
		}
		if deployed {
			t.Error("swallowing the state error would fall through to deploy")
		}
	})

	t.Run("layout inspect failure propagates", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("layout inspect failed")
		calls := 0
		mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
			switch {
			case strings.Contains(cmd, "Config.Labels"):
				calls++
				return "", sentinel
			case strings.Contains(cmd, ".State.Status"):
				return "running\n", nil
			}
			return "", nil
		}}
		if err := Converge(io.Discard, newUtilHost(t, mock), opts(nil, nil)); !errors.Is(err, sentinel) {
			t.Errorf("want sentinel, got %v", err)
		}
		if calls != 1 {
			t.Errorf("expected exactly 1 layout inspect, got %d", calls)
		}
	})
}

// TestConverge_servicePrefix pins the registry-flavored rendering so both
// packages can share the machine without sharing their message text.
func TestConverge_servicePrefix(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "Config.Labels"):
			return "v2\n", nil
		case strings.Contains(cmd, ".State.Status"):
			return "exited\n", nil
		case strings.Contains(cmd, "docker start"):
			return "", errors.New("start denied")
		}
		return "", nil
	}}
	var buf strings.Builder
	err := Converge(&buf, newUtilHost(t, mock), ConvergeOpts{
		Container: "gh-sr-registry", LayoutLabel: "gh-sr.registry-layout", LayoutRev: "v2",
		Service: "registry", Noun: "registry mirror container",
		Deploy: func() error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "starting registry mirror container") {
		t.Fatalf("expected registry noun in error, got %v", err)
	}
	if !strings.Contains(buf.String(), "  registry: starting gh-sr-registry...") {
		t.Errorf("expected registry prefix on progress line, got: %q", buf.String())
	}
}
