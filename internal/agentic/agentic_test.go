package agentic

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

type prereqTestExecutor struct {
	mu       sync.Mutex
	seen     []string
	response map[string]string
}

func (e *prereqTestExecutor) Run(cmd string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seen = append(e.seen, cmd)

	if out, ok := e.response[cmd]; ok {
		return out, nil
	}
	return "", fmt.Errorf("unexpected command: %s", cmd)
}

func (e *prereqTestExecutor) Upload(string, string) error { return nil }

func (e *prereqTestExecutor) Close() error { return nil }

func (e *prereqTestExecutor) saw(cmd string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, seen := range e.seen {
		if seen == cmd {
			return true
		}
	}
	return false
}

// TestDockerChainCheckCommandEchoOutsideRedirect guards against placing the
// status-tag echo inside the `{ ... } >/dev/null 2>&1` block — that would
// discard stdout and parseDockerChainOutput would always see empty output.
func TestDockerChainCheckCommandEchoOutsideRedirect(t *testing.T) {
	t.Parallel()
	for _, variant := range []string{"privileged"} {
		variant := variant
		t.Run(variant, func(t *testing.T) {
			t.Parallel()
			cmd := dockerChainCheckCommand(variant)
			for _, line := range strings.Split(cmd, "\n") {
				line = strings.TrimSpace(line)
				if !strings.Contains(line, `echo "#docker-`) {
					continue
				}
				// Status tags must follow the block-level redirect, not sit
				// inside `{ ... } >/dev/null 2>&1` where stdout is discarded.
				if !strings.Contains(line, `} >/dev/null 2>&1; echo "#docker-`) {
					t.Errorf("probe tag echo must follow block redirect, got: %q", line)
				}
			}
		})
	}
}

// TestValidateContainerPrereqsDockerChainConsolidation pins the SSH
// round-trip count for the docker CLI → daemon → privileged chain to
// exactly 1 — the consolidated dockerChainCheckCommand replaces the three
// sequential h.Run calls that existed before this optimization. This is
// the hot-path equivalent for the container-mode runner prereq check
// (called once per container-mode runner from `gh sr doctor`).
func TestValidateContainerPrereqsDockerChainConsolidation(t *testing.T) {
	t.Parallel()

	t.Run("uses exactly one h.Run call on happy path", func(t *testing.T) {
		t.Parallel()
		chainCmd := dockerChainCheckCommand("privileged")
		exec := &prereqTestExecutor{
			response: map[string]string{
				chainCmd: "#docker-cli:0\n#docker-daemon:0\n#docker-privileged:0",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)

		failures := ValidateContainerPrereqs(h)

		if len(exec.seen) != 1 {
			t.Errorf("expected exactly 1 h.Run call on happy path, saw %d (%v)", len(exec.seen), exec.seen)
		}
		if exec.seen[0] != chainCmd {
			t.Errorf("expected single call to be dockerChainCheckCommand(\"privileged\"), got %q", exec.seen[0])
		}
		if len(failures) != 0 {
			t.Errorf("happy path should produce no failures, got %#v", failures)
		}
	})

	t.Run("non-linux still short-circuits to one linux-required failure", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{}
		h := host.NewHost("h", config.HostConfig{OS: "darwin"})
		h.SetConn(exec)
		failures := ValidateContainerPrereqs(h)
		if len(failures) != 1 || failures[0].Name != "linux-required" {
			t.Errorf("non-linux must short-circuit to linux-required, got %#v", failures)
		}
		if len(exec.seen) != 0 {
			t.Errorf("non-linux must make zero h.Run calls, saw %d (%v)", len(exec.seen), exec.seen)
		}
	})
}
