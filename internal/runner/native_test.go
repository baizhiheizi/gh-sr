package runner

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/an-lee/ghr/internal/config"
	"github.com/an-lee/ghr/internal/host"
)

func Test_archForGitHub(t *testing.T) {
	t.Parallel()
	if archForGitHub("amd64") != "x64" {
		t.Errorf("amd64 -> x64")
	}
	if archForGitHub("arm64") != "arm64" {
		t.Errorf("arm64")
	}
	if archForGitHub("riscv") != "riscv" {
		t.Errorf("passthrough: got %q", archForGitHub("riscv"))
	}
}

func Test_runnerTarballURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ver, os, arch, wantSub string
	}{
		{"2.320.0", "linux", "x64", "actions-runner-linux-x64-2.320.0.tar.gz"},
		{"2.320.0", "darwin", "arm64", "actions-runner-osx-arm64-2.320.0.tar.gz"},
		{"2.320.0", "windows", "x64", "actions-runner-win-x64-2.320.0.zip"},
		{"2.320.0", "windows", "arm64", "actions-runner-win-arm64-2.320.0.zip"},
	}
	for _, tc := range cases {
		u := runnerTarballURL(tc.ver, tc.os, tc.arch)
		if !strings.Contains(u, tc.wantSub) {
			t.Errorf("%s/%s/%s: got %q want substring %q", tc.ver, tc.os, tc.arch, u, tc.wantSub)
		}
		if !strings.HasPrefix(u, "https://github.com/actions/runner/releases/download/v") {
			t.Errorf("prefix: %q", u)
		}
	}
	if runnerTarballURL("1", "freebsd", "amd64") != "" {
		t.Errorf("unsupported OS should return empty")
	}
}

func TestNativeRunnerConfigPresent_local(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-style paths and sh -c probe")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	inst := "ghr-native-probe-" + filepath.Base(t.TempDir())

	base := filepath.Join(home, ".ghr", "runners", inst)
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })

	if err := os.WriteFile(filepath.Join(base, ".runner"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := host.NewHost("local-test", config.HostConfig{Addr: config.LocalAddr, OS: "linux", Arch: "amd64"})
	if err := h.Connect(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })

	ok, err := NativeRunnerConfigPresent(h, inst)
	if err != nil || !ok {
		t.Fatalf("expected installed: ok=%v err=%v", ok, err)
	}

	okMissing, errMissing := NativeRunnerConfigPresent(h, inst+"-not-there")
	if errMissing != nil || okMissing {
		t.Fatalf("expected not installed: ok=%v err=%v", okMissing, errMissing)
	}
}
