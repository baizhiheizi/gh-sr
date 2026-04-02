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
			name: "empty_pat",
			cfg: Config{
				Hosts: map[string]HostConfig{"h": {Addr: "a@b", OS: "linux", Arch: "amd64"}},
				Runners: []RunnerConfig{
					{Name: "r", Repo: "o/r", Host: "h"},
				},
			},
			frag: "github.pat",
		},
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
			name: "host_os",
			cfg: Config{
				GitHub:  GitHubConfig{PAT: "x"},
				Hosts:   map[string]HostConfig{"h": {Addr: "a@b", OS: "freebsd", Arch: "amd64"}},
				Runners: []RunnerConfig{{Name: "r", Repo: "o/r", Host: "h"}},
			},
			frag: "os must be linux",
		},
		{
			name: "host_arch",
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
