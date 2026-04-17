package agentic

import (
	"strings"
	"testing"
)

func TestHasBlockingFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		failures []PrereqFailure
		want     bool
	}{
		{"empty", nil, false},
		{"warnings only", []PrereqFailure{
			{Name: "a", Severity: SeverityWarning},
			{Name: "b", Severity: SeverityWarning},
		}, false},
		{"one error", []PrereqFailure{
			{Name: "a", Severity: SeverityError},
		}, true},
		{"mixed warning and error", []PrereqFailure{
			{Name: "a", Severity: SeverityWarning},
			{Name: "b", Severity: SeverityError},
		}, true},
		{"all errors", []PrereqFailure{
			{Name: "a", Severity: SeverityError},
			{Name: "b", Severity: SeverityError},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HasBlockingFailures(tc.failures)
			if got != tc.want {
				t.Errorf("HasBlockingFailures(%v) = %v, want %v", tc.failures, got, tc.want)
			}
		})
	}
}

func TestFormatRemediation(t *testing.T) {
	t.Parallel()

	t.Run("with DocRef", func(t *testing.T) {
		t.Parallel()
		f := PrereqFailure{
			Name:        "docker-cli",
			Severity:    SeverityError,
			Message:     "docker CLI not found",
			Remediation: "sudo apt-get install -y docker.io",
			DocRef:      "agentic-workflows.md §3g",
		}
		got := FormatRemediation(f)
		if !strings.Contains(got, "[agentic-workflows.md §3g]") {
			t.Errorf("expected DocRef in output, got:\n%s", got)
		}
		if !strings.Contains(got, "docker CLI not found") {
			t.Errorf("expected message in output, got:\n%s", got)
		}
		if !strings.Contains(got, "To fix:") {
			t.Errorf("expected 'To fix:' in output, got:\n%s", got)
		}
		if !strings.Contains(got, "sudo apt-get install -y docker.io") {
			t.Errorf("expected remediation command in output, got:\n%s", got)
		}
	})

	t.Run("without DocRef", func(t *testing.T) {
		t.Parallel()
		f := PrereqFailure{
			Name:        "some-check",
			Severity:    SeverityWarning,
			Message:     "something missing",
			Remediation: "run fix-it",
		}
		got := FormatRemediation(f)
		if strings.Contains(got, "[") {
			t.Errorf("expected no DocRef bracket in output, got:\n%s", got)
		}
		if !strings.Contains(got, "something missing") {
			t.Errorf("expected message in output, got:\n%s", got)
		}
		if !strings.Contains(got, "run fix-it") {
			t.Errorf("expected remediation in output, got:\n%s", got)
		}
	})

	t.Run("multiline remediation indented", func(t *testing.T) {
		t.Parallel()
		f := PrereqFailure{
			Message:     "need stuff",
			Remediation: "line1\nline2",
		}
		got := FormatRemediation(f)
		lines := strings.Split(got, "\n")
		for _, line := range lines {
			if strings.Contains(line, "line1") || strings.Contains(line, "line2") {
				if !strings.HasPrefix(line, "    ") {
					t.Errorf("remediation line not indented with 4 spaces: %q", line)
				}
			}
		}
	})
}

func TestFormatAllRemediations(t *testing.T) {
	t.Parallel()

	t.Run("empty returns empty string", func(t *testing.T) {
		t.Parallel()
		got := FormatAllRemediations(nil)
		if got != "" {
			t.Errorf("expected empty string for no failures, got %q", got)
		}
	})

	t.Run("error uses FAIL label", func(t *testing.T) {
		t.Parallel()
		failures := []PrereqFailure{
			{Name: "docker-cli", Severity: SeverityError, Message: "docker missing", Remediation: "install docker"},
		}
		got := FormatAllRemediations(failures)
		if !strings.Contains(got, "FAIL") {
			t.Errorf("expected FAIL label for error severity, got:\n%s", got)
		}
		if !strings.Contains(got, "docker-cli") {
			t.Errorf("expected failure name in output, got:\n%s", got)
		}
		if !strings.Contains(got, "1 failure") {
			t.Errorf("expected failure count in banner, got:\n%s", got)
		}
	})

	t.Run("warning uses WARN label", func(t *testing.T) {
		t.Parallel()
		failures := []PrereqFailure{
			{Name: "sudo-iptables", Severity: SeverityWarning, Message: "no passwordless sudo", Remediation: "add sudoers rule"},
		}
		got := FormatAllRemediations(failures)
		if !strings.Contains(got, "WARN") {
			t.Errorf("expected WARN label for warning severity, got:\n%s", got)
		}
	})

	t.Run("multiple failures numbered and all included", func(t *testing.T) {
		t.Parallel()
		failures := []PrereqFailure{
			{Name: "a", Severity: SeverityError, Message: "err-a", Remediation: "fix-a"},
			{Name: "b", Severity: SeverityWarning, Message: "warn-b", Remediation: "fix-b"},
		}
		got := FormatAllRemediations(failures)
		if !strings.Contains(got, "[1]") {
			t.Errorf("expected [1] in output, got:\n%s", got)
		}
		if !strings.Contains(got, "[2]") {
			t.Errorf("expected [2] in output, got:\n%s", got)
		}
		if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
			t.Errorf("expected both failure names in output, got:\n%s", got)
		}
		if !strings.Contains(got, "2 failure") {
			t.Errorf("expected '2 failure' in banner, got:\n%s", got)
		}
	})
}
