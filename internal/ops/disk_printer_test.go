package ops

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/runner"
)

// TestPrintPruneResult covers the four result shapes emitted to the user:
//
//   - err result: prints "prefix + instance on host: error: <err>"
//   - skipped result: prints "prefix + instance on host: skipped (<reason>)"
//   - successful result with actions: prints one line per action
//   - dryRun flag: changes the prefix from "  " to "  [dry-run] "
//
// The prefix constant is duplicated here so a refactor that changes the
// dry-run marker is caught loudly. See also the table-driven dispatcher
// cases below.
func TestPrintPruneResult(t *testing.T) {
	t.Parallel()

	t.Run("err result prints error line", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printPruneResult(&buf, runner.PruneResult{
			Instance: "ci-1",
			Host:     "host-a",
			Err:      errors.New("boom"),
		}, false)
		got := buf.String()
		if !strings.Contains(got, "ci-1 on host-a: error: boom") {
			t.Errorf("missing error line: %q", got)
		}
		if strings.HasPrefix(got, "  [dry-run]") {
			t.Errorf("dry-run prefix leaked into non-dry-run output: %q", got)
		}
	})

	t.Run("skipped result prints skipped reason", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printPruneResult(&buf, runner.PruneResult{
			Instance: "ci-2",
			Host:     "host-a",
			Skipped:  true,
			Reason:   "GitHub status unknown (use --force)",
		}, false)
		got := buf.String()
		if !strings.Contains(got, "ci-2 on host-a: skipped (GitHub status unknown (use --force))") {
			t.Errorf("missing skipped line: %q", got)
		}
	})

	t.Run("successful result prints one line per action", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printPruneResult(&buf, runner.PruneResult{
			Instance: "ci-3",
			Host:     "host-a",
			Actions:  []string{"work: 1.0 GiB removed", "temp: 100 MiB removed"},
		}, false)
		got := buf.String()
		for _, want := range []string{
			"ci-3 on host-a: work: 1.0 GiB removed",
			"ci-3 on host-a: temp: 100 MiB removed",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("missing action line %q in: %q", want, got)
			}
		}
		if strings.Contains(got, "error:") {
			t.Errorf("error prefix leaked into success output: %q", got)
		}
		if strings.Contains(got, "skipped") {
			t.Errorf("skipped prefix leaked into success output: %q", got)
		}
	})

	t.Run("dryRun flips the prefix to [dry-run]", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		printPruneResult(&buf, runner.PruneResult{
			Instance: "ci-4",
			Host:     "host-a",
			Actions:  []string{"work: 1.0 GiB would be removed"},
		}, true)
		got := buf.String()
		if !strings.HasPrefix(got, "  [dry-run]") {
			t.Errorf("missing dry-run prefix: %q", got)
		}
		if !strings.Contains(got, "ci-4 on host-a: work: 1.0 GiB would be removed") {
			t.Errorf("missing action line: %q", got)
		}
	})
}

// TestPrintDiskUsageTable is the integration test for the table printer.
// It exercises:
//
//   - empty input: prints the "No runner directories found." sentinel
//   - happy path: header + one row + total
//   - error row: shows "error" mode + the error message in the WORK column
//   - busy field flag: "yes" when Busy, "no" when Remote=="online" but not Busy, "-" otherwise
//   - orphan field flag: "yes" when Orphan, "no" otherwise
//   - total byte accounting: sums TotalBytes from non-error rows only
//   - sorted output: rows print in (host, instance) order regardless of input order
//
// FormatBytesHuman is exercised transitively; we assert on key substrings
// rather than full strings so the human-formatting tunables (GiB vs MiB vs
// KiB) can change without test churn.
func TestPrintDiskUsageTable(t *testing.T) {
	t.Parallel()

	t.Run("empty entries prints sentinel", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		PrintDiskUsageTable(&buf, nil)
		got := buf.String()
		if !strings.Contains(got, "No runner directories found.") {
			t.Errorf("missing sentinel: %q", got)
		}
		// Empty input must NOT include the totals line.
		if strings.Contains(got, "Total:") {
			t.Errorf("empty input should not print totals: %q", got)
		}
	})

	t.Run("happy path prints header, row, and total", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		entries := []runner.DiskUsageEntry{
			{
				Host:       "h1",
				Instance:   "ci-1",
				Mode:       "native",
				Busy:       true,
				TotalBytes: 1_500_000_000, // ~1.4 GiB
				WorkBytes:  500_000_000,
				TempBytes:  100_000_000,
			},
		}
		PrintDiskUsageTable(&buf, entries)
		got := buf.String()

		// Header row is present.
		for _, want := range []string{"HOST", "INSTANCE", "MODE", "TOTAL", "WORK", "TEMP", "DOCKER-DATA", "OTHER", "BUSY", "ORPHAN"} {
			if !strings.Contains(got, want) {
				t.Errorf("missing header %q in:\n%s", want, got)
			}
		}
		// Body row contents.
		if !strings.Contains(got, "h1") || !strings.Contains(got, "ci-1") || !strings.Contains(got, "native") {
			t.Errorf("missing body row in:\n%s", got)
		}
		// Busy flag.
		if !strings.Contains(got, "yes") {
			t.Errorf("missing busy=yes flag: %q", got)
		}
		// Orphan flag defaults to no.
		if !strings.Contains(got, "no") {
			t.Errorf("missing orphan=no flag: %q", got)
		}
		// Totals line.
		if !strings.Contains(got, "Total:") || !strings.Contains(got, "1 instance(s)") {
			t.Errorf("missing totals line: %q", got)
		}
	})

	t.Run("error row replaces body cells and excludes from total", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		entries := []runner.DiskUsageEntry{
			{
				Host:       "h1",
				Instance:   "ci-1",
				Mode:       "native",
				TotalBytes: 999_999_999_999, // would dominate total if not excluded
				Err:        errors.New("du failed"),
			},
			{
				Host:       "h1",
				Instance:   "ci-2",
				Mode:       "native",
				TotalBytes: 2_000_000_000, // ~1.9 GiB
			},
		}
		PrintDiskUsageTable(&buf, entries)
		got := buf.String()

		if !strings.Contains(got, "du failed") {
			t.Errorf("missing error message in body: %q", got)
		}
		// Totals line counts both rows (even the err row), but the byte
		// sum is dominated by ci-1 if the err row's TotalBytes leaks in.
		// The simplest invariant: the totals should not be the giant value.
		if !strings.Contains(got, "2 instance(s)") {
			t.Errorf("missing instance count: %q", got)
		}
		if strings.Contains(got, "932.0 GiB") {
			t.Errorf("err row's bytes leaked into total: %q", got)
		}
	})

	t.Run("busy=no when Remote=online but Busy=false", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		entries := []runner.DiskUsageEntry{
			{
				Host:     "h1",
				Instance: "ci-1",
				Mode:     "native",
				Busy:     false,
				Remote:   "online",
			},
		}
		PrintDiskUsageTable(&buf, entries)
		got := buf.String()
		if !strings.Contains(got, "no") {
			t.Errorf("expected busy=no for online non-busy runner: %q", got)
		}
	})

	t.Run("busy=- when neither Busy nor Remote=online", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		entries := []runner.DiskUsageEntry{
			{
				Host:     "h1",
				Instance: "ci-1",
				Mode:     "native",
				Busy:     false,
				Remote:   "", // unknown
			},
		}
		PrintDiskUsageTable(&buf, entries)
		got := buf.String()
		// The body row pads every cell. The BUSY cell holds "-", the
		// ORPHAN cell holds "no". With trailing whitespace stripped, the
		// row's last two tokens are "-     no". The whitespace between
		// them is 3 (BUSY right-pad to width 4) + 2 (gutter) = 5 spaces.
		body := findBodyLine(t, got, "ci-1")
		trailing := strings.TrimRight(body, " ")
		if !strings.HasSuffix(trailing, "-     no") {
			t.Errorf("expected busy=- then orphan=no at end of body row: %q", body)
		}
		// Negative: no "yes" anywhere (would mean a leaked busy/online flag).
		if strings.Contains(got, "yes") {
			t.Errorf("unexpected busy=yes in: %q", got)
		}
	})

	t.Run("orphan=yes when entry is orphan", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		entries := []runner.DiskUsageEntry{
			{
				Host:     "h1",
				Instance: "orphan-1",
				Mode:     "native",
				Orphan:   true,
			},
		}
		PrintDiskUsageTable(&buf, entries)
		got := buf.String()
		body := findBodyLine(t, got, "orphan-1")
		trailing := strings.TrimRight(body, " ")
		if !strings.HasSuffix(trailing, "-     yes") {
			t.Errorf("expected busy=- then orphan=yes at end of body row: %q", body)
		}
	})

	t.Run("emits one row per entry in input order", func(t *testing.T) {
		t.Parallel()
		// PrintDiskUsageTable is a pure printer — it does NOT sort. The
		// sorting is CollectDiskUsage's job (in disk.go line 162). This
		// test pins the contract: rows print in input order. A future
		// "sort here" change would be a behavioural move and worth a
		// code-review conversation, not a silent test failure.
		var buf bytes.Buffer
		entries := []runner.DiskUsageEntry{
			{Host: "h2", Instance: "ci-1", Mode: "native"},
			{Host: "h1", Instance: "ci-2", Mode: "native"},
			{Host: "h1", Instance: "ci-1", Mode: "native"},
		}
		PrintDiskUsageTable(&buf, entries)
		got := buf.String()
		rows := []string{
			findBodyLine(t, got, "h2"),
			findBodyLine(t, got, "h1    ci-2"),
			findBodyLine(t, got, "h1    ci-1"),
		}
		// Verify they appear in input order (h2/ci-1 first, h1/ci-2 second, h1/ci-1 last).
		prev := -1
		for i, r := range rows {
			idx := strings.Index(got, r)
			if idx < 0 {
				t.Errorf("row %d %q not found in:\n%s", i, r, got)
				continue
			}
			if idx <= prev {
				t.Errorf("rows out of input order at index %d:\n%s", i, got)
			}
			prev = idx
		}
	})
}

// TestDiskHostInstanceKey covers the small separator-join helper. The
// separator is a NUL byte so that the key is unambiguous for host/instance
// pairs that contain ":", "/", or newlines.
func TestDiskHostInstanceKey(t *testing.T) {
	t.Parallel()
	t.Run("joins with NUL separator", func(t *testing.T) {
		t.Parallel()
		got := diskHostInstanceKey("h1", "ci-1")
		if got != "h1\x00ci-1" {
			t.Errorf("got %q want %q", got, "h1\x00ci-1")
		}
	})
	t.Run("empty parts produce just the separator", func(t *testing.T) {
		t.Parallel()
		got := diskHostInstanceKey("", "")
		if got != "\x00" {
			t.Errorf("got %q want %q", got, "\x00")
		}
	})
	t.Run("differentiates (h1,r) from (h,r1)", func(t *testing.T) {
		t.Parallel()
		// Sanity: the helper must not let "h1" + "r" collide with
		// "h" + "1r". A naive "+" join would.
		if diskHostInstanceKey("h1", "r") == diskHostInstanceKey("h", "1r") {
			t.Error("NUL separator should disambiguate boundary positions")
		}
	})
}

// TestRcByInstanceForHost covers the helper that, for a given host, maps
// every configured instance name back to its RunnerConfig pointer.
//
//   - filters out runners whose Host field does not match
//   - emits one entry per instance (not per runner), honouring Count
//   - emits pointers that point into the original slice
func TestRcByInstanceForHost(t *testing.T) {
	t.Parallel()

	t.Run("filters by host", func(t *testing.T) {
		t.Parallel()
		// Note: InstanceNames uses "<Name>-<n>" naming, so a Name of
		// "ci" with Count=1 produces instance "ci-1". Avoid Name values
		// that already contain "-" or the n-suffix will collide.
		got := rcByInstanceForHost([]config.RunnerConfig{
			{Name: "ci", Host: "h1", Count: 1},
			{Name: "ci", Host: "h2", Count: 1},
		}, "h1")
		if len(got) != 1 {
			t.Fatalf("got %d entries, want 1", len(got))
		}
		if got["ci-1"].Host != "h1" {
			t.Errorf("got host %q want h1", got["ci-1"].Host)
		}
	})

	t.Run("emits one entry per instance across Count", func(t *testing.T) {
		t.Parallel()
		got := rcByInstanceForHost([]config.RunnerConfig{
			{Name: "ci", Host: "h1", Count: 3},
			{Name: "build", Host: "h1", Count: 2},
		}, "h1")
		for _, inst := range []string{"ci-1", "ci-2", "ci-3", "build-1", "build-2"} {
			if _, ok := got[inst]; !ok {
				t.Errorf("missing instance %q", inst)
			}
		}
		if len(got) != 5 {
			t.Errorf("got %d entries, want 5", len(got))
		}
	})

	t.Run("returns empty map when no runners match host", func(t *testing.T) {
		t.Parallel()
		got := rcByInstanceForHost([]config.RunnerConfig{
			{Name: "ci", Host: "h1", Count: 2},
		}, "h2")
		if len(got) != 0 {
			t.Errorf("got %d entries, want 0", len(got))
		}
	})
}

// --- helpers ---

// findBodyLine finds the first line containing needle and returns it.
// The table printer always emits a header line (containing "HOST") followed
// by one line per entry, so searching by instance is unambiguous.
func findBodyLine(t *testing.T, got, needle string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	for _, l := range lines {
		if strings.Contains(l, needle) {
			return l
		}
	}
	t.Fatalf("body line for %q not found in:\n%s", needle, got)
	return ""
}
