package registry

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

const gwOutput = "2: docker0    inet 172.17.0.1/16 brd 172.17.255.255 scope global docker0\\       valid_lft forever preferred_lft forever\n"

func newRegistryHost(t *testing.T, mock *testutil.MockExecutor) *host.Host {
	t.Helper()
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	return h
}

func TestRunnerURL_precedence(t *testing.T) {
	t.Parallel()
	h := newRegistryHost(t, &testutil.MockExecutor{Output: gwOutput})

	s := Settings{Enabled: true, URLOverride: "http://override:9999"}
	if got := s.RunnerURL(h); got != "http://override:9999" {
		t.Errorf("override: got %q", got)
	}

	s = Settings{Enabled: true, BindAddr: "192.168.1.5", Mirrors: []MirrorSettings{{Name: "dockerhub", Port: 3001}}}
	if got := s.RunnerURL(h); got != "http://192.168.1.5:3001" {
		t.Errorf("explicit bind: got %q", got)
	}

	// 0.0.0.0 is not injectable; fall through to the docker0 gateway.
	s = Settings{Enabled: true, BindAddr: "0.0.0.0"}
	if got := s.RunnerURL(h); got != fmt.Sprintf("http://172.17.0.1:%d", DefaultPort) {
		t.Errorf("0.0.0.0 bind + gateway: got %q", got)
	}

	s = Settings{Enabled: true}
	if got := s.RunnerURL(h); got != fmt.Sprintf("http://172.17.0.1:%d", DefaultPort) {
		t.Errorf("auto bind: got %q", got)
	}

	// No docker0 → no container-reachable address → skip injection.
	s = Settings{Enabled: true}
	if got := s.RunnerURL(newRegistryHost(t, &testutil.MockExecutor{RunErr: io.EOF})); got != "" {
		t.Errorf("gateway failure: got %q", got)
	}
	s = Settings{Enabled: true}
	if got := s.RunnerURL(newRegistryHost(t, &testutil.MockExecutor{Output: "device not found\n"})); got != "" {
		t.Errorf("no docker0: got %q", got)
	}
}

func TestResolvedStoragePath(t *testing.T) {
	t.Parallel()
	h := newRegistryHost(t, &testutil.MockExecutor{Output: "/home/op\n"})
	s := Settings{Enabled: true}
	got, err := s.mirrorStoragePath(0, h)
	if err != nil || got != "/home/op/.gh-sr/registry" {
		t.Errorf("default path: got %q, err %v", got, err)
	}
	s = Settings{Enabled: true, StoragePath: "/srv/mirror"}
	if got, _ := s.mirrorStoragePath(0, h); got != "/srv/mirror" {
		t.Errorf("absolute path: got %q", got)
	}
	// Extra mirrors append -<name> to the same base; $HOME is expanded once.
	s = Settings{Enabled: true, Mirrors: []MirrorSettings{
		{Name: "dockerhub", Port: 5577},
		{Name: "ghcr", Port: 5578},
	}}
	paths, err := s.mirrorStoragePaths(h)
	if err != nil {
		t.Fatalf("mirrorStoragePaths: %v", err)
	}
	want := []string{"/home/op/.gh-sr/registry", "/home/op/.gh-sr/registry-ghcr"}
	if len(paths) != len(want) {
		t.Fatalf("got %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("path[%d]: got %q, want %q", i, paths[i], want[i])
		}
	}
}

// TestMirrorStoragesQuotingPerPath guards the disk-usage measurement: every
// mirror dir must reach du as its own argument. Quoting the joined list hands
// du a single path containing spaces, which measures nothing and reports 0 for
// every multi-mirror host.
func TestMirrorStoragesQuotingPerPath(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{Output: ""}
	h := newRegistryHost(t, mock)
	s := Settings{
		Enabled:     true,
		StoragePath: "/srv/registry",
		Mirrors: []MirrorSettings{
			{Name: "dockerhub", Port: 5577},
			{Name: "ghcr", Port: 5578},
		},
	}
	mock.Output = "4096\t/srv/registry\n8192\t/srv/registry-ghcr\n"
	stores, err := MirrorStorages(h, s)
	if err != nil {
		t.Fatalf("MirrorStorages: %v", err)
	}
	if len(stores) != 2 {
		t.Fatalf("want one entry per mirror, got %d", len(stores))
	}
	if stores[0].Container != ContainerName || stores[0].Path != "/srv/registry" || stores[0].Bytes != 4096 {
		t.Errorf("mirror 0: %+v", stores[0])
	}
	if stores[1].Container != ContainerNamePrefix+"ghcr" || stores[1].Path != "/srv/registry-ghcr" || stores[1].Bytes != 8192 {
		t.Errorf("mirror 1: %+v", stores[1])
	}
	var du string
	for _, c := range mock.Calls {
		if strings.HasPrefix(c, "du ") {
			du = c
		}
	}
	if !strings.Contains(du, "'/srv/registry' '/srv/registry-ghcr'") {
		t.Errorf("each dir must be quoted separately, got: %s", du)
	}

	// A mirror whose dir does not exist yet reports 0 without shifting the
	// other mirrors' sizes onto it.
	mock.Output = "4096\t/srv/registry\n"
	stores, err = MirrorStorages(h, s)
	if err != nil {
		t.Fatalf("MirrorStorages (missing dir): %v", err)
	}
	if stores[0].Bytes != 4096 || stores[1].Bytes != 0 {
		t.Errorf("missing dir must not shift sizes: %+v", stores)
	}
}

func TestEnsure_disabled(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{}
	if err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{}); err != nil {
		t.Fatalf("Ensure disabled: %v", err)
	}
	if len(mock.Calls) != 0 {
		t.Fatalf("disabled mirror must not run commands, got %v", mock.Calls)
	}
}

func TestEnsure_runningNoop(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "Config.Labels") {
			return registryLayoutRev + "\n", nil
		}
		if strings.Contains(cmd, ".State.Status") {
			return "running\n", nil
		}
		return "", nil
	}}
	if err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}); err != nil {
		t.Fatalf("Ensure running: %v", err)
	}
	for _, c := range mock.Calls {
		if strings.Contains(c, "docker start") || strings.Contains(c, "docker rm") || strings.Contains(c, "docker run") {
			t.Fatalf("running container with the current layout must be a no-op, calls: %v", mock.Calls)
		}
	}
}

func TestEnsure_existsStarts(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "Config.Labels"):
			return registryLayoutRev + "\n", nil
		case strings.Contains(cmd, ".State.Status"):
			return "exited\n", nil
		default:
			return "", nil
		}
	}}
	if err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}); err != nil {
		t.Fatalf("Ensure exists: %v", err)
	}
	last := mock.Calls[len(mock.Calls)-1]
	if !strings.Contains(last, "docker start "+ContainerName) {
		t.Fatalf("stopped container must be started, got %q", last)
	}
}

func TestEnsure_recreatesWhenLayoutStale(t *testing.T) {
	t.Parallel()
	created := false
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "Config.Labels"):
			return "<no value>\n", nil
		case strings.Contains(cmd, ".State.Status"):
			return "running\n", nil
		case strings.Contains(cmd, "docker rm -f"):
			return "", nil
		case strings.Contains(cmd, "docker run"):
			created = true
			return "", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		default:
			return "", nil
		}
	}}
	var buf strings.Builder
	if err := Ensure(&buf, newRegistryHost(t, mock), Settings{Enabled: true}); err != nil {
		t.Fatalf("Ensure stale layout: %v", err)
	}
	if !strings.Contains(buf.String(), "recreating") {
		t.Errorf("expected recreate message, got: %s", buf.String())
	}
	if !created {
		t.Errorf("expected redeploy, calls: %v", mock.Calls)
	}
}

func TestEnsure_missingDeploys(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status"):
			return "", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		default:
			return "", nil
		}
	}}
	if err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}); err != nil {
		t.Fatalf("Ensure missing: %v", err)
	}

	var runLine string
	for _, c := range mock.Calls {
		if strings.Contains(c, "docker run") {
			runLine = c
		}
	}
	if runLine == "" {
		t.Fatalf("no docker run call: %v", mock.Calls)
	}
	for _, want := range []string{
		"docker run -d --name " + ContainerName,
		"--restart unless-stopped",
		"--label " + LayoutLabel + "=" + registryLayoutRev,
		fmt.Sprintf("-p 172.17.0.1:%d:%d", DefaultPort, ContainerPort),
		"-v '/root/.gh-sr/registry':/var/lib/registry",
		fmt.Sprintf("-e 'REGISTRY_PROXY_REMOTEURL=%s'", DefaultUpstream),
		"'registry:2'",
	} {
		if !strings.Contains(runLine, want) {
			t.Errorf("docker run missing %q:\n%s", want, runLine)
		}
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "echo $HOME") {
			return "/root\n", nil
		}
		return "", nil
	}}
	if err := Remove(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}, true); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !contains(mock.Calls, "docker rm -f "+ContainerName) {
		t.Errorf("expected container removal, calls: %v", mock.Calls)
	}
	if !contains(mock.Calls, "rm -rf '/root/.gh-sr/registry'") {
		t.Errorf("expected storage purge, calls: %v", mock.Calls)
	}
}

func contains(calls []string, want string) bool {
	for _, c := range calls {
		if c == want {
			return true
		}
	}
	return false
}
