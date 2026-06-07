package agentic

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// failureByName returns the first failure with the given Name, or nil if not found.
func failureByName(t *testing.T, failures []PrereqFailure, name string) PrereqFailure {
	t.Helper()
	for _, f := range failures {
		if f.Name == name {
			return f
		}
	}
	return PrereqFailure{}
}

func TestValidateContainerInnerNetwork(t *testing.T) {
	t.Parallel()

	t.Run("non-linux short-circuits", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h", config.HostConfig{OS: "darwin"})
		h.SetConn(&prereqTestExecutor{})
		if got := ValidateContainerInnerNetwork(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("non-linux must short-circuit, got %#v", got)
		}
	})

	t.Run("successful check returns nil", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				`docker exec "gh-sr-runner" sh -c 'set -eu
ok=0
for i in 1 2 3 4 5; do
  ip=$(docker run --rm alpine getent hosts host.docker.internal 2>/dev/null | awk "{print \$1; exit}")
  case "$ip" in
    "" | 127.* | ::1) ;;
    *) ok=1; break ;;
  esac
  sleep 1
done
[ "$ok" -eq 1 ]'`: ``,
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		if got := ValidateContainerInnerNetwork(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("successful check must return nil, got %#v", got)
		}
	})

	t.Run("failing check returns host-docker-internal warning", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{} // every command errors
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateContainerInnerNetwork(h, "gh-sr-runner", "agentic-1")
		if len(failures) != 1 {
			t.Fatalf("expected 1 failure, got %d (%#v)", len(failures), failures)
		}
		f := failures[0]
		if f.Name != "container-inner-host-docker-internal" {
			t.Errorf("Name = %q, want %q", f.Name, "container-inner-host-docker-internal")
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		if !strings.Contains(f.Message, "gh-sr-runner") {
			t.Errorf("Message should name the container, got %q", f.Message)
		}
		if !strings.Contains(f.Remediation, "gh sr rebuild agentic-1") {
			t.Errorf("Remediation should reference gh sr rebuild, got %q", f.Remediation)
		}
		if f.DocRef == "" {
			t.Error("DocRef should be populated")
		}
	})
}

func TestValidateContainerInnerResolv(t *testing.T) {
	t.Parallel()

	t.Run("non-linux short-circuits", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h", config.HostConfig{OS: "darwin"})
		h.SetConn(&prereqTestExecutor{})
		if got := ValidateContainerInnerResolv(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("non-linux must short-circuit, got %#v", got)
		}
	})

	t.Run("successful check returns nil", func(t *testing.T) {
		t.Parallel()
		// Mock by always returning nil for the resolv check command.
		exec := &prereqTestExecutor{
			response: map[string]string{
				`docker exec "gh-sr-runner" sh -c 'set -eu
gw=$(ip -4 -o addr show docker0 2>/dev/null | awk "{print \$4}" | cut -d/ -f1 | head -n1)
[ -n "$gw" ] || gw=10.200.0.1
grep -Eq "^nameserver[[:space:]]+$gw([[:space:]]|$)" /etc/resolv.conf'`: ``,
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		if got := ValidateContainerInnerResolv(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("successful check must return nil, got %#v", got)
		}
	})

	t.Run("failing check returns container-inner-resolv warning with both runners", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateContainerInnerResolv(h, "gh-sr-runner", "agentic-1")
		f := failureByName(t, failures, "container-inner-resolv")
		if f.Name == "" {
			t.Fatalf("expected container-inner-resolv failure, got %#v", failures)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		if !strings.Contains(f.Remediation, "gh sr rebuild agentic-1") {
			t.Errorf("Remediation should reference gh sr rebuild, got %q", f.Remediation)
		}
		if !strings.Contains(f.Remediation, "docker exec gh-sr-runner") {
			t.Errorf("Remediation should reference docker exec verification, got %q", f.Remediation)
		}
	})
}

func TestValidateContainerAWFServiceRouting(t *testing.T) {
	t.Parallel()

	t.Run("non-linux short-circuits", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h", config.HostConfig{OS: "darwin"})
		h.SetConn(&prereqTestExecutor{})
		if got := ValidateContainerAWFServiceRouting(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("non-linux must short-circuit, got %#v", got)
		}
	})

	t.Run("successful check returns nil", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				`docker exec "gh-sr-runner" sh -c 'iptables -t nat -S PREROUTING 2>/dev/null | grep -Fq -e "-A PREROUTING -s 172.30.0.0/24 -m addrtype --dst-type LOCAL -j RETURN"'`: ``,
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		if got := ValidateContainerAWFServiceRouting(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("successful check must return nil, got %#v", got)
		}
	})

	t.Run("failing check returns container-awf-service-routing warning", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateContainerAWFServiceRouting(h, "gh-sr-runner", "agentic-1")
		f := failureByName(t, failures, "container-awf-service-routing")
		if f.Name == "" {
			t.Fatalf("expected container-awf-service-routing failure, got %#v", failures)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		if !strings.Contains(f.Message, "Connection refused") {
			t.Errorf("Message should mention 'Connection refused' symptom, got %q", f.Message)
		}
		if !strings.Contains(f.Remediation, "iptables -t nat -I PREROUTING") {
			t.Errorf("Remediation should show the live iptables workaround, got %q", f.Remediation)
		}
	})
}

func TestValidateContainerAWF(t *testing.T) {
	t.Parallel()

	t.Run("non-linux short-circuits", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h", config.HostConfig{OS: "darwin"})
		h.SetConn(&prereqTestExecutor{})
		if got := ValidateContainerAWF(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("non-linux must short-circuit, got %#v", got)
		}
	})

	t.Run("successful check returns nil", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				`docker exec "gh-sr-runner" sh -lc 'set -eu
command -v awf >/dev/null
sudo -n -E awf --version >/dev/null'`: ``,
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		if got := ValidateContainerAWF(h, "gh-sr-runner", "agentic-1"); got != nil {
			t.Errorf("successful check must return nil, got %#v", got)
		}
	})

	t.Run("failing check returns container-awf warning with live unblock hint", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateContainerAWF(h, "gh-sr-runner", "agentic-1")
		f := failureByName(t, failures, "container-awf")
		if f.Name == "" {
			t.Fatalf("expected container-awf failure, got %#v", failures)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		if !strings.Contains(f.Message, "awf is not available") {
			t.Errorf("Message should mention 'awf is not available', got %q", f.Message)
		}
		if !strings.Contains(f.Remediation, "AWF_FORCE_BINARY=1") {
			t.Errorf("Remediation should reference the live unblock env var, got %q", f.Remediation)
		}
	})
}

func TestValidateContainerPrereqs_nonLinux(t *testing.T) {
	t.Parallel()
	for _, os := range []string{"darwin", "windows"} {
		os := os
		t.Run(os, func(t *testing.T) {
			t.Parallel()
			h := host.NewHost("h", config.HostConfig{OS: os})
			h.SetConn(&prereqTestExecutor{})
			failures := ValidateContainerPrereqs(h)
			if len(failures) != 1 {
				t.Fatalf("expected 1 failure on %s, got %d (%#v)", os, len(failures), failures)
			}
			if failures[0].Name != "linux-required" {
				t.Errorf("Name = %q, want linux-required", failures[0].Name)
			}
			if failures[0].Severity != SeverityError {
				t.Errorf("Severity = %q, want error", failures[0].Severity)
			}
		})
	}
}

// TestValidateContainerPrereqs_happyPath checks the linear chain:
// docker --version -> docker info -> docker --privileged smoke test all pass.
func TestValidateContainerPrereqs_happyPath(t *testing.T) {
	t.Parallel()
	exec := &prereqTestExecutor{
		response: map[string]string{
			`docker --version 2>/dev/null`: `Docker version 28.0.0, build abc`,
			`docker info >/dev/null 2>&1`:  ``,
			`docker run --rm --privileged alpine sh -c "echo privileged-ok" 2>/dev/null`: `privileged-ok`,
		},
	}
	h := host.NewHost("h", config.HostConfig{OS: "linux"})
	h.SetConn(exec)
	if got := ValidateContainerPrereqs(h); len(got) != 0 {
		t.Errorf("expected no failures on happy path, got %#v", got)
	}
}

func TestValidateContainerPrereqs_dockerCLIMissing(t *testing.T) {
	t.Parallel()
	exec := &prereqTestExecutor{} // every command errors
	h := host.NewHost("h", config.HostConfig{OS: "linux"})
	h.SetConn(exec)
	failures := ValidateContainerPrereqs(h)
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure (docker-cli short-circuits), got %d (%#v)", len(failures), failures)
	}
	if failures[0].Name != "docker-cli" {
		t.Errorf("Name = %q, want docker-cli", failures[0].Name)
	}
	if failures[0].Severity != SeverityError {
		t.Errorf("Severity = %q, want error", failures[0].Severity)
	}
}

func TestValidateContainerPrereqs_dockerDaemonDown(t *testing.T) {
	t.Parallel()
	exec := &prereqTestExecutor{
		response: map[string]string{
			`docker --version 2>/dev/null`: `Docker version 28.0.0`,
			// `docker info` errors → daemon-down branch
		},
	}
	h := host.NewHost("h", config.HostConfig{OS: "linux"})
	h.SetConn(exec)
	failures := ValidateContainerPrereqs(h)
	f := failureByName(t, failures, "docker-daemon")
	if f.Name == "" {
		t.Fatalf("expected docker-daemon failure, got %#v", failures)
	}
	if f.Severity != SeverityError {
		t.Errorf("Severity = %q, want error", f.Severity)
	}
}

func TestValidateContainerPrereqs_privilegedBlocked(t *testing.T) {
	t.Parallel()
	exec := &prereqTestExecutor{
		response: map[string]string{
			`docker --version 2>/dev/null`: `Docker version 28.0.0`,
			`docker info >/dev/null 2>&1`:  ``,
			`docker run --rm --privileged alpine sh -c "echo privileged-ok" 2>/dev/null`: `permission denied`,
		},
	}
	h := host.NewHost("h", config.HostConfig{OS: "linux"})
	h.SetConn(exec)
	failures := ValidateContainerPrereqs(h)
	f := failureByName(t, failures, "docker-privileged")
	if f.Name == "" {
		t.Fatalf("expected docker-privileged failure, got %#v", failures)
	}
	if f.Severity != SeverityError {
		t.Errorf("Severity = %q, want error", f.Severity)
	}
	if !strings.Contains(f.Remediation, "userns-remap") {
		t.Errorf("Remediation should mention userns-remap as a common cause, got %q", f.Remediation)
	}
}

// TestPrereqTestExecutor_RunErrorsWhenNoResponse ensures the mock surfaces
// run errors as wrapped errors — important so callers can branch on err != nil.
func TestPrereqTestExecutor_RunErrorsWhenNoResponse(t *testing.T) {
	t.Parallel()
	exec := &prereqTestExecutor{} // no responses, every Run errors
	_, err := exec.Run("anything")
	if err == nil {
		t.Fatal("expected error when no response is configured")
	}
	if !strings.Contains(err.Error(), "unexpected command") {
		t.Errorf("error should mention 'unexpected command', got %q", err)
	}
}
