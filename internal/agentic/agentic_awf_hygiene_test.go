package agentic

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// The three docker/iptables probes that ValidateAWFHygiene fans out across
// goroutines. Lifted into named constants so the test assertions can reuse
// the same strings the function emits — guard against silent refactors of
// the probe shapes (the previous test-improver run noted that the exact-string
// match in prereqTestExecutor is a feature, not a bug, for exactly this reason).
//
// Note: the host-level iptables probe uses `sudo -n` (operators don't run
// gh-sr as root on the host), but the inner-Docker probe drops sudo because
// DinD runs its inner daemon as root. The two helpers below keep the two
// shapes in sync — touching one without the other will break
// ValidateAWFHygieneInner's tests.
const (
	awfHygieneAwfCmd      = `docker ps -a --filter "name=awf-" --filter "name=gh-aw" --format '{{.Names}}' 2>/dev/null | head -20`
	awfHygieneIptablesCmd = `sudo -n iptables -L DOCKER-USER --line-numbers -n 2>/dev/null | grep -i "awf\|gh-aw" | head -20`
	awfHygieneMcpgCmd     = `docker ps -a --filter "name=gh-aw-mcpg-" --format '{{.Names}}' 2>/dev/null | head -20`

	// The inner-Docker variant of the iptables probe — no `sudo -n`
	// because the DinD inner daemon runs as root.
	awfHygieneIptablesInnerCmd = `iptables -L DOCKER-USER --line-numbers -n 2>/dev/null | grep -i "awf\|gh-aw" | head -20`
)

// allAwfHygieneClean is the canonical "no leftovers" mock: every probe returns
// an empty string. Build a fresh map per test so concurrent subtests don't
// share mutation state.
func allAwfHygieneClean() *prereqTestExecutor {
	return &prereqTestExecutor{
		response: map[string]string{
			awfHygieneAwfCmd:      "",
			awfHygieneIptablesCmd: "",
			awfHygieneMcpgCmd:     "",
		},
	}
}

func TestValidateAWFHygiene(t *testing.T) {
	t.Parallel()

	t.Run("non-linux short-circuits", func(t *testing.T) {
		t.Parallel()
		for _, os := range []string{"darwin", "windows"} {
			os := os
			t.Run(os, func(t *testing.T) {
				t.Parallel()
				h := host.NewHost("h", config.HostConfig{OS: os})
				h.SetConn(&prereqTestExecutor{}) // unused — short-circuit
				if got := ValidateAWFHygiene(h); got != nil {
					t.Errorf("non-linux must return nil, got %#v", got)
				}
			})
		}
	})

	t.Run("clean host returns nil and all three probes ran", func(t *testing.T) {
		t.Parallel()
		exec := allAwfHygieneClean()
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		if got := ValidateAWFHygiene(h); got != nil {
			t.Errorf("clean host must return nil, got %#v", got)
		}
		// Lock in the fan-out: all three goroutines must have run, even when
		// the result is empty. A regression that short-circuits after the
		// first probe would leave mcpg-orphan-detector blind.
		for _, cmd := range []string{awfHygieneAwfCmd, awfHygieneIptablesCmd, awfHygieneMcpgCmd} {
			if !exec.saw(cmd) {
				t.Errorf("expected probe to run: %q", cmd)
			}
		}
	})

	t.Run("whitespace-only output treated as clean", func(t *testing.T) {
		t.Parallel()
		// TrimSpace is the function's "no artefact" gate; a probe that
		// returns "   \n  " should not produce a failure.
		exec := &prereqTestExecutor{
			response: map[string]string{
				awfHygieneAwfCmd:      "   \n  \t",
				awfHygieneIptablesCmd: "",
				awfHygieneMcpgCmd:     "",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		if got := ValidateAWFHygiene(h); got != nil {
			t.Errorf("whitespace-only output must be treated as clean, got %#v", got)
		}
	})

	t.Run("orphan awf containers returns awf-orphan-containers warning", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				awfHygieneAwfCmd:      "awf-c1\nawf-c2",
				awfHygieneIptablesCmd: "",
				awfHygieneMcpgCmd:     "",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygiene(h)
		f := failureByName(t, failures, "awf-orphan-containers")
		if f.Name == "" {
			t.Fatalf("expected awf-orphan-containers failure, got %#v", failures)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		if !strings.Contains(f.Message, "crashed jobs") {
			t.Errorf("Message should mention crashed jobs, got %q", f.Message)
		}
		if !strings.Contains(f.Remediation, "docker rm -f") {
			t.Errorf("Remediation should show docker rm -f cleanup, got %q", f.Remediation)
		}
		if !strings.Contains(f.Remediation, "xargs") {
			t.Errorf("Remediation should pipeline through xargs, got %q", f.Remediation)
		}
		if f.DocRef == "" {
			t.Error("DocRef should be populated")
		}
		// The other two probes ran with empty output → only one failure total.
		if len(failures) != 1 {
			t.Errorf("expected 1 failure, got %d (%#v)", len(failures), failures)
		}
	})

	t.Run("stale DOCKER-USER rules returns stale-docker-user-rules warning", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				awfHygieneAwfCmd:      "",
				awfHygieneIptablesCmd: "1  DROP  all  --  172.30.0.5  anywhere",
				awfHygieneMcpgCmd:     "",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygiene(h)
		f := failureByName(t, failures, "stale-docker-user-rules")
		if f.Name == "" {
			t.Fatalf("expected stale-docker-user-rules failure, got %#v", failures)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		if !strings.Contains(f.Remediation, "iptables -F DOCKER-USER") {
			t.Errorf("Remediation should show the flush command, got %q", f.Remediation)
		}
		if !strings.Contains(f.Remediation, "no agentic jobs are running") {
			t.Errorf("Remediation should warn about safe-flush precondition, got %q", f.Remediation)
		}
		if len(failures) != 1 {
			t.Errorf("expected 1 failure, got %d (%#v)", len(failures), failures)
		}
	})

	t.Run("orphan MCP gateway containers returns mcpg-orphan-containers warning", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				awfHygieneAwfCmd:      "",
				awfHygieneIptablesCmd: "",
				awfHygieneMcpgCmd:     "gh-aw-mcpg-1\ngh-aw-mcpg-2",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygiene(h)
		f := failureByName(t, failures, "mcpg-orphan-containers")
		if f.Name == "" {
			t.Fatalf("expected mcpg-orphan-containers failure, got %#v", failures)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		if !strings.Contains(f.Remediation, "MCP gateway") {
			t.Errorf("Remediation should mention MCP gateway, got %q", f.Remediation)
		}
		if !strings.Contains(f.Remediation, "gh-aw-mcpg-") {
			t.Errorf("Remediation should reference the filter pattern, got %q", f.Remediation)
		}
		if len(failures) != 1 {
			t.Errorf("expected 1 failure, got %d (%#v)", len(failures), failures)
		}
	})

	t.Run("all three probes fail returns three warnings", func(t *testing.T) {
		t.Parallel()
		// No responses → every Run errors → out is "" → no failure.
		// So we need to wire up all three commands with non-empty output.
		exec := &prereqTestExecutor{
			response: map[string]string{
				awfHygieneAwfCmd:      "awf-c1",
				awfHygieneIptablesCmd: "1  DROP",
				awfHygieneMcpgCmd:     "gh-aw-mcpg-1",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygiene(h)
		if len(failures) != 3 {
			t.Fatalf("expected 3 failures when all probes hit, got %d (%#v)", len(failures), failures)
		}
		// Lookup by name because the goroutine fan-out makes the slice order
		// non-deterministic — a regression that serialised the probes via
		// an ordered channel would not be caught by this test, but a
		// regression that dropped one of the three would be.
		for _, want := range []string{"awf-orphan-containers", "stale-docker-user-rules", "mcpg-orphan-containers"} {
			f := failureByName(t, failures, want)
			if f.Name == "" {
				t.Errorf("missing %q in failures %#v", want, failures)
				continue
			}
			if f.Severity != SeverityWarning {
				t.Errorf("%s: Severity = %q, want warning", want, f.Severity)
			}
			if f.DocRef == "" {
				t.Errorf("%s: DocRef should be populated", want)
			}
		}
	})
}

func TestValidateAWFHygieneInner(t *testing.T) {
	t.Parallel()

	const outerContainer = "gh-sr-myinstance"

	// The three inner probes are the outer ones wrapped in `docker exec "X" `.
	// strcov.Quote is used by the function so the prefix is literally
	// `docker exec "gh-sr-myinstance" ` (double-quoted, space-prefixed).
	// The iptables probe drops the `sudo -n` prefix because the DinD inner
	// daemon runs as root.
	innerAwfCmd := `docker exec "gh-sr-myinstance" ` + awfHygieneAwfCmd
	innerIptablesCmd := `docker exec "gh-sr-myinstance" ` + awfHygieneIptablesInnerCmd
	innerMcpgCmd := `docker exec "gh-sr-myinstance" ` + awfHygieneMcpgCmd

	t.Run("non-linux short-circuits", func(t *testing.T) {
		t.Parallel()
		for _, os := range []string{"darwin", "windows"} {
			os := os
			t.Run(os, func(t *testing.T) {
				t.Parallel()
				h := host.NewHost("h", config.HostConfig{OS: os})
				h.SetConn(&prereqTestExecutor{})
				if got := ValidateAWFHygieneInner(h, outerContainer); got != nil {
					t.Errorf("non-linux must return nil, got %#v", got)
				}
			})
		}
	})

	t.Run("clean inner Docker returns nil", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				innerAwfCmd:      "",
				innerIptablesCmd: "",
				innerMcpgCmd:     "",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		if got := ValidateAWFHygieneInner(h, outerContainer); got != nil {
			t.Errorf("clean inner Docker must return nil, got %#v", got)
		}
		for _, cmd := range []string{innerAwfCmd, innerIptablesCmd, innerMcpgCmd} {
			if !exec.saw(cmd) {
				t.Errorf("expected inner probe to run: %q", cmd)
			}
		}
	})

	t.Run("orphan awf inside container returns inner warning naming the container", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				innerAwfCmd:      "awf-c1",
				innerIptablesCmd: "",
				innerMcpgCmd:     "",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygieneInner(h, outerContainer)
		f := failureByName(t, failures, "awf-orphan-containers-inner")
		if f.Name == "" {
			t.Fatalf("expected awf-orphan-containers-inner failure, got %#v", failures)
		}
		if f.Severity != SeverityWarning {
			t.Errorf("Severity = %q, want warning", f.Severity)
		}
		// The Message and Remediation must name the container so an operator
		// running `gh sr doctor` knows which runner to ssh into.
		if !strings.Contains(f.Message, outerContainer) {
			t.Errorf("Message should name the runner container %q, got %q", outerContainer, f.Message)
		}
		if !strings.Contains(f.Message, "inner Docker") {
			t.Errorf("Message should distinguish inner Docker from host, got %q", f.Message)
		}
		if !strings.Contains(f.Remediation, outerContainer) {
			t.Errorf("Remediation should name the runner container, got %q", f.Remediation)
		}
		if !strings.Contains(f.Remediation, "docker exec") {
			t.Errorf("Remediation should use docker exec to enter the container, got %q", f.Remediation)
		}
		if !strings.Contains(f.Remediation, "docker rm -f") {
			t.Errorf("Remediation should pipeline cleanup, got %q", f.Remediation)
		}
		if len(failures) != 1 {
			t.Errorf("expected 1 failure, got %d (%#v)", len(failures), failures)
		}
	})

	t.Run("stale DOCKER-USER rules in inner netns returns inner warning", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				innerAwfCmd:      "",
				innerIptablesCmd: "1  DROP  all  --  172.30.0.5  anywhere",
				innerMcpgCmd:     "",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygieneInner(h, outerContainer)
		f := failureByName(t, failures, "stale-docker-user-rules-inner")
		if f.Name == "" {
			t.Fatalf("expected stale-docker-user-rules-inner failure, got %#v", failures)
		}
		if !strings.Contains(f.Message, "inner netns") {
			t.Errorf("Message should mention inner netns, got %q", f.Message)
		}
		if !strings.Contains(f.Remediation, "docker exec "+outerContainer+" iptables -F DOCKER-USER") {
			t.Errorf("Remediation should flush via docker exec %s, got %q", outerContainer, f.Remediation)
		}
		if len(failures) != 1 {
			t.Errorf("expected 1 failure, got %d (%#v)", len(failures), failures)
		}
	})

	t.Run("orphan MCP gateway inside container returns inner warning", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				innerAwfCmd:      "",
				innerIptablesCmd: "",
				innerMcpgCmd:     "gh-aw-mcpg-1",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygieneInner(h, outerContainer)
		f := failureByName(t, failures, "mcpg-orphan-containers-inner")
		if f.Name == "" {
			t.Fatalf("expected mcpg-orphan-containers-inner failure, got %#v", failures)
		}
		if !strings.Contains(f.Message, outerContainer) {
			t.Errorf("Message should name the runner container, got %q", f.Message)
		}
		if !strings.Contains(f.Remediation, "docker exec -it "+outerContainer) {
			t.Errorf("Remediation should show docker exec -it entrypoint, got %q", f.Remediation)
		}
		if len(failures) != 1 {
			t.Errorf("expected 1 failure, got %d (%#v)", len(failures), failures)
		}
	})

	t.Run("all three inner probes fail returns three inner warnings", func(t *testing.T) {
		t.Parallel()
		exec := &prereqTestExecutor{
			response: map[string]string{
				innerAwfCmd:      "awf-c1",
				innerIptablesCmd: "1  DROP",
				innerMcpgCmd:     "gh-aw-mcpg-1",
			},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		failures := ValidateAWFHygieneInner(h, outerContainer)
		if len(failures) != 3 {
			t.Fatalf("expected 3 inner failures, got %d (%#v)", len(failures), failures)
		}
		for _, want := range []string{
			"awf-orphan-containers-inner",
			"stale-docker-user-rules-inner",
			"mcpg-orphan-containers-inner",
		} {
			f := failureByName(t, failures, want)
			if f.Name == "" {
				t.Errorf("missing inner failure %q in %#v", want, failures)
				continue
			}
			if !strings.Contains(f.Message, outerContainer) {
				t.Errorf("%s: Message should name %s, got %q", want, outerContainer, f.Message)
			}
		}
	})

	t.Run("container name with special chars is shell-quoted in commands", func(t *testing.T) {
		t.Parallel()
		// strconv.Quote should escape a name that contains a double-quote
		// (or other shell-special characters). We don't assert the exact
		// output, just that the function does not panic and that the mock
		// recorded a command beginning with the quoted form.
		const weirdName = `name"with$special`
		exec := &prereqTestExecutor{
			response: map[string]string{},
		}
		h := host.NewHost("h", config.HostConfig{OS: "linux"})
		h.SetConn(exec)
		_ = ValidateAWFHygieneInner(h, weirdName)
		// Verify all three commands were prefixed with `docker exec "..." `.
		// strconv.Quote("name\"with$special") = `"name\"with$special"`.
		const expectedPrefix = `docker exec "name\"with$special" `
		count := 0
		for _, seen := range exec.seen {
			if strings.HasPrefix(seen, expectedPrefix) {
				count++
			}
		}
		if count != 3 {
			t.Errorf("expected 3 commands prefixed with %q, got %d (seen=%v)", expectedPrefix, count, exec.seen)
		}
	})
}
