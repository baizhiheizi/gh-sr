package registry

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// ----------------------------------------------------------------------------
// Pure / config helpers
// ----------------------------------------------------------------------------

func TestEffectivePort(t *testing.T) {
	t.Parallel()
	if got := (Settings{}).EffectivePort(); got != DefaultPort {
		t.Errorf("zero-value: got %d, want %d", got, DefaultPort)
	}
	if got := (Settings{Port: 3001}).EffectivePort(); got != 3001 {
		t.Errorf("explicit port: got %d, want 3001", got)
	}
}

func TestSettingsFromConfig(t *testing.T) {
	t.Parallel()

	// nil config → nil.
	if got := SettingsFromConfig(nil); got != nil {
		t.Errorf("nil config: got %+v, want nil", got)
	}

	// Disabled mirror (Enabled=false) → nil.
	disabled := false
	cfg := &config.Config{RegistryMirror: config.RegistryMirrorConfig{Enabled: &disabled}}
	if got := SettingsFromConfig(cfg); got != nil {
		t.Errorf("disabled: got %+v, want nil", got)
	}

	// Enabled mirror → every field copied verbatim.
	enabled := true
	cfg = &config.Config{RegistryMirror: config.RegistryMirrorConfig{
		Enabled:     &enabled,
		Port:        6000,
		BindAddr:    "10.0.0.5",
		StoragePath: "/srv/mirror",
		Image:       "registry:3",
		Upstream:    "https://quay.io",
		URLOverride: "http://override:9999",
	}}
	got := SettingsFromConfig(cfg)
	if got == nil {
		t.Fatalf("enabled: got nil")
	}
	if !got.Enabled || got.Port != 6000 || got.BindAddr != "10.0.0.5" ||
		got.StoragePath != "/srv/mirror" || got.Image != "registry:3" ||
		got.Upstream != "https://quay.io" || got.URLOverride != "http://override:9999" {
		t.Errorf("fields not copied: %+v", got)
	}

	// Enabled with nil Enabled pointer (not in defaults): treat as disabled.
	cfg = &config.Config{RegistryMirror: config.RegistryMirrorConfig{}}
	if got := SettingsFromConfig(cfg); got != nil {
		t.Errorf("nil Enabled pointer: got %+v, want nil", got)
	}
}

// ----------------------------------------------------------------------------
// localURL: bind precedence + gateway/loopback fallback
// ----------------------------------------------------------------------------

func TestLocalURL(t *testing.T) {
	t.Parallel()

	s := Settings{Port: 4000}
	if got := s.localURL("172.17.0.1"); got != "http://172.17.0.1:4000" {
		t.Errorf("gateway + auto bind: got %q", got)
	}

	s.BindAddr = "0.0.0.0"
	if got := s.localURL("172.17.0.1"); got != "http://172.17.0.1:4000" {
		t.Errorf("0.0.0.0 + gateway: got %q", got)
	}
	if got := s.localURL(""); got != "http://127.0.0.1:4000" {
		t.Errorf("0.0.0.0 + no gateway: got %q", got)
	}

	s.BindAddr = "192.168.1.5"
	if got := s.localURL("172.17.0.1"); got != "http://192.168.1.5:4000" {
		t.Errorf("explicit bind wins over gateway: got %q", got)
	}

	// Auto bind + no gateway → loopback.
	s.BindAddr = ""
	if got := s.localURL(""); got != "http://127.0.0.1:4000" {
		t.Errorf("auto + no gateway: got %q", got)
	}
}

// ----------------------------------------------------------------------------
// resolvedStoragePath: $HOME failure paths
// ----------------------------------------------------------------------------

func TestResolvedStoragePath_errors(t *testing.T) {
	t.Parallel()

	// echo $HOME fails.
	h := newRegistryHost(t, &testutil.MockExecutor{RunErr: errors.New("ssh down")})
	s := Settings{StoragePath: "$HOME/mirror"}
	if got, err := s.resolvedStoragePath(h); err == nil || got != "" {
		t.Errorf("echo failure: got %q, err %v", got, err)
	}

	// echo $HOME empty.
	h = newRegistryHost(t, &testutil.MockExecutor{Output: "\n"})
	s = Settings{StoragePath: "$HOME/mirror"}
	if got, err := s.resolvedStoragePath(h); err == nil || got != "" {
		t.Errorf("empty $HOME: got %q, err %v", got, err)
	}

	// $HOME prefix with no suffix → expands to bare home (covers edge case).
	h = newRegistryHost(t, &testutil.MockExecutor{Output: "/home/op\n"})
	s = Settings{StoragePath: "$HOME"}
	if got, err := s.resolvedStoragePath(h); err != nil || got != "/home/op" {
		t.Errorf("$HOME bare: got %q, err %v", got, err)
	}
}

// ----------------------------------------------------------------------------
// Ensure orchestrator: error branches
// ----------------------------------------------------------------------------

func TestEnsure_containerStateError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunErr: errors.New("inspect failed")}
	if err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}); err == nil {
		t.Fatal("expected error when containerState fails")
	}
}

func TestEnsure_layoutCurrentError(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, ".State.Status") {
			return "running\n", nil
		}
		if strings.Contains(cmd, "Config.Labels") {
			calls++
			return "", errors.New("layout inspect failed")
		}
		return "", nil
	}}
	if err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}); err == nil {
		t.Fatal("expected error when registryLayoutCurrent fails")
	}
	if calls != 1 {
		t.Errorf("expected 1 layout inspect call, got %d", calls)
	}
}

func TestEnsure_recreateRmError(t *testing.T) {
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
	err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "removing outdated registry mirror container") {
		t.Errorf("expected rm error, got %v", err)
	}
}

func TestEnsure_startError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "Config.Labels"):
			return registryLayoutRev + "\n", nil
		case strings.Contains(cmd, ".State.Status"):
			return "exited\n", nil
		case strings.Contains(cmd, "docker start"):
			return "", errors.New("start denied")
		}
		return "", nil
	}}
	err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "starting registry mirror container") {
		t.Errorf("expected start error, got %v", err)
	}
}

// ----------------------------------------------------------------------------
// deploy: gateway-fail fallback, explicit bind, mkdir error, run error
// ----------------------------------------------------------------------------

func TestDeploy_explicitBindAddr(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			calls++
			return gwOutput, nil
		case strings.Contains(cmd, "mkdir -p"):
			return "", nil
		case strings.Contains(cmd, "docker pull"):
			return "", nil
		case strings.Contains(cmd, "docker run"):
			return "", nil
		}
		return "", nil
	}}
	var buf strings.Builder
	if err := deploy(&buf, newRegistryHost(t, mock), Settings{Enabled: true, BindAddr: "10.0.0.5", Port: 6000}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if calls != 0 {
		t.Errorf("explicit BindAddr must skip gateway lookup, got %d calls", calls)
	}
	runLine := lastCallContaining(mock.Calls, "docker run")
	if runLine == "" {
		t.Fatalf("no docker run, calls: %v", mock.Calls)
	}
	if !strings.Contains(runLine, "-p 10.0.0.5:6000:5000") {
		t.Errorf("expected explicit bind in -p, got: %s", runLine)
	}
	if strings.Contains(buf.String(), "docker0 gateway not found") {
		t.Errorf("explicit bind must not print gateway warning: %s", buf.String())
	}
}

func TestDeploy_gatewayMissingBindsAll(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return "device not found\n", nil
		case strings.Contains(cmd, "mkdir -p"):
			return "", nil
		case strings.Contains(cmd, "docker pull"):
			return "", nil
		case strings.Contains(cmd, "docker run"):
			return "", nil
		}
		return "", nil
	}}
	if err := deploy(&buf, newRegistryHost(t, mock), Settings{Enabled: true}); err != nil {
		t.Fatalf("deploy: %v", err)
	}
	if !strings.Contains(buf.String(), "docker0 gateway not found") {
		t.Errorf("expected gateway warning, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "binding 0.0.0.0") {
		t.Errorf("expected 0.0.0.0 warning, got: %s", buf.String())
	}
	runLine := lastCallContaining(mock.Calls, "docker run")
	if !strings.Contains(runLine, "-p 0.0.0.0:5577:5000") {
		t.Errorf("expected 0.0.0.0 bind in -p, got: %s", runLine)
	}
}

func TestDeploy_mkdirError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "mkdir -p"):
			return "", errors.New("read-only fs")
		}
		return "", nil
	}}
	err := deploy(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "creating registry mirror storage dir") {
		t.Errorf("expected mkdir error, got %v", err)
	}
}

func TestDeploy_runError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "mkdir -p"):
			return "", nil
		case strings.Contains(cmd, "docker pull"):
			return "", nil
		case strings.Contains(cmd, "docker run"):
			return "", errors.New("port in use")
		}
		return "", nil
	}}
	err := deploy(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "starting registry mirror container") {
		t.Errorf("expected run error, got %v", err)
	}
}

func TestDeploy_pullErrorIgnored(t *testing.T) {
	t.Parallel()
	// docker pull is best-effort — even if it fails, deploy must succeed if docker run succeeds.
	ran := false
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "mkdir -p"):
			return "", nil
		case strings.Contains(cmd, "docker pull"):
			return "", errors.New("no network")
		case strings.Contains(cmd, "docker run"):
			ran = true
			return "", nil
		}
		return "", nil
	}}
	if err := deploy(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}); err != nil {
		t.Fatalf("deploy with failing pull: %v", err)
	}
	if !ran {
		t.Errorf("expected docker run despite failing pull, calls: %v", mock.Calls)
	}
}

// ----------------------------------------------------------------------------
// Inspect
// ----------------------------------------------------------------------------

func TestInspect_runningHealthy(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status}}"):
			return "running|registry:2\n", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "curl -fsS"):
			return "", nil
		case strings.Contains(cmd, "du -sb"):
			return "1234567\n", nil
		}
		return "", nil
	}}
	info, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.State != "running" || info.Image != "registry:2" {
		t.Errorf("state/image: %+v", info)
	}
	if !info.Healthy {
		t.Errorf("expected Healthy=true")
	}
	if info.StorageBytes != 1234567 {
		t.Errorf("StorageBytes: got %d", info.StorageBytes)
	}
	if !strings.HasPrefix(info.URL, "http://172.17.0.1:") {
		t.Errorf("URL: %q", info.URL)
	}
}

func TestInspect_absentNoHealthProbe(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status}}"):
			return "", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "curl -fsS"):
			calls++
			return "", nil
		case strings.Contains(cmd, "du -sb"):
			return "", errors.New("no dir")
		}
		return "", nil
	}}
	info, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.State != "" {
		t.Errorf("State: got %q", info.State)
	}
	if calls != 0 {
		t.Errorf("absent container must skip health probe, got %d calls", calls)
	}
	if info.Healthy {
		t.Errorf("absent container must report Healthy=false")
	}
}

func TestInspect_unhealthyProbe(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status}}"):
			return "running|registry:2\n", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "curl -fsS"):
			return "", errors.New("connection refused")
		}
		return "", nil
	}}
	info, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.Healthy {
		t.Errorf("expected Healthy=false when curl fails")
	}
}

func TestInspect_storageResolveError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status}}"):
			return "running|registry:2\n", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "", errors.New("ssh gone")
		}
		return "", nil
	}}
	_, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true, StoragePath: "$HOME/mirror"})
	if err == nil {
		t.Fatal("expected error when storage resolve fails")
	}
}

func TestInspect_duGarbage(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status}}"):
			return "running|registry:2\n", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		case strings.Contains(cmd, "curl -fsS"):
			return "", nil
		case strings.Contains(cmd, "du -sb"):
			return "not a number\n", nil
		}
		return "", nil
	}}
	info, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.StorageBytes != 0 {
		t.Errorf("expected StorageBytes=0 for garbage output, got %d", info.StorageBytes)
	}
}

func TestInspect_urlOverrideSkipsGateway(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status}}"):
			return "running|registry:2\n", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			calls++
			return gwOutput, nil
		case strings.Contains(cmd, "curl -fsS"):
			return "", nil
		}
		return "", nil
	}}
	info, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true, URLOverride: "http://override:9999"})
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if info.URL != "http://override:9999" {
		t.Errorf("URL override: got %q", info.URL)
	}
	if calls != 0 {
		t.Errorf("URL override must skip gateway lookup, got %d calls", calls)
	}
}

// ----------------------------------------------------------------------------
// MeasureStorage
// ----------------------------------------------------------------------------

func TestMeasureStorage_success(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "du -sb"):
			return "9876543\n", nil
		}
		return "", nil
	}}
	storage, bytes, err := MeasureStorage(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("MeasureStorage: %v", err)
	}
	if storage != "/root/.gh-sr/registry" {
		t.Errorf("storage: got %q", storage)
	}
	if bytes != 9876543 {
		t.Errorf("bytes: got %d", bytes)
	}
}

func TestMeasureStorage_duFails(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "du -sb"):
			return "", errors.New("dir missing")
		}
		return "", nil
	}}
	storage, bytes, err := MeasureStorage(newRegistryHost(t, mock), Settings{Enabled: true})
	if err == nil {
		t.Fatal("expected error when du fails")
	}
	if storage != "/root/.gh-sr/registry" {
		t.Errorf("storage still resolved: got %q", storage)
	}
	if bytes != 0 {
		t.Errorf("bytes: got %d", bytes)
	}
}

func TestMeasureStorage_garbageBytes(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "du -sb"):
			return "garbage\n", nil
		}
		return "", nil
	}}
	storage, bytes, err := MeasureStorage(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("MeasureStorage: %v", err)
	}
	if storage != "/root/.gh-sr/registry" || bytes != 0 {
		t.Errorf("storage=%q bytes=%d err=%v", storage, bytes, err)
	}
}

func TestMeasureStorage_resolveError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunErr: errors.New("ssh down")}
	storage, bytes, err := MeasureStorage(newRegistryHost(t, mock), Settings{Enabled: true, StoragePath: "$HOME/m"})
	if err == nil {
		t.Fatal("expected error when $HOME echo fails")
	}
	if storage != "" || bytes != 0 {
		t.Errorf("expected zero storage/bytes on resolve failure, got %q/%d", storage, bytes)
	}
}

// ----------------------------------------------------------------------------
// Remove: extra branches
// ----------------------------------------------------------------------------

func TestRemove_keepData(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "echo $HOME") {
			return "", errors.New("must not probe home")
		}
		return "", nil
	}}
	if err := Remove(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}, false); err != nil {
		t.Fatalf("Remove keepData: %v", err)
	}
	if lastCallContaining(mock.Calls, "rm -rf") != "" {
		t.Errorf("keepData must not run rm -rf, calls: %v", mock.Calls)
	}
}

func TestRemove_rmError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "docker rm -f") {
			return "", errors.New("locked")
		}
		return "", nil
	}}
	err := Remove(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}, false)
	if err == nil || !strings.Contains(err.Error(), "removing registry mirror container") {
		t.Errorf("expected rm error, got %v", err)
	}
}

func TestRemove_purgeStorageError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker rm -f"):
			return "", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "rm -rf"):
			return "", errors.New("read-only fs")
		}
		return "", nil
	}}
	err := Remove(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}, true)
	if err == nil || !strings.Contains(err.Error(), "removing registry mirror storage") {
		t.Errorf("expected storage purge error, got %v", err)
	}
}

func TestRemove_purgeResolveError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker rm -f"):
			return "", nil
		case strings.Contains(cmd, "echo $HOME"):
			return "", errors.New("ssh gone")
		}
		return "", nil
	}}
	// purgeData=true with $HOME-prefixed StoragePath exercises the resolve path
	// (Remove with keepData=true and default StoragePath would still skip it).
	err := Remove(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true, StoragePath: "$HOME/m"}, true)
	if err == nil {
		t.Fatal("expected resolve error")
	}
}

func lastCallContaining(calls []string, substr string) string {
	for i := len(calls) - 1; i >= 0; i-- {
		if strings.Contains(calls[i], substr) {
			return calls[i]
		}
	}
	return ""
}
