package ops

import (
	"net"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/testutil"
)

// ConnectHost is the default host-connection factory behind connectHostFn,
// which the orchestrator tests mock. These tests exercise the real factory
// end-to-end over an in-process SSH server, so the seam every orchestrator
// depends on (dial, handshake, auth, exec) is pinned once, unmocked.
func TestConnectHostLive(t *testing.T) {
	t.Run("returns a connected host that executes commands over SSH", func(t *testing.T) {
		testutil.InstallClientKey(t)
		srv := testutil.StartSSHServer(t, func(cmd string, ch ssh.Channel) testutil.ExecResult {
			return testutil.ExecResult{Stdout: "containerd running\n", Status: 0}
		})
		h, err := ConnectHost("srv1", config.HostConfig{Addr: "root@" + srv.Addr()})
		if err != nil {
			t.Fatalf("ConnectHost(%s): %v", srv.Addr(), err)
		}
		defer h.Close()

		out, err := h.Run("systemctl is-active containerd")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if out != "containerd running" {
			t.Fatalf("Run = %q, want %q", out, "containerd running")
		}
		served := srv.ServedCommands()
		if len(served) != 1 || served[0] != "systemctl is-active containerd" {
			t.Fatalf("server received commands %v, want [systemctl is-active containerd]", served)
		}
	})

	t.Run("dial failure wraps the host name and addr", func(t *testing.T) {
		testutil.InstallClientKey(t)
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		ln.Close()

		h, err := ConnectHost("deadhost", config.HostConfig{Addr: "root@" + addr})
		if err == nil {
			h.Close()
			t.Fatal("ConnectHost on unreachable host = nil error, want dial failure")
		}
		if h != nil {
			t.Fatalf("ConnectHost error = %v but host = %+v, want nil host", err, h)
		}
		msg := err.Error()
		if !strings.Contains(msg, "connecting to deadhost") {
			t.Fatalf("error %q missing host name", msg)
		}
		if !strings.Contains(msg, addr) {
			t.Fatalf("error %q missing addr %s", msg, addr)
		}
	})
}
