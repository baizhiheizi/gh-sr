package hostsvc

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func newHost(t *testing.T, mock *testutil.MockExecutor) *host.Host {
	t.Helper()
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	return h
}

func TestStoragePathOrDefault(t *testing.T) {
	t.Parallel()

	if got := StoragePathOrDefault("", "/def"); got != "/def" {
		t.Errorf("empty: got %q, want /def", got)
	}
	if got := StoragePathOrDefault("/cfg", "/def"); got != "/cfg" {
		t.Errorf("configured: got %q, want /cfg", got)
	}
}

func TestRemoteHome(t *testing.T) {
	t.Parallel()

	home, err := RemoteHome(newHost(t, &testutil.MockExecutor{Output: "/root\n"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if home != "/root" {
		t.Errorf("got %q, want /root", home)
	}

	if _, err := RemoteHome(newHost(t, &testutil.MockExecutor{Output: "\n"})); err == nil {
		t.Error("empty $HOME should error")
	}
	if _, err := RemoteHome(newHost(t, &testutil.MockExecutor{RunErr: io.EOF})); err == nil {
		t.Error("failed probe should error")
	}
}

func TestExpandHomePath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		home       string
		configured string
		defaultP   string
		want       string
		wantErr    bool
	}{
		{name: "default", home: "/root", defaultP: "$HOME/.gh-sr/cache", want: "/root/.gh-sr/cache"},
		{name: "absolute", home: "/root", configured: "/srv/cache", defaultP: "$HOME/.gh-sr/cache", want: "/srv/cache"},
		{name: "$HOME prefix", home: "/root", configured: "$HOME/cache", defaultP: "/def", want: "/root/cache"},
		{name: "$HOME only", home: "/root", configured: "$HOME", defaultP: "/def", want: "/root"},
		{name: "empty home errors", home: "", configured: "$HOME/cache", defaultP: "/def", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExpandHomePath(tc.home, tc.configured, tc.defaultP)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveStoragePath(t *testing.T) {
	t.Parallel()

	h := newHost(t, &testutil.MockExecutor{Output: "/root\n"})

	got, err := ResolveStoragePath(h, "", "$HOME/.gh-sr/cache")
	if err != nil || got != "/root/.gh-sr/cache" {
		t.Errorf("default: got %q err %v", got, err)
	}

	got, err = ResolveStoragePath(h, "/srv/cache", "$HOME/.gh-sr/cache")
	if err != nil || got != "/srv/cache" {
		t.Errorf("absolute: got %q err %v", got, err)
	}

	got, err = ResolveStoragePath(h, "$HOME/cache", "/def")
	if err != nil || got != "/root/cache" {
		t.Errorf("$HOME prefix: got %q err %v", got, err)
	}
}

func TestStoragePathShell(t *testing.T) {
	t.Parallel()

	if got := StoragePathShell("", "$HOME/.gh-sr/cache"); got != "storage=$HOME'/.gh-sr/cache'" {
		t.Errorf("default: got %q", got)
	}
	if got := StoragePathShell("$HOME/cache", "/def"); got != "storage=$HOME'/cache'" {
		t.Errorf("$HOME prefix: got %q", got)
	}
	if got := StoragePathShell("/srv/cache", "/def"); got != "storage='/srv/cache'" {
		t.Errorf("absolute: got %q", got)
	}
}

// The shell snippet and the Go-side parser must agree on the convention: the
// script splices $HOME, the parser splices the home the probe reported.
func TestStoragePathShell_AgreesWithExpandHomePath(t *testing.T) {
	t.Parallel()

	shell := StoragePathShell("$HOME/cache", "/def")
	if !strings.HasPrefix(shell, "storage=$HOME") {
		t.Fatalf("shell snippet does not splice $HOME: %q", shell)
	}
	// What the remote shell computes: $HOME expands and is concatenated with
	// the single-quoted literal remainder.
	remainder := strings.Trim(strings.TrimPrefix(shell, "storage=$HOME"), "'")
	expanded := "/root" + remainder
	if expanded != "/root/cache" {
		t.Fatalf("shell expands to %q, want /root/cache", expanded)
	}

	got, err := ExpandHomePath("/root", "$HOME/cache", "/def")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expanded {
		t.Errorf("Go side = %q, shell side = %q", got, expanded)
	}
}

func TestRunnerURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		override  string
		bindAddr  string
		gatewayIP string
		port      int
		want      string
	}{
		{name: "override wins", override: "http://override:9999", bindAddr: "10.0.0.1", gatewayIP: "172.17.0.1", port: 3000, want: "http://override:9999"},
		{name: "explicit bind wins", bindAddr: "10.0.0.1", gatewayIP: "172.17.0.1", port: 3000, want: "http://10.0.0.1:3000"},
		{name: "0.0.0.0 falls through to the gateway", bindAddr: "0.0.0.0", gatewayIP: "172.17.0.1", port: 3000, want: "http://172.17.0.1:3000"},
		{name: "auto bind uses the gateway", gatewayIP: "172.17.0.1", port: 3000, want: "http://172.17.0.1:3000"},
		{name: "no gateway yields no URL", port: 3000, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RunnerURL(tc.override, tc.bindAddr, tc.gatewayIP, tc.port)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLocalURL(t *testing.T) {
	t.Parallel()

	if got := LocalURL("10.0.0.1", "172.17.0.1", 3000); got != "http://10.0.0.1:3000" {
		t.Errorf("explicit bind: got %q", got)
	}
	if got := LocalURL("0.0.0.0", "172.17.0.1", 3000); got != "http://172.17.0.1:3000" {
		t.Errorf("wildcard bind: got %q", got)
	}
	if got := LocalURL("", "172.17.0.1", 3000); got != "http://172.17.0.1:3000" {
		t.Errorf("auto bind: got %q", got)
	}
	if got := LocalURL("", "", 3000); got != "http://127.0.0.1:3000" {
		t.Errorf("loopback fallback: got %q", got)
	}
}

func TestRemoveContainer_RemovesAndPurges(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockExecutor{}
	var buf bytes.Buffer
	err := RemoveContainer(&buf, newHost(t, mock), RemoveOpts{
		Container: "gh-sr-cache",
		Noun:      "cache",
		Label:     "cache",
		PurgeData: true,
		StoragePath: func() (string, error) {
			return "/root/.gh-sr/cache", nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"docker rm -f gh-sr-cache",
		"rm -rf '/root/.gh-sr/cache'",
	}
	if len(mock.Calls) != len(want) {
		t.Fatalf("calls = %v, want %v", mock.Calls, want)
	}
	for i, c := range want {
		if mock.Calls[i] != c {
			t.Errorf("call %d = %q, want %q", i, mock.Calls[i], c)
		}
	}

	out := buf.String()
	for _, s := range []string{"  cache: removed gh-sr-cache\n", "  cache: purged /root/.gh-sr/cache\n"} {
		if !strings.Contains(out, s) {
			t.Errorf("output %q missing %q", out, s)
		}
	}
}

func TestRemoveContainer_WithoutPurgeKeepsData(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockExecutor{}
	var buf bytes.Buffer
	err := RemoveContainer(&buf, newHost(t, mock), RemoveOpts{
		Container:   "gh-sr-cache",
		Noun:        "cache",
		Label:       "cache",
		StoragePath: func() (string, error) { return "/should/not/be/resolved", nil },
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.Calls) != 1 || mock.Calls[0] != "docker rm -f gh-sr-cache" {
		t.Errorf("calls = %v, want only the container removal", mock.Calls)
	}
}

// A storage-path resolution failure must not leave the container running: the
// container is removed first, and only then is the data dir resolved.
func TestRemoveContainer_ResolvesStorageAfterContainerRemoval(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockExecutor{}
	err := RemoveContainer(io.Discard, newHost(t, mock), RemoveOpts{
		Container: "gh-sr-cache",
		Noun:      "cache",
		Label:     "cache",
		PurgeData: true,
		StoragePath: func() (string, error) {
			return "", errors.New("resolving home dir: empty $HOME")
		},
	})
	if err == nil {
		t.Fatal("expected the storage resolution error to propagate")
	}
	if len(mock.Calls) != 1 || mock.Calls[0] != "docker rm -f gh-sr-cache" {
		t.Errorf("calls = %v, want only the container removal", mock.Calls)
	}
}

func TestRemoveContainer_ReportsRemoveFailure(t *testing.T) {
	t.Parallel()

	mock := &testutil.MockExecutor{RunErr: errors.New("no such container")}
	err := RemoveContainer(io.Discard, newHost(t, mock), RemoveOpts{
		Container: "gh-sr-cache",
		Noun:      "cache",
		Label:     "cache",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "removing cache container") {
		t.Errorf("error = %q, want it to mention the cache container", err)
	}
}
