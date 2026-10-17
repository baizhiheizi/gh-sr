package registry

import (
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestRegistryHostOf(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"ubuntu":                                "docker.io",
		"postgres:16":                           "docker.io",
		"docker.io/library/postgres:16":         "docker.io",
		"ghcr.io/github/gh-aw-mcpg:latest":      "ghcr.io",
		"ghcr.io/github/a/b@sha256:abcd":        "ghcr.io",
		"registry.example.com:5000/team/app:v1": "registry.example.com:5000",
		"localhost:5000/app":                    "localhost:5000",
		"GHCR.IO/upper/case":                    "ghcr.io",
	}
	for ref, want := range cases {
		if got := RegistryHostOf(ref); got != want {
			t.Errorf("RegistryHostOf(%q) = %q, want %q", ref, got, want)
		}
	}
}

func TestMirrorPullRef(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		// Bare official names gain library/ on the mirror path.
		"ubuntu": "127.0.0.1:5577/library/ubuntu",
		// Docker Hub refs drop the docker.io prefix.
		"docker.io/library/postgres:16": "127.0.0.1:5577/library/postgres:16",
		// Other registries drop their host prefix.
		"ghcr.io/github/gh-aw-mcpg:latest": "127.0.0.1:5578/github/gh-aw-mcpg:latest",
		"ghcr.io/github/a@sha256:abcd":     "127.0.0.1:5578/github/a@sha256:abcd",
	}
	for ref, want := range cases {
		base := "127.0.0.1:5577"
		if strings.HasPrefix(want, "127.0.0.1:5578") {
			base = "127.0.0.1:5578"
		}
		if got := mirrorPullRef(base, ref); got != want {
			t.Errorf("mirrorPullRef(%q) = %q, want %q", ref, got, want)
		}
	}
}

func TestMirrorForRef(t *testing.T) {
	t.Parallel()
	s := &Settings{Mirrors: []MirrorSettings{
		{Name: "dockerhub", Port: 5577}, // Upstream "" = DefaultUpstream
		{Name: "ghcr", Upstream: "https://ghcr.io", Port: 5578},
	}}
	cases := map[string]int{
		"ubuntu":                        0,
		"docker.io/library/postgres:16": 0,
		"ghcr.io/github/gh-aw-mcpg":     1,
		"quay.io/x/y":                   -1,
	}
	for ref, want := range cases {
		if got := s.mirrorForRef(ref); got != want {
			t.Errorf("mirrorForRef(%q) = %d, want %d", ref, got, want)
		}
	}
}

func TestHostPullRefBase(t *testing.T) {
	t.Parallel()
	s := Settings{Mirrors: []MirrorSettings{{Port: 5577}, {Name: "ghcr", Port: 5578}}}
	if got := s.hostPullRefBase(1); got != "127.0.0.1:5578" {
		t.Errorf("default bind: got %q", got)
	}
	s.BindAddr = "0.0.0.0"
	if got := s.hostPullRefBase(0); got != "127.0.0.1:5577" {
		t.Errorf("0.0.0.0 bind: got %q", got)
	}
	s.BindAddr = "127.0.0.1"
	if got := s.hostPullRefBase(0); got != "127.0.0.1:5577" {
		t.Errorf("loopback bind: got %q", got)
	}
	// A non-loopback explicit bind publishes no loopback address: the host
	// daemon (no insecure-registry config) cannot pull plain HTTP through it.
	s.BindAddr = "192.168.1.5"
	if got := s.hostPullRefBase(0); got != "" {
		t.Errorf("remote bind: got %q, want empty", got)
	}
}

// TestSyncPrefetch pins the happy-path flow: a ghcr.io ref is pulled through
// the matching mirror (loopback base), retagged to its canonical name, and
// streamed into every runner container; containers that already have the
// image ID are skipped.
func TestSyncPrefetch(t *testing.T) {
	t.Parallel()
	const imageID = "sha256:deadbeef"
	var commands []string
	// The inspect-by-ID check runs via `docker exec <c> docker image inspect
	// <id> && echo yes || echo no`: the first runner lacks the image, the
	// second already has it.
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		commands = append(commands, cmd)
		switch {
		case strings.HasPrefix(cmd, "docker pull "):
			return "", nil
		case strings.HasPrefix(cmd, "docker tag "):
			return "", nil
		case strings.HasPrefix(cmd, "docker image inspect --format='{{.Id}}'"):
			return imageID + "\n", nil
		case strings.Contains(cmd, "'gh-sr-r-1'") && strings.Contains(cmd, "echo yes"):
			return "no\n", nil
		case strings.Contains(cmd, "'gh-sr-r-2'") && strings.Contains(cmd, "echo yes"):
			return "yes\n", nil
		default:
			return "", nil
		}
	}}
	s := &Settings{Mirrors: []MirrorSettings{
		{Name: "dockerhub", Port: 5577},
		{Name: "ghcr", Upstream: "https://ghcr.io", Port: 5578},
	}}
	var buf strings.Builder
	res, err := SyncPrefetch(&buf, newRegistryHost(t, mock), s,
		[]string{"ghcr.io/github/gh-aw-mcpg:latest"}, []string{"gh-sr-r-1", "gh-sr-r-2"})
	if err != nil {
		t.Fatalf("SyncPrefetch: %v (log: %s)", err, buf.String())
	}
	if res.Pulled != 1 || res.Loaded != 1 || res.Cached != 1 {
		t.Fatalf("got %+v, want pulled=1 loaded=1 cached=1", res)
	}
	all := strings.Join(commands, "\n")
	for _, want := range []string{
		// Pulled through the ghcr mirror's loopback address…
		"docker pull '127.0.0.1:5578/github/gh-aw-mcpg:latest'",
		// …then retagged to the canonical reference.
		"docker tag '127.0.0.1:5578/github/gh-aw-mcpg:latest' 'ghcr.io/github/gh-aw-mcpg:latest'",
		// Streamed by image ID into the runner that lacked it, then retagged
		// inside the inner dockerd.
		"docker save 'sha256:deadbeef' | docker exec -i 'gh-sr-r-1' docker load",
		"docker exec 'gh-sr-r-1' docker tag 'sha256:deadbeef' 'ghcr.io/github/gh-aw-mcpg:latest'",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing command %q; got:\n%s", want, all)
		}
	}
	// The runner that already had the image must not receive a save/load.
	if strings.Contains(all, "docker exec -i 'gh-sr-r-2'") {
		t.Errorf("cached runner must be skipped, got:\n%s", all)
	}
}

// TestSyncPrefetchDirectPull pins the no-matching-mirror fallback: the
// upstream reference itself is pulled, no retag happens.
func TestSyncPrefetchDirectPull(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.HasPrefix(cmd, "docker pull "):
			if cmd != "docker pull 'quay.io/x/y:1'" {
				t.Errorf("expected direct pull of the upstream ref, got %q", cmd)
			}
			return "", nil
		case strings.HasPrefix(cmd, "docker image inspect --format='{{.Id}}'"):
			return "sha256:ab\n", nil
		default:
			return "yes\n", nil
		}
	}}
	s := &Settings{Mirrors: []MirrorSettings{{Name: "dockerhub", Port: 5577}}}
	var buf strings.Builder
	res, err := SyncPrefetch(&buf, newRegistryHost(t, mock), s, []string{"quay.io/x/y:1"}, nil)
	if err != nil || res.Pulled != 1 {
		t.Fatalf("got %+v, err %v", res, err)
	}
}

// TestSyncPrefetchPullFailure pins the error contract: a failed host pull is
// reported once and skips the runner fan-out for that ref, but later refs
// still sync.
func TestSyncPrefetchPullFailure(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.HasPrefix(cmd, "docker pull ") {
			if strings.Contains(cmd, "broken") {
				return "", io.ErrUnexpectedEOF
			}
			return "", nil
		}
		if strings.HasPrefix(cmd, "docker image inspect --format='{{.Id}}'") {
			return "sha256:cd\n", nil
		}
		return "yes\n", nil
	}}
	s := &Settings{Mirrors: []MirrorSettings{
		{Name: "dockerhub", Port: 5577},
		{Name: "ghcr", Upstream: "https://ghcr.io", Port: 5578},
	}}
	var buf strings.Builder
	res, err := SyncPrefetch(&buf, newRegistryHost(t, mock), s,
		[]string{"ghcr.io/broken:1", "ghcr.io/ok:1"}, []string{"gh-sr-r-1"})
	if err == nil || !strings.Contains(err.Error(), "ghcr.io/broken") {
		t.Fatalf("expected pull error for broken ref, got %v", err)
	}
	if res.Pulled != 1 || res.Failed != 1 {
		t.Fatalf("got %+v, want pulled=1 failed=1", res)
	}
}

// TestDeployMirrorMulti pins the multi-upstream deploy contract: per-mirror
// container names, sequential ports, per-mirror storage suffixes, upstream
// auth env, and the dual gateway+loopback publish.
func TestDeployMirrorMulti(t *testing.T) {
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
	s := Settings{
		Enabled: true,
		Mirrors: []MirrorSettings{
			{Name: "dockerhub", Port: 5577},
			{Name: "ghcr", Upstream: "https://ghcr.io", Port: 5578, Username: "ci-bot", Password: "pat-token"},
		},
	}
	if err := Ensure(io.Discard, newRegistryHost(t, mock), s); err != nil {
		t.Fatalf("Ensure multi: %v", err)
	}
	all := strings.Join(mock.Calls, "\n")
	for _, want := range []string{
		"--name gh-sr-registry ",
		"--name gh-sr-registry-ghcr ",
		"-p 172.17.0.1:5577:5000",
		"-p 127.0.0.1:5577:5000",
		"-p 172.17.0.1:5578:5000",
		"-p 127.0.0.1:5578:5000",
		"-v '/root/.gh-sr/registry':/var/lib/registry",
		"-v '/root/.gh-sr/registry-ghcr':/var/lib/registry",
		"-e 'REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io'",
		"-e 'REGISTRY_PROXY_REMOTEURL=https://ghcr.io'",
		"-e 'REGISTRY_PROXY_USERNAME=ci-bot'",
		"-e 'REGISTRY_PROXY_PASSWORD=pat-token'",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in deploy calls:\n%s", want, all)
		}
	}
}

// TestSettingsFromConfigMulti pins the config resolution: the mirrors list
// reaches Settings with password_env resolved, and the legacy single-mirror
// config synthesizes exactly one Docker Hub mirror.
func TestSettingsFromConfigMulti(t *testing.T) {
	// Not parallel: t.Setenv.
	t.Setenv("GHCR_SYNC_PAT", "secret-pat")

	cfg := &config.Config{}
	cfg.RegistryMirror.Enabled = boolPtrTrue()
	s := SettingsFromConfig(cfg)
	if s == nil || len(s.Mirrors) != 1 || s.Mirrors[0].Name != config.DefaultRegistryMirrorName {
		t.Fatalf("legacy config: got %+v", s)
	}

	cfg = &config.Config{}
	cfg.RegistryMirror.Enabled = boolPtrTrue()
	cfg.RegistryMirror.Mirrors = []config.RegistryMirrorEntry{
		{Name: "hub"},
		{Name: "ghcr", Upstream: "https://ghcr.io", Port: 5578, Username: "bot", PasswordEnv: "GHCR_SYNC_PAT"},
	}
	s = SettingsFromConfig(cfg)
	if s == nil || len(s.Mirrors) != 2 {
		t.Fatalf("multi config: got %+v", s)
	}
	if s.Mirrors[1].Password != "secret-pat" {
		t.Errorf("password_env not resolved: got %q", s.Mirrors[1].Password)
	}
}

func boolPtrTrue() *bool {
	b := true
	return &b
}
