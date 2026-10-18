package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
)

// widthFixtures are the terminal widths the responsive-layout policy
// targets. 30 is the narrowest width every panel is expected to honour;
// below it the prose renderers' max(20, width) floors and packSegments'
// "a single segment wider than the width stays on its own line" rule take
// over by design.
var widthFixtures = []int{30, 40, 45, 60, 80, 100, 120, 200}

// overflowFixtureStatuses uses deliberately long values (instance names,
// hostnames, image refs, labels) so the natural widths land where a real
// fleet's do — a fixture of short values would pass without ever
// exercising truncation or dropping.
func overflowFixtureStatuses() []runner.RunnerStatus {
	return []runner.RunnerStatus{
		{
			Instance:            "prod-runner-with-a-fairly-long-instance-name-01",
			Host:                "host1.very-long-hostname.example.internal",
			Repo:                "organisation-with-long-name/extremely-long-repository-name",
			Mode:                "container",
			ContainerImage:      "gh-sr/agentic-runner:2.320.0-20240101",
			ContainerImageBuild: "sha256:abcdef0123456789",
			Local:               "running",
			Remote:              "online",
			Labels:              "self-hosted,linux,x64,production,region-eu-west-1",
		},
		{
			Instance: "runner-2",
			Host:     "host2",
			Repo:     "o/r",
			Mode:     "shell",
			Local:    "stopped",
			Remote:   "offline",
			Labels:   "self-hosted",
		},
	}
}

func overflowFixtureMetrics() []host.HostMetrics {
	return []host.HostMetrics{
		{
			Name:       "host1.very-long-hostname.example.internal",
			CPUPercent: 12.5,
			MemUsedMiB: 3174, MemTotalMiB: 15974,
			DiskUsedGiB: 120, DiskTotalGiB: 512,
			Load1: 0.52, Load5: 0.61, Load15: 0.70,
			Uptime: "18 days, 4:21",
		},
		{
			Name:       "host2",
			CPUPercent: 99.9,
			MemUsedMiB: 14336, MemTotalMiB: 15974,
			DiskUsedGiB: 480, DiskTotalGiB: 512,
			Load1: 8.10, Load5: 7.90, Load15: 6.80,
			Uptime: "2 hours",
		},
	}
}

// assertLinesFit fails when any rendered line is wider than the terminal.
// lipgloss.Width measures the visible cell width with ANSI escapes
// accounted for, which is what the terminal actually consumes.
func assertLinesFit(t *testing.T, label, content string, width int) {
	t.Helper()
	for i, line := range strings.Split(content, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Errorf("%s @ %d cols: line %d is %d cols wide: %q", label, width, i, got, line)
		}
	}
}

// TestViewMain_fitsTerminalWidth is the end-to-end guard for the
// responsive-layout policy: whatever the terminal width, no line of the
// main panel may exceed it. Tables that keep only pinned columns used to
// overflow here (the runner table ran 53 columns wide in a 45-column
// terminal), and the filter line's long repo slug was emitted verbatim
// because wrapWordsToWidth had no break opportunity inside a word.
func TestViewMain_fitsTerminalWidth(t *testing.T) {
	t.Parallel()
	for _, width := range widthFixtures {
		for _, tc := range []struct {
			name  string
			model *dashboardModel
		}{
			{
				name: "plain",
				model: &dashboardModel{
					statuses: overflowFixtureStatuses(),
					panel:    panelMain,
				},
			},
			{
				name: "filters",
				model: &dashboardModel{
					statuses:      overflowFixtureStatuses(),
					tuiHostFilter: "host1.very-long-hostname.example.internal",
					tuiRepoFilter: "organisation-with-long-name/extremely-long-repository-name",
					panel:         panelMain,
				},
			},
			{
				name: "help-overlay",
				model: &dashboardModel{
					statuses: overflowFixtureStatuses(),
					showHelp: true,
					panel:    panelMain,
				},
			},
			{
				name: "error",
				model: &dashboardModel{
					statuses: overflowFixtureStatuses(),
					lastErr:  strings.Repeat("something went wrong talking to the remote host at /var/lib/gh-sr/runners/runner-01; ", 3),
					panel:    panelMain,
				},
			},
			{
				name: "toast",
				model: &dashboardModel{
					statuses: overflowFixtureStatuses(),
					toast:    strings.Repeat("runner setup completed successfully; ", 4),
					panel:    panelMain,
				},
			},
			{
				name: "empty",
				model: &dashboardModel{
					statuses: nil,
					panel:    panelMain,
				},
			},
		} {
			tc.model.width = width
			tc.model.height = 50
			assertLinesFit(t, "main/"+tc.name, tc.model.viewMain().Content, width)
		}
	}
}

// TestViewHostMetrics_fitsTerminalWidth guards the same invariant for the
// host-metrics panel, which overflowed a 60-column terminal at 64 columns
// wide because CPU/MEMORY/DISK were pinned and nothing could be dropped.
func TestViewHostMetrics_fitsTerminalWidth(t *testing.T) {
	t.Parallel()
	for _, width := range widthFixtures {
		m := &dashboardModel{
			width:       width,
			height:      50,
			hostMetrics: overflowFixtureMetrics(),
			panel:       panelHostMetrics,
		}
		assertLinesFit(t, "host-metrics", m.viewHostMetrics().Content, width)
	}
}

// TestViewScroll_fitsTerminalWidth guards the scroll panel's hint line,
// a fixed-width string that hard-wrapped at the widths this policy
// claims to support.
func TestViewScroll_fitsTerminalWidth(t *testing.T) {
	t.Parallel()
	raw := strings.Repeat("a representative log line with some detail in it\n", 40)
	for _, width := range widthFixtures {
		for _, off := range []int{0, 12} {
			m := &dashboardModel{width: width, height: 24, panel: panelScroll}
			m.setScroll("Logs", raw)
			m.scrollOff = off
			assertLinesFit(t, "scroll", m.viewScroll().Content, width)
		}
	}
}

// TestRenderTitleLine_suffixStaysOnTitleLine pins the right-aligned
// "<n> runners" suffix. It previously rendered on the line below the
// title because the suffix went through helpStyle, whose MarginTop(1)
// prepends a newline to the styled string.
func TestRenderTitleLine_suffixStaysOnTitleLine(t *testing.T) {
	t.Parallel()
	m := &dashboardModel{width: 80}
	got := m.renderTitleLine("gh sr dashboard", "2 runners")

	if strings.Contains(got, "\n") {
		t.Fatalf("title line must stay on one line, got %q", got)
	}
	if !strings.Contains(visibleText(got), "2 runners") {
		t.Errorf("title line must carry the count suffix, got %q", visibleText(got))
	}
	if w := lipgloss.Width(got); w != 80-3 {
		t.Errorf("suffix must be right-aligned at the terminal edge: width = %d, want %d", w, 80-3)
	}
}

// TestRenderTitleLine_dropsSuffixWhenItCannotFit keeps a long suffix from
// wrapping over the title on a narrow terminal.
func TestRenderTitleLine_dropsSuffixWhenItCannotFit(t *testing.T) {
	t.Parallel()
	m := &dashboardModel{width: 12}
	if got := m.renderTitleLine("gh sr dashboard", "2 runners"); strings.Contains(got, "runners") {
		t.Errorf("suffix must be dropped when it cannot share the title line, got %q", got)
	}
}

// TestCountLabel_pluralises guards the panel-title count, which read
// "1 runners" / "1 hosts" for a single-row table.
func TestCountLabel_pluralises(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		n    int
		want string
	}{
		{0, "0 runners"},
		{1, "1 runner"},
		{2, "2 runners"},
	} {
		if got := countLabel(tc.n, "runner"); got != tc.want {
			t.Errorf("countLabel(%d, \"runner\") = %q, want %q", tc.n, got, tc.want)
		}
	}
	if got := countLabel(1, "host"); got != "1 host" {
		t.Errorf("countLabel(1, \"host\") = %q, want %q", got, "1 host")
	}
}

// TestWrapWordsToWidth_breaksOverlongWord pins the hard break for a single
// token wider than the wrap width — repo slugs, hostnames and paths carry
// no spaces, so greedy packing alone emitted them verbatim.
func TestWrapWordsToWidth_breaksOverlongWord(t *testing.T) {
	t.Parallel()
	const long = "organisation-with-long-name/extremely-long-repository-name"
	for _, width := range []int{12, 20, 40} {
		for _, line := range wrapWordsToWidth("Filters: repo="+long, width) {
			if len(line) > width {
				t.Errorf("width %d: line %q is %d bytes", width, line, len(line))
			}
		}
	}
	// The broken-up text must still contain the original token.
	var joined string
	for _, line := range wrapWordsToWidth(long, 20) {
		joined += line
	}
	if joined != long {
		t.Errorf("hard wrap must preserve the word: got %q, want %q", joined, long)
	}
}

// TestSplitOverlongWords_runeSafe guards against splitting a multi-byte
// character across the break.
func TestSplitOverlongWords_runeSafe(t *testing.T) {
	t.Parallel()
	got := splitOverlongWords([]string{"日本語のホスト名です"}, 4)
	for _, chunk := range got {
		if !utf8ValidString(chunk) {
			t.Errorf("chunk %q is not valid UTF-8", chunk)
		}
	}
	var joined string
	for _, chunk := range got {
		joined += chunk
	}
	if joined != "日本語のホスト名です" {
		t.Errorf("chunks must rejoin to the original, got %q", joined)
	}
}

// TestSplitOverlongWords_zeroWidthSpansLeadingRune pins the defensive
// cut==0 branch. With width=0 the loop never decrements cut (it would
// require cut > 0), so we fall through to DecodeRuneInString and emit
// the leading rune unchanged. The function must still make progress —
// otherwise it would loop forever. The resulting chunks must rejoin to
// the input exactly.
func TestSplitOverlongWords_zeroWidthSpansLeadingRune(t *testing.T) {
	t.Parallel()
	got := splitOverlongWords([]string{"hello"}, 0)
	var joined string
	for _, chunk := range got {
		joined += chunk
	}
	if joined != "hello" {
		t.Errorf("chunks must rejoin to the original, got %q", got)
	}
}

// visibleText strips ANSI escape sequences so assertions can target the
// rendered text rather than the styled bytes.
func visibleText(s string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inEscape {
			if c == 'm' {
				inEscape = false
			}
			continue
		}
		if c == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func utf8ValidString(s string) bool {
	for _, r := range s {
		if r == 0xFFFD {
			return false
		}
	}
	return true
}
