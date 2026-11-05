package registry

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
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
	if got := (Settings{Mirrors: []MirrorSettings{{Name: "dockerhub", Port: 3001}}}).EffectivePort(); got != 3001 {
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
	first := got.first()
	if !got.Enabled || first.Port != 6000 || got.BindAddr != "10.0.0.5" ||
		got.StoragePath != "/srv/mirror" || got.Image != "registry:3" ||
		first.Upstream != "https://quay.io" || got.URLOverride != "http://override:9999" {
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

	s := Settings{Mirrors: []MirrorSettings{{Name: "dockerhub", Port: 4000}}}
	if got := s.localURL(0, "172.17.0.1"); got != "http://172.17.0.1:4000" {
		t.Errorf("gateway + auto bind: got %q", got)
	}

	s.BindAddr = "0.0.0.0"
	if got := s.localURL(0, "172.17.0.1"); got != "http://172.17.0.1:4000" {
		t.Errorf("0.0.0.0 + gateway: got %q", got)
	}
	if got := s.localURL(0, ""); got != "http://127.0.0.1:4000" {
		t.Errorf("0.0.0.0 + no gateway: got %q", got)
	}

	s.BindAddr = "192.168.1.5"
	if got := s.localURL(0, "172.17.0.1"); got != "http://192.168.1.5:4000" {
		t.Errorf("explicit bind wins over gateway: got %q", got)
	}

	// Auto bind + no gateway → loopback.
	s.BindAddr = ""
	if got := s.localURL(0, ""); got != "http://127.0.0.1:4000" {
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
	if got, err := s.mirrorStoragePath(0, h); err == nil || got != "" {
		t.Errorf("echo failure: got %q, err %v", got, err)
	}

	// echo $HOME empty.
	h = newRegistryHost(t, &testutil.MockExecutor{Output: "\n"})
	s = Settings{StoragePath: "$HOME/mirror"}
	if got, err := s.mirrorStoragePath(0, h); err == nil || got != "" {
		t.Errorf("empty $HOME: got %q, err %v", got, err)
	}

	// $HOME prefix with no suffix → expands to bare home (covers edge case).
	h = newRegistryHost(t, &testutil.MockExecutor{Output: "/home/op\n"})
	s = Settings{StoragePath: "$HOME"}
	if got, err := s.mirrorStoragePath(0, h); err != nil || got != "/home/op" {
		t.Errorf("$HOME bare: got %q, err %v", got, err)
	}
}

// ----------------------------------------------------------------------------
// Ensure orchestrator: error branches
// ----------------------------------------------------------------------------

// Ensure must surface the containerState failure itself. Only the state
// inspect fails here — every later command succeeds — so swallowing that error
// makes Ensure fall through to deploy and return nil, which this test catches.
// A blanket RunErr would instead let a later failure satisfy a bare err check.
func TestEnsure_containerStateError(t *testing.T) {
	t.Parallel()
	stateErr := errors.New("inspect failed")
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, ".State.Status") {
			return "", stateErr
		}
		return "", nil
	}}
	err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true})
	if !errors.Is(err, stateErr) {
		t.Fatalf("expected containerState error %v, got %v", stateErr, err)
	}
}

// Same discipline as above: fail only the label inspect and let the rest of
// the deploy path succeed, so a swallowed layout error surfaces as Ensure
// returning nil rather than as an unrelated downstream error.
func TestEnsure_layoutCurrentError(t *testing.T) {
	t.Parallel()
	layoutErr := errors.New("layout inspect failed")
	calls := 0
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, ".State.Status"):
			return "running\n", nil
		case strings.Contains(cmd, "Config.Labels"):
			calls++
			return "", layoutErr
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	err := Ensure(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true})
	if !errors.Is(err, layoutErr) {
		t.Fatalf("expected layout inspect error %v, got %v", layoutErr, err)
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
	if err := deployMirror(&buf, newRegistryHost(t, mock), Settings{Enabled: true, BindAddr: "10.0.0.5", Mirrors: []MirrorSettings{{Name: "dockerhub", Port: 6000}}}, 0); err != nil {
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
	if err := deployMirror(&buf, newRegistryHost(t, mock), Settings{Enabled: true}, 0); err != nil {
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
	err := deployMirror(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}, 0)
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
	err := deployMirror(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}, 0)
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
	if err := deployMirror(io.Discard, newRegistryHost(t, mock), Settings{Enabled: true}, 0); err != nil {
		t.Fatalf("deploy with failing pull: %v", err)
	}
	if !ran {
		t.Errorf("expected docker run despite failing pull, calls: %v", mock.Calls)
	}
}

// ----------------------------------------------------------------------------
// Inspect
// ----------------------------------------------------------------------------

// inspectBatchOut renders the batched stream the remote Inspect script emits
// for one mirror: the section marker, the inspect state|image line, and —
// only when the container exists — the health and du tags.
func inspectBatchOut(n int, stateImage, health, du string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "@@m:%d\n", n)
	fmt.Fprintf(&b, "@@st:%d|%s\n", n, stateImage)
	if stateImage != "" {
		fmt.Fprintf(&b, "@@health:%d|%s\n", n, health)
		fmt.Fprintf(&b, "@@du:%d|%s\n", n, du)
	}
	return b.String()
}

// threeMirrorSettings is a direct 3-mirror Settings (no config round-trip) so
// multi-mirror tests don't depend on SettingsFromConfig's defaults.
func threeMirrorSettings() Settings {
	return Settings{Enabled: true, Mirrors: []MirrorSettings{
		{Name: "dockerhub", Upstream: DefaultUpstream, Port: DefaultPort},
		{Name: "ghcr", Upstream: "https://ghcr.io", Port: 5578},
		{Name: "quay", Upstream: "https://quay.io", Port: 5579},
	}}
}

func TestInspect_runningHealthy(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			return inspectBatchOut(1, "running|registry:2", "ok", "1234567"), nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	infos, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil || len(infos) != 1 {
		t.Fatalf("Inspect: len=%d err=%v", len(infos), err)
	}
	info := infos[0].Info
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
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			return inspectBatchOut(1, "", "", ""), nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	infos, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil || len(infos) != 1 {
		t.Fatalf("Inspect: len=%d err=%v", len(infos), err)
	}
	info := infos[0].Info
	if info.State != "" {
		t.Errorf("State: got %q", info.State)
	}
	if info.Healthy {
		t.Errorf("absent container must report Healthy=false")
	}
	if info.StorageBytes != 0 {
		t.Errorf("absent container must report StorageBytes=0, got %d", info.StorageBytes)
	}
}

func TestInspect_unhealthyProbe(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			return inspectBatchOut(1, "running|registry:2", "fail", "500"), nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	infos, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil || len(infos) != 1 {
		t.Fatalf("Inspect: len=%d err=%v", len(infos), err)
	}
	if infos[0].Info.Healthy {
		t.Errorf("expected Healthy=false when the /v2/ probe fails")
	}
}

// TestInspect_healthProbeCapturesCurlExit pins the health semantics: the
// remote script branches on curl's own exit status (`curl ... >/dev/null
// 2>&1; then echo ... ok ... fail`) instead of appending `|| true`. The old
// per-mirror probe masked a failed curl behind a 0 shell exit, so Healthy
// reported SSH-transport success rather than the mirror answering /v2/ —
// doctor's "mirror healthy" line was true for a dead mirror.
func TestInspect_healthProbeCapturesCurlExit(t *testing.T) {
	t.Parallel()
	var batchCmd string
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			batchCmd = cmd
			return inspectBatchOut(1, "running|registry:2", "ok", "1"), nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	if _, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true}); err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !strings.Contains(batchCmd, `/v2/ >/dev/null 2>&1; then echo "@@health:1|ok"`) ||
		!strings.Contains(batchCmd, `else echo "@@health:1|fail"`) {
		t.Errorf("health probe must branch on curl's exit status:\n%s", batchCmd)
	}
	if strings.Contains(batchCmd, "/v2/ 2>/dev/null || true") {
		t.Errorf("health probe must not mask curl failures with `|| true`:\n%s", batchCmd)
	}
}

func TestInspect_multiMirrorBatched(t *testing.T) {
	t.Parallel()
	out := inspectBatchOut(1, "running|registry:2", "ok", "111") +
		inspectBatchOut(2, "exited|registry:2", "", "") +
		inspectBatchOut(3, "", "", "")
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			return out, nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	infos, err := Inspect(newRegistryHost(t, mock), threeMirrorSettings())
	if err != nil || len(infos) != 3 {
		t.Fatalf("Inspect: len=%d err=%v", len(infos), err)
	}
	if infos[0].Info.State != "running" || !infos[0].Info.Healthy || infos[0].Info.StorageBytes != 111 {
		t.Errorf("mirror 1: %+v", infos[0].Info)
	}
	if infos[1].Info.State != "exited" || infos[1].Info.Healthy {
		t.Errorf("mirror 2: exited without a health tag must be unhealthy: %+v", infos[1].Info)
	}
	if infos[2].Info.State != "" || infos[2].Info.Healthy || infos[2].Info.StorageBytes != 0 {
		t.Errorf("mirror 3: absent must stay clean-zero: %+v", infos[2].Info)
	}
	if infos[1].Container != ContainerNamePrefix+"ghcr" || infos[2].Container != ContainerNamePrefix+"quay" {
		t.Errorf("containers: %q %q", infos[1].Container, infos[2].Container)
	}
	// Three mirrors, one probe: gateway + storage resolve + the batch.
	if len(mock.Calls) != 3 {
		t.Errorf("round-trips: got %d calls, want 3 (gateway + $HOME + batch): %q", len(mock.Calls), mock.Calls)
	}
}

func TestInspect_duGarbage(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			return inspectBatchOut(1, "running|registry:2", "ok", "not a number"), nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	infos, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil || len(infos) != 1 {
		t.Fatalf("Inspect: len=%d err=%v", len(infos), err)
	}
	info := infos[0].Info
	if info.StorageBytes != 0 {
		t.Errorf("expected StorageBytes=0 for garbage output, got %d", info.StorageBytes)
	}
}

func TestInspect_storageResolveError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
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

func TestInspect_urlOverrideSkipsGateway(t *testing.T) {
	t.Parallel()
	calls := 0
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			return inspectBatchOut(1, "running|registry:2", "ok", "1234567"), nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			calls++
			return gwOutput, nil
		}
		return "", nil
	}}
	infos, err := Inspect(newRegistryHost(t, mock), Settings{Enabled: true, URLOverride: "http://override:9999"})
	if err != nil || len(infos) != 1 {
		t.Fatalf("Inspect: len=%d err=%v", len(infos), err)
	}
	info := infos[0].Info
	if info.URL != "http://override:9999" {
		t.Errorf("URL override: got %q", info.URL)
	}
	if calls != 0 {
		t.Errorf("URL override must skip gateway lookup, got %d calls", calls)
	}
}

// BenchmarkInspectRoundTrips pins Inspect's per-host round-trip cost at 3
// (gateway resolve + storage resolve + one batched probe) regardless of the
// mirror count. Re-splitting the probes into per-mirror round-trips would
// multiply `gh sr registry status` / doctor's remote I/O by the mirror count,
// and this metric is what would show it.
func BenchmarkInspectRoundTrips(b *testing.B) {
	out := inspectBatchOut(1, "running|registry:2", "ok", "111") +
		inspectBatchOut(2, "running|registry:2", "ok", "222") +
		inspectBatchOut(3, "exited|registry:2", "fail", "333")
	mock := &testutil.RoundTripCounter{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "@@m:1"):
			return out, nil
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "ip -4 -o addr show docker0"):
			return gwOutput, nil
		}
		return "", nil
	}}
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock.Reset()
		if _, err := Inspect(h, threeMirrorSettings()); err != nil {
			b.Fatalf("Inspect: %v", err)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(mock.RoundTrips), "round-trips/op")
}

// ----------------------------------------------------------------------------
// MirrorStorages
// ----------------------------------------------------------------------------

// mirrorStorageMock answers the du call with `<bytes>\t<path>` lines, the
// shape MirrorStorages parses (one line per directory, matched back by path).
func duOutput(total int, path string) string {
	return fmt.Sprintf("%d\t%s\n", total, path)
}

func TestMirrorStorages_success(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "echo $HOME"):
			return "/root\n", nil
		case strings.Contains(cmd, "du -sb"):
			return duOutput(9876543, "/root/.gh-sr/registry"), nil
		}
		return "", nil
	}}
	stores, err := MirrorStorages(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("MirrorStorages: %v", err)
	}
	if len(stores) != 1 {
		t.Fatalf("want 1 entry, got %d", len(stores))
	}
	if stores[0].Container != ContainerName {
		t.Errorf("container: got %q", stores[0].Container)
	}
	if stores[0].Path != "/root/.gh-sr/registry" {
		t.Errorf("path: got %q", stores[0].Path)
	}
	if stores[0].Bytes != 9876543 {
		t.Errorf("bytes: got %d", stores[0].Bytes)
	}
}

func TestMirrorStorages_duFails(t *testing.T) {
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
	stores, err := MirrorStorages(newRegistryHost(t, mock), Settings{Enabled: true})
	if err == nil {
		t.Fatal("expected error when du fails")
	}
	// The resolved paths survive the du failure so the caller can still show
	// them alongside the error.
	if len(stores) != 1 || stores[0].Path != "/root/.gh-sr/registry" {
		t.Errorf("path still resolved: %+v", stores)
	}
	if stores[0].Bytes != 0 {
		t.Errorf("bytes: got %d", stores[0].Bytes)
	}
}

func TestMirrorStorages_garbageBytes(t *testing.T) {
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
	stores, err := MirrorStorages(newRegistryHost(t, mock), Settings{Enabled: true})
	if err != nil {
		t.Fatalf("MirrorStorages: %v", err)
	}
	if stores[0].Path != "/root/.gh-sr/registry" || stores[0].Bytes != 0 {
		t.Errorf("path=%q bytes=%d err=%v", stores[0].Path, stores[0].Bytes, err)
	}
}

func TestMirrorStorages_resolveError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunErr: errors.New("ssh down")}
	stores, err := MirrorStorages(newRegistryHost(t, mock), Settings{Enabled: true, StoragePath: "$HOME/m"})
	if err == nil {
		t.Fatal("expected error when $HOME echo fails")
	}
	if stores != nil {
		t.Errorf("expected no entries on resolve failure, got %+v", stores)
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
	// purgeData=true is what reaches resolvedStoragePath at all (Remove returns
	// early otherwise); a $HOME-prefixed StoragePath is what makes that resolve
	// shell out to `echo $HOME`, which is the call failing here.
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
