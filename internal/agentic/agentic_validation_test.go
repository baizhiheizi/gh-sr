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
			dockerChainCheckCommand("privileged"): "#docker-cli:0\n#docker-daemon:0\n#docker-privileged:0",
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
	// With dockerChain consolidation, all three probes run unconditionally
	// so a missing CLI surfaces all three dependent failures in one pass
	// (matches the containerAgenticFanout UX — see PR #322).
	exec := &prereqTestExecutor{
		response: map[string]string{
			dockerChainCheckCommand("privileged"): "#docker-cli:127\n#docker-daemon:127\n#docker-privileged:127",
		},
	}
	h := host.NewHost("h", config.HostConfig{OS: "linux"})
	h.SetConn(exec)
	failures := ValidateContainerPrereqs(h)
	if len(failures) != 3 {
		t.Fatalf("expected 3 failures (cli+daemon+privileged all surface when CLI missing), got %d (%#v)", len(failures), failures)
	}
	f := failureByName(t, failures, "docker-cli")
	if f.Name == "" {
		t.Fatalf("expected docker-cli failure, got %#v", failures)
	}
	if f.Severity != SeverityError {
		t.Errorf("Severity = %q, want error", f.Severity)
	}
}

func TestValidateContainerPrereqs_dockerDaemonDown(t *testing.T) {
	t.Parallel()
	exec := &prereqTestExecutor{
		response: map[string]string{
			// docker --version OK, but daemon down → daemon + privileged
			// both surface because the privileged probe depends on the
			// running daemon.
			dockerChainCheckCommand("privileged"): "#docker-cli:0\n#docker-daemon:1\n#docker-privileged:1",
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
	// Privileged must NOT also surface — the docker-privileged tag was :1
	// (failure) in the mocked output, so it WILL surface; this is intended
	// parallel-failures UX. Verify it's there with the right severity.
	fp := failureByName(t, failures, "docker-privileged")
	if fp.Name == "" {
		t.Fatalf("expected docker-privileged failure (daemon down → privileged also fails), got %#v", failures)
	}
}

func TestValidateContainerPrereqs_privilegedBlocked(t *testing.T) {
	t.Parallel()
	exec := &prereqTestExecutor{
		response: map[string]string{
			dockerChainCheckCommand("privileged"): "#docker-cli:0\n#docker-daemon:0\n#docker-privileged:1",
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
