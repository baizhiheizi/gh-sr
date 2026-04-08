package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerConfig_EffectiveMode(t *testing.T) {
	t.Parallel()
	rc := RunnerConfig{}
	if got := rc.EffectiveMode("linux"); got != "docker" {
		t.Errorf("linux default: got %q want docker", got)
	}
	if got := rc.EffectiveMode("darwin"); got != "native" {
		t.Errorf("darwin default: got %q want native", got)
	}
	if got := rc.EffectiveMode("windows"); got != "native" {
		t.Errorf("windows default: got %q want native", got)
	}
	rc.Mode = "native"
	if got := rc.EffectiveMode("linux"); got != "native" {
		t.Errorf("explicit mode: got %q want native", got)
	}
	rc.Mode = "docker"
	if got := rc.EffectiveMode("windows"); got != "docker" {
		t.Errorf("explicit docker on windows: got %q want docker", got)
	}
}

func TestRunnerConfig_EffectiveDockerNetworkMode(t *testing.T) {
	t.Parallel()
	rc := RunnerConfig{Mode: "docker", DockerNetworkMode: "host"}
	if got := rc.EffectiveDockerNetworkMode("linux"); got != "host" {
		t.Errorf("docker host: got %q want host", got)
	}
	rc.DockerNetworkMode = "bridge"
	if got := rc.EffectiveDockerNetworkMode("linux"); got != "bridge" {
		t.Errorf("explicit bridge: got %q want bridge", got)
	}
	rc.DockerNetworkMode = ""
	if got := rc.EffectiveDockerNetworkMode("linux"); got != "bridge" {
		t.Errorf("empty: got %q want bridge", got)
	}
	rc.Mode = "native"
	rc.DockerNetworkMode = "host"
	if got := rc.EffectiveDockerNetworkMode("linux"); got != "bridge" {
		t.Errorf("native mode ignores host: got %q want bridge", got)
	}
}

func TestRunnerConfig_InstanceNames(t *testing.T) {
	t.Parallel()
	rc := RunnerConfig{Name: "ci", Count: 0}
	names := rc.InstanceNames()
	if len(names) != 1 || names[0] != "ci-1" {
		t.Fatalf("count<1: got %v", names)
	}
	rc.Count = 3
	names = rc.InstanceNames()
	want := []string{"ci-1", "ci-2", "ci-3"}
	if len(names) != len(want) {
		t.Fatalf("got %v want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("[%d]: got %q want %q", i, names[i], want[i])
		}
	}
}

func TestLoad_resolveEnv(t *testing.T) {
	t.Setenv("GHR_TEST_PAT", "secret-from-env")
	dir := t.TempDir()
	path := filepath.Join(dir, "runners.yml")
	content := `
github:
  pat: env:GHR_TEST_PAT
hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHub.PAT != "secret-from-env" {
		t.Errorf("PAT: got %q", cfg.GitHub.PAT)
	}
}

func TestLoad_applyDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runners.yml")
	content := `
github:
  pat: tok
hosts:
  h1:
    addr: a@b
    os: linux
    arch: amd64
runners:
  - name: r1
    repo: o/r
    host: h1
    count: 0
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Runners[0].Count != 1 {
		t.Errorf("count default: got %d", cfg.Runners[0].Count)
	}
}

func TestValidate_errors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  Config
		frag string
	}{
		{
			name: "no_hosts",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h"}},
			},
			frag: "at least one host",
		},
		{
			name: "host_addr",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h"}},
			},
			frag: `host "h"`,
		},
		{
			name: "host_os_invalid",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "freebsd", Arch: "amd64"}},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h"}},
			},
			frag: "os must be linux",
		},
		{
			name: "host_arch_invalid",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "riscv64"}},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h"}},
			},
			frag: "arch must be amd64",
		},
		{
			name: "no_runners",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: nil,
			},
			frag: "at least one runner",
		},
		{
			name: "runner_name",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{Name: "", Repo: "o/r", Host: "h"}},
			},
			frag: "runner name is required",
		},
		{
			name: "runner_repo",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{Name: "r", Repo: "", Host: "h"}},
			},
			frag: "repo is required",
		},
		{
			name: "runner_host_missing",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "unknown"}},
			},
			frag: "not found in hosts",
		},
		{
			name: "bad_mode",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h", Mode: "k8s"}},
			},
			frag: "mode must be",
		},
		{
			name: "windows_ps_on_linux",
			cfg: Config{
				GitHub: GitHubConfig{PAT: "x"},
				Hosts: map[string]HostConfig{
					"h": {Addr: "a@b", OS: "linux", Arch: "amd64", WindowsPS: "pwsh"},
				},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h"}},
			},
			frag: "windows_ps is only valid",
		},
		{
			name: "windows_ps_invalid_value",
			cfg: Config{
				GitHub: GitHubConfig{PAT: "x"},
				Hosts: map[string]HostConfig{
					"w": {Addr: "a@b", OS: "windows", Arch: "amd64", WindowsPS: "bash"},
				},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "w"}},
			},
			frag: "windows_ps must be powershell or pwsh",
		},
		{
			name: "docker_network_mode_invalid",
			cfg: Config{
				GitHub: GitHubConfig{PAT: "x"},
				Hosts:  map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{
					Name: "r", Repo: "o/r", Host: "h", Mode: "docker", DockerNetworkMode: "overlay",
				}},
			},
			frag: "docker_network_mode must be",
		},
		{
			name: "docker_network_mode_with_native",
			cfg: Config{
				GitHub: GitHubConfig{PAT: "x"},
				Hosts:  map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{
					Name: "r", Repo: "o/r", Host: "h", Mode: "native", DockerNetworkMode: "bridge",
				}},
			},
			frag: "docker_network_mode applies only when mode is docker",
		},
		{
			name: "docker_cap_add_with_native",
			cfg: Config{
				GitHub: GitHubConfig{PAT: "x"},
				Hosts:  map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{
					Name: "r", Repo: "o/r", Host: "h", Mode: "native", DockerCapAdd: []string{"NET_ADMIN"},
				}},
			},
			frag: "docker_cap_add applies only when mode is docker",
		},
		{
			name: "docker_cap_add_invalid_char",
			cfg: Config{
				GitHub: GitHubConfig{PAT: "x"},
				Hosts:  map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{
					Name: "r", Repo: "o/r", Host: "h", Mode: "docker", DockerCapAdd: []string{"NET-ADMIN"},
				}},
			},
			frag: "docker_cap_add invalid capability",
		},
		{
			name: "docker_cap_add_empty_string",
			cfg: Config{
				GitHub: GitHubConfig{PAT: "x"},
				Hosts:  map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{{
					Name: "r", Repo: "o/r", Host: "h", Mode: "docker", DockerCapAdd: []string{"NET_ADMIN", ""},
				}},
			},
			frag: "docker_cap_add contains an empty entry",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cfg.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.frag) {
				t.Errorf("error %q should contain %q", err.Error(), tc.frag)
			}
		})
	}
}

func TestDefaultLabels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode, os, arch string
		want           []string
	}{
		{"docker", "linux", "amd64", []string{"self-hosted", "Linux", "X64"}},
		{"docker", "darwin", "arm64", []string{"self-hosted", "Linux", "ARM64"}},
		{"docker", "windows", "amd64", []string{"self-hosted", "Linux", "X64"}},
		{"native", "linux", "amd64", []string{"self-hosted", "Linux", "X64"}},
		{"native", "darwin", "arm64", []string{"self-hosted", "macOS", "ARM64"}},
		{"native", "windows", "amd64", []string{"self-hosted", "Windows", "X64"}},
	}
	for _, tc := range cases {
		got := DefaultLabels(tc.mode, tc.os, tc.arch)
		if len(got) != len(tc.want) {
			t.Errorf("DefaultLabels(%s,%s,%s): got %v want %v", tc.mode, tc.os, tc.arch, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("DefaultLabels(%s,%s,%s)[%d]: got %q want %q", tc.mode, tc.os, tc.arch, i, got[i], tc.want[i])
			}
		}
	}
}

func TestEffectiveLabels(t *testing.T) {
	t.Parallel()
	rc := RunnerConfig{Name: "r", Labels: []string{"custom"}}
	if got := rc.EffectiveLabels("linux", "amd64"); len(got) != 1 || got[0] != "custom" {
		t.Errorf("explicit labels should be used: got %v", got)
	}
	rc2 := RunnerConfig{Name: "r"}
	got := rc2.EffectiveLabels("linux", "amd64")
	if len(got) != 3 || got[0] != "self-hosted" || got[1] != "Linux" || got[2] != "X64" {
		t.Errorf("empty labels should be auto-generated: got %v", got)
	}
}

func TestValidate_emptyOSArchRemoteHost(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub:  GitHubConfig{PAT: "x"},
		Hosts:   map[string]HostConfig{"h": {Addr: "user@host"}},
		Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty os/arch on remote host should be valid (auto-detected at runtime): %v", err)
	}
}

func TestNeedsDetection(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Hosts: map[string]HostConfig{
			"h1": {Addr: "a@b", OS: "linux", Arch: "amd64"},
		},
	}
	if cfg.NeedsDetection() {
		t.Error("all hosts have os/arch, should not need detection")
	}
	cfg.Hosts["h2"] = HostConfig{Addr: "c@d"}
	if !cfg.NeedsDetection() {
		t.Error("h2 has no os/arch, should need detection")
	}
	cfg.Hosts["local"] = HostConfig{Addr: "local"}
	if !cfg.NeedsDetection() {
		t.Error("local host should be skipped, h2 still needs detection")
	}
}

func TestValidate_dockerSocketOnDarwin(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts: map[string]HostConfig{
			"mac": {Addr: "user@mac", OS: "darwin", Arch: "arm64", DockerSocket: "/Users/me/.colima/default/docker.sock"},
		},
		Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "mac", Mode: "docker"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("docker_socket on darwin should be valid, got: %v", err)
	}
}

func TestValidate_dockerSocketOnWindows_rejected(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts: map[string]HostConfig{
			"win": {Addr: "user@win", OS: "windows", Arch: "amd64", DockerSocket: "/var/run/docker.sock"},
		},
		Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "win", Mode: "docker"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("docker_socket on windows should be rejected")
	}
	if !strings.Contains(err.Error(), "docker_socket is only supported") {
		t.Errorf("unexpected error: %v", err)
	}
}


func TestValidate_dockerOnWindowsHost(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts:  map[string]HostConfig{"w": {Addr: "a@b", OS: "windows", Arch: "amd64"}},
		Runners: []RunnerConfig{
			{Name: "linux-on-win", Repo: "o/r", Host: "w", Mode: "docker"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mode: docker on windows host should be valid, got: %v", err)
	}
}

func TestValidate_dockerNetworkModeHostOnWindows(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts:  map[string]HostConfig{"w": {Addr: "a@b", OS: "windows", Arch: "amd64"}},
		Runners: []RunnerConfig{{
			Name: "r", Repo: "o/r", Host: "w", Mode: "docker", DockerNetworkMode: "host",
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("docker_network_mode: host on Windows should be valid, got: %v", err)
	}
}

func TestValidate_dockerNetworkModeHostOnDarwin(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts:  map[string]HostConfig{"m": {Addr: "a@b", OS: "darwin", Arch: "arm64"}},
		Runners: []RunnerConfig{{
			Name: "r", Repo: "o/r", Host: "m", Mode: "docker", DockerNetworkMode: "host",
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("docker_network_mode: host on macOS should be valid, got: %v", err)
	}
}

func TestValidate_docker_cap_add_ok(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts:  map[string]HostConfig{"w": {Addr: "a@b", OS: "windows", Arch: "amd64"}},
		Runners: []RunnerConfig{{
			Name: "r", Repo: "o/r", Host: "w", Mode: "docker",
			DockerNetworkMode: "host", DockerCapAdd: []string{"NET_ADMIN"},
		}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("docker_cap_add with docker mode should be valid, got: %v", err)
	}
}

func TestValidate_windows_ps_pwsh(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts: map[string]HostConfig{
			"w": {Addr: "a@b", OS: "windows", Arch: "amd64", WindowsPS: "pwsh"},
		},
		Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "w"}},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("windows_ps: pwsh should be valid, got: %v", err)
	}
}

func TestIsLocalAddr(t *testing.T) {
	t.Parallel()
	if !IsLocalAddr("local") {
		t.Error("\"local\" should be local")
	}
	if !IsLocalAddr("Local") {
		t.Error("\"Local\" should be local (case-insensitive)")
	}
	if IsLocalAddr("user@host") {
		t.Error("\"user@host\" should not be local")
	}
	if IsLocalAddr("") {
		t.Error("empty string should not be local")
	}
}

func TestValidate_localHost(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts:  map[string]HostConfig{"laptop": {Addr: "local", OS: "linux", Arch: "amd64"}},
		Runners: []RunnerConfig{
			{Name: "r", Repo: "o/r", Host: "laptop"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local host should be valid: %v", err)
	}
}

func TestApplyDefaults_localHostAutoDetect(t *testing.T) {
	t.Parallel()
	cfg := Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts:  map[string]HostConfig{"laptop": {Addr: "local"}},
		Runners: []RunnerConfig{
			{Name: "r", Repo: "o/r", Host: "laptop"},
		},
	}
	cfg.applyDefaults()

	h := cfg.Hosts["laptop"]
	if h.OS == "" {
		t.Error("OS should be auto-detected for local host")
	}
	if h.Arch == "" {
		t.Error("Arch should be auto-detected for local host")
	}
}

func TestLoad_localHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runners.yml")
	content := `
github:
  pat: tok
hosts:
  laptop:
    addr: local
runners:
  - name: r1
    repo: o/r
    host: laptop
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	h := cfg.Hosts["laptop"]
	if h.OS == "" {
		t.Error("OS should be auto-detected")
	}
	if h.Arch == "" {
		t.Error("Arch should be auto-detected")
	}
}

func TestConfig_queries(t *testing.T) {
	cfg := &Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts: map[string]HostConfig{
			"h1": {Addr: "a@b", OS: "linux", Arch: "amd64"},
			"h2": {Addr: "c@d", OS: "darwin", Arch: "arm64"},
		},
		Runners: []RunnerConfig{
			{Name: "alpha", Repo: "o/a", Host: "h1", Count: 2},
			{Name: "beta", Repo: "o/b", Host: "h2", Count: 1},
			{Name: "gamma", Repo: "o/a", Host: "h1", Count: 1},
		},
	}

	h1 := cfg.RunnersForHost("h1")
	if len(h1) != 2 {
		t.Fatalf("RunnersForHost h1: got %d", len(h1))
	}
	br := cfg.RunnersForRepo("o/a")
	if len(br) != 2 {
		t.Fatalf("RunnersForRepo o/a: got %d", len(br))
	}
	repos := cfg.UniqueRepos()
	if len(repos) != 2 {
		t.Fatalf("UniqueRepos: got %v", repos)
	}

	rc, ok := cfg.FindRunner("alpha")
	if !ok || rc.Name != "alpha" {
		t.Fatalf("FindRunner alpha: ok=%v", ok)
	}
	rc, ok = cfg.FindRunner("alpha-2")
	if !ok || rc.Name != "alpha" {
		t.Fatalf("FindRunner alpha-2: ok=%v name=%v", ok, rc)
	}
	rc, ok = cfg.FindRunner("nope")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestFindRunnerForLogs_and_ResolveRunnerInstance(t *testing.T) {
	cfg := &Config{
		GitHub: GitHubConfig{PAT: "x"},
		Hosts: map[string]HostConfig{
			"h1": {Addr: "a@b", OS: "linux", Arch: "amd64"},
			"h2": {Addr: "c@d", OS: "windows", Arch: "amd64"},
		},
		Runners: []RunnerConfig{
			{Name: "dup", Repo: "o/r", Host: "h1", Count: 1},
			{Name: "dup", Repo: "o/r", Host: "h2", Count: 1},
		},
	}

	_, err := cfg.FindRunnerForLogs("dup-1", "")
	if err == nil {
		t.Fatal("expected ambiguous without --host")
	}

	rc, err := cfg.FindRunnerForLogs("dup-1", "h2")
	if err != nil || rc.Host != "h2" {
		t.Fatalf("FindRunnerForLogs with host: err=%v host=%v", err, rc)
	}

	inst, err := rc.ResolveRunnerInstance("dup-1")
	if err != nil || inst != "dup-1" {
		t.Fatalf("ResolveRunnerInstance dup-1: %v %q", err, inst)
	}
	inst, err = rc.ResolveRunnerInstance("dup")
	if err != nil || inst != "dup-1" {
		t.Fatalf("ResolveRunnerInstance dup: %v %q", err, inst)
	}

	solo := &Config{
		GitHub:  GitHubConfig{PAT: "x"},
		Hosts:   map[string]HostConfig{"h1": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
		Runners: []RunnerConfig{{Name: "solo", Repo: "o/r", Host: "h1", Count: 2}},
	}
	_, err = solo.FindRunnerForLogs("solo", "")
	if err != nil {
		t.Fatal(err)
	}
	soloRC := &solo.Runners[0]
	_, err = soloRC.ResolveRunnerInstance("solo")
	if err == nil {
		t.Fatal("expected error for multi-instance base name")
	}
}

func TestApplyEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env")
	content := `
# comment
export FOO=bar
EMPTY=
SKIP
BAZ="quoted"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FOO", "")
	t.Setenv("BAZ", "")
	if err := ApplyEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FOO") != "bar" {
		t.Errorf("FOO: got %q", os.Getenv("FOO"))
	}
	if os.Getenv("BAZ") != "quoted" {
		t.Errorf("BAZ: got %q", os.Getenv("BAZ"))
	}
}

func TestApplyEnvFile_missing(t *testing.T) {
	t.Parallel()
	if err := ApplyEnvFile(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatal(err)
	}
}

func TestResolveConfigPath_ghrConfig(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other.yml")
	if err := os.WriteFile(other, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv(EnvVarConfigPath, other)

	got, err := ResolveConfigPath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != other {
		t.Errorf("GHR_CONFIG: want %q got %q", other, got)
	}
}

func TestResolveConfigPath_ignoresCwdLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := t.TempDir()
	local := filepath.Join(dir, "config", "runners.yml")
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv(EnvVarConfigPath, "")

	got, err := ResolveConfigPath("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".ghr", "runners.yml")
	if got != want {
		t.Errorf("cwd config/runners.yml must not be auto-used: want %q got %q", want, got)
	}
}

func TestResolveConfigPath_explicit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "custom.yml")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv(EnvVarConfigPath, filepath.Join(dir, "ignored.yml"))
	if err := os.WriteFile(filepath.Join(dir, "ignored.yml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveConfigPath("custom.yml")
	if err != nil {
		t.Fatal(err)
	}
	if got != p {
		t.Errorf("explicit flag wins: got %q want %q", got, p)
	}
}

func TestFilterRunners(t *testing.T) {
	cfg := &Config{
		Runners: []RunnerConfig{
			{Name: "a", Repo: "o/1", Host: "h1", Count: 2},
			{Name: "b", Repo: "o/2", Host: "h2", Count: 1},
			{Name: "c", Repo: "o/1", Host: "h1", Count: 1},
		},
	}

	all := FilterRunners(cfg, "", "", nil)
	if len(all) != 3 {
		t.Fatalf("no filter: got %d", len(all))
	}

	byHost := FilterRunners(cfg, "h1", "", nil)
	if len(byHost) != 2 {
		t.Fatalf("host h1: got %d", len(byHost))
	}

	byRepo := FilterRunners(cfg, "", "o/1", nil)
	if len(byRepo) != 2 {
		t.Fatalf("repo o/1: got %d", len(byRepo))
	}

	combo := FilterRunners(cfg, "h1", "o/1", nil)
	if len(combo) != 2 {
		t.Fatalf("host+repo: got %d", len(combo))
	}

	byName := FilterRunners(cfg, "", "", []string{"b"})
	if len(byName) != 1 || byName[0].Name != "b" {
		t.Fatalf("name b: got %v", byName)
	}

	byInst := FilterRunners(cfg, "", "", []string{"a-2"})
	if len(byInst) != 1 || byInst[0].Name != "a" {
		t.Fatalf("instance a-2: got %v", byInst)
	}

	stack := FilterRunners(cfg, "h1", "o/1", []string{"c"})
	if len(stack) != 1 || stack[0].Name != "c" {
		t.Fatalf("stacked filters: got %v", stack)
	}
}
