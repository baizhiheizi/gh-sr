package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/runner"
)

// planFixture mirrors a realistic status set: one wide row with a long
// image reference and labels plus a compact row, so natural widths land
// where the real dashboard puts them (IMAGE ~29, LABELS ~20).
func planFixture() []runner.RunnerStatus {
	return []runner.RunnerStatus{
		{
			Instance:            "runner-1",
			Host:                "host1.example",
			Repo:                "org/backend",
			Mode:                "container",
			ContainerImage:      "gh-sr/agentic-runner:2.320.0",
			ContainerImageBuild: "-",
			Local:               "running",
			Remote:              "online",
			Labels:              "self-hosted,linux,x64",
		},
		{
			Instance: "r2",
			Host:     "vps",
			Repo:     "org/web",
			Mode:     "native",
			Local:    "stopped",
			Labels:   "linux",
		},
	}
}

func runnerSpecs() []columnSpec {
	specs := make([]columnSpec, len(runnerStatusColumns))
	for i, c := range runnerStatusColumns {
		specs[i] = c.columnSpec
	}
	return specs
}

func naturalWidths(statuses []runner.RunnerStatus) []int {
	natural := make([]int, len(runnerStatusColumns))
	for i, c := range runnerStatusColumns {
		natural[i] = len(c.header)
		for _, s := range statuses {
			if l := len(c.cell(s)); l > natural[i] {
				natural[i] = l
			}
		}
	}
	return natural
}

func keptHeaders(kept []int) []string {
	out := make([]string, len(kept))
	for k, i := range kept {
		out[k] = runnerStatusColumns[i].header
	}
	return out
}

// TestPlanColumns_wideKeepsEverything verifies a wide terminal keeps all
// nine columns at natural width — no truncation when everything fits.
func TestPlanColumns_wideKeepsEverything(t *testing.T) {
	t.Parallel()
	statuses := planFixture()
	natural := naturalWidths(statuses)
	kept, widths := planColumns(runnerSpecs(), natural, 200)
	if len(kept) != len(runnerStatusColumns) {
		t.Fatalf("wide terminal: kept %d columns, want all %d", len(kept), len(runnerStatusColumns))
	}
	if !reflect.DeepEqual(widths, natural) {
		t.Errorf("wide terminal widths = %v, want natural %v", widths, natural)
	}
}

// TestPlanColumns_mediumTruncatesBeforeDropping pins the
// truncate-before-drop policy: at a width where truncation can absorb the
// deficit, every column is kept — IMAGE shrinks (29 → 13) instead of being
// dropped, and no other column loses width.
func TestPlanColumns_mediumTruncatesBeforeDropping(t *testing.T) {
	t.Parallel()
	statuses := planFixture()
	natural := naturalWidths(statuses)
	// total natural = 8+13+11+9+29+5+7+6+20 + 9*2 = 126; avail 110 → the
	// IMAGE column alone absorbs the 16-column deficit.
	kept, widths := planColumns(runnerSpecs(), natural, 110)
	if len(kept) != len(runnerStatusColumns) {
		t.Fatalf("avail 110: kept %d columns, want all %d (truncation beats dropping)", len(kept), len(runnerStatusColumns))
	}
	for k, i := range kept {
		if runnerStatusColumns[i].header == "IMAGE" {
			if widths[k] >= natural[i] {
				t.Errorf("IMAGE width = %d, want truncated below natural %d", widths[k], natural[i])
			}
		} else if widths[k] != natural[i] {
			t.Errorf("%s width = %d, want natural %d (only IMAGE should shrink)", runnerStatusColumns[i].header, widths[k], natural[i])
		}
	}
	if total := tableTotal(widths); total > 110 {
		t.Errorf("table total %d exceeds avail 110", total)
	}
}

// TestPlanColumns_narrowDropsWidestFirst verifies a 60-column terminal
// sheds the wide optional columns (LABELS, then IMAGE) while keeping the
// operational core — INSTANCE/HOST/REPO/MODE/BUILD/LOCAL/GITHUB —
// truncated to readable floors.
func TestPlanColumns_narrowDropsWidestFirst(t *testing.T) {
	t.Parallel()
	statuses := planFixture()
	natural := naturalWidths(statuses)
	kept, widths := planColumns(runnerSpecs(), natural, 60)
	want := []string{"INSTANCE", "HOST", "REPO", "MODE", "BUILD", "LOCAL", "GITHUB"}
	if got := keptHeaders(kept); !reflect.DeepEqual(got, want) {
		t.Errorf("avail 60 kept %v, want %v", got, want)
	}
	// The kept table must actually fit: cells + 2 padding per column.
	if total := tableTotal(widths); total > 60 {
		t.Errorf("kept table total %d exceeds avail 60 (widths %v)", total, widths)
	}
}

// TestPlanColumns_narrowTruncatesFlexCells verifies flex columns truncate
// with an ellipsis (via truncateCell at the planned width) while IMAGE is
// dropped entirely and pinned status columns keep their full text.
func TestPlanColumns_narrowTruncatesFlexCells(t *testing.T) {
	t.Parallel()
	statuses := planFixture()
	natural := naturalWidths(statuses)
	kept, widths := planColumns(runnerSpecs(), natural, 60)
	for _, i := range kept {
		if runnerStatusColumns[i].header == "IMAGE" {
			t.Fatalf("IMAGE should be dropped at avail 60")
		}
	}

	// HOST is flex; with avail 60 the natural host column (13) shrinks.
	var hostWidth int
	for k, i := range kept {
		if runnerStatusColumns[i].header == "HOST" {
			hostWidth = widths[k]
		}
	}
	if hostWidth >= 13 {
		t.Errorf("HOST column should be truncated below its natural 13 at avail 60, got %d", hostWidth)
	}
	truncated := truncateCell("host1.example", hostWidth)
	if !strings.HasSuffix(truncated, "…") {
		t.Errorf("truncated HOST cell %q should end with an ellipsis", truncated)
	}
}

// TestPlanColumns_degenerateAvailKeepsColumns pins the guard for
// avail <= 0 (no WindowSizeMsg yet): every column is kept at natural
// width rather than being truncated against an unknown terminal.
func TestPlanColumns_degenerateAvailKeepsColumns(t *testing.T) {
	t.Parallel()
	natural := []int{8, 13, 11, 9, 29, 1, 7, 6, 20}
	kept, widths := planColumns(runnerSpecs(), natural, 0)
	if len(kept) != 9 {
		t.Fatalf("avail 0 kept %d columns, want all 9", len(kept))
	}
	if !reflect.DeepEqual(widths, natural) {
		t.Errorf("avail 0 widths = %v, want natural %v", widths, natural)
	}
}

// TestPlanColumns_metricsUptimeDroppedLast verifies the host-metrics
// policy: at a width where even the flex floors do not fit, UPTIME (the
// only droppable column) goes and CPU/MEMORY/DISK survive.
func TestPlanColumns_metricsUptimeDroppedLast(t *testing.T) {
	t.Parallel()
	natural := []int{13, 6, 25, 22, 14, 7}
	kept, widths := planColumns(hostMetricsSpecs, natural, 74)
	var headers []string
	for _, i := range kept {
		headers = append(headers, hostMetricsHeaders[i])
	}
	want := []string{"HOST", "CPU", "MEMORY", "DISK", "LOAD AVG"}
	if !reflect.DeepEqual(headers, want) {
		t.Fatalf("avail 74 kept %v, want %v", headers, want)
	}
	if total := tableTotal(widths); total > 74 {
		t.Errorf("metrics table total %d exceeds avail 74 (widths %v)", total, widths)
	}
}

// TestRunnerStatusAccessorsDerivedFromColumns pins the CLI contract from
// openspec/specs/tui-table-rendering: nine cells in fixed order, empty
// image fields as "-", GitHub status via formatGitHubStatus, and headers
// matching the column table — all now derived from runnerStatusColumns.
func TestRunnerStatusAccessorsDerivedFromColumns(t *testing.T) {
	t.Parallel()
	if len(runnerStatusHeaders) != 9 || len(runnerStatusColumns) != 9 {
		t.Fatalf("runner status table must have 9 columns, got headers=%d columns=%d", len(runnerStatusHeaders), len(runnerStatusColumns))
	}
	wantHeaders := []string{"INSTANCE", "HOST", "REPO", "MODE", "IMAGE", "BUILD", "LOCAL", "GITHUB", "LABELS"}
	if !reflect.DeepEqual(runnerStatusHeaders, wantHeaders) {
		t.Errorf("runnerStatusHeaders = %v, want %v", runnerStatusHeaders, wantHeaders)
	}

	s := runner.RunnerStatus{
		Instance: "i", Host: "h", Repo: "r", Mode: "m",
		Local: "running", Remote: "online", Labels: "l1,l2",
	}
	cells := runnerStatusCells(s)
	want := []string{"i", "h", "r", "m", "-", "-", "running", "online", "l1,l2"}
	if !reflect.DeepEqual(cells, want) {
		t.Errorf("runnerStatusCells = %v, want %v", cells, want)
	}

	// Colorize maps by derived column: LOCAL (6) and GITHUB (7) colorize,
	// INSTANCE (0) passes through.
	if runnerStatusColorize(0, "x") != "x" {
		t.Errorf("INSTANCE column must pass through uncolored")
	}
	if !strings.Contains(runnerStatusColorize(6, "running"), "running") {
		t.Errorf("LOCAL column colorize must keep the cell text")
	}
	if !strings.Contains(runnerStatusColorize(7, "online"), "online") {
		t.Errorf("GITHUB column colorize must keep the cell text")
	}
	if runnerStatusColorize(-1, "x") != "x" || runnerStatusColorize(99, "x") != "x" {
		t.Errorf("out-of-range columns must pass through uncolored")
	}
}

// TestTruncateCell covers the ellipsis truncation used by responsive
// tables: byte-exact fits pass through, overflows cut at rune boundaries,
// and degenerate widths collapse safely.
func TestTruncateCell(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		s    string
		w    int
		want string
	}{
		{"shorter passes through", "abc", 10, "abc"},
		{"exact fit passes through", "abcdef", 6, "abcdef"},
		{"one over cuts with ellipsis", "abcdef", 5, "abcd…"},
		{"wide cut", "gh-sr/agentic-runner:2.320.0", 8, "gh-sr/a…"},
		{"rune-safe cut", "héllo wörld", 6, "héll…"},
		{"width one collapses to ellipsis", "abcdef", 1, "…"},
		{"zero width empty", "abcdef", 0, ""},
		{"negative width empty", "abcdef", -3, ""},
		{"empty cell", "", 4, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := truncateCell(tc.s, tc.w); got != tc.want {
				t.Errorf("truncateCell(%q, %d) = %q, want %q", tc.s, tc.w, got, tc.want)
			}
		})
	}
}

// TestPackSegments_wrapsToWidth verifies the footer hint packer keeps
// every line within the requested width and preserves segment order.
func TestPackSegments_wrapsToWidth(t *testing.T) {
	t.Parallel()
	segs := []string{"aaaa", "bbbbbbbb", "cc", "dddd"}
	got := packSegments(segs, 15)
	for i, line := range got {
		if len(line) > 15 {
			t.Errorf("line %d = %q exceeds width 15", i, line)
		}
	}
	var joined []string
	for _, line := range got {
		joined = append(joined, strings.Split(line, "  ")...)
	}
	if !reflect.DeepEqual(joined, segs) {
		t.Errorf("packed segments %v do not preserve order/content %v", joined, segs)
	}
}

// TestPackSegments_singleLineWhenWide verifies no wrapping happens when
// everything fits — the wide-terminal footer stays on one line.
func TestPackSegments_singleLineWhenWide(t *testing.T) {
	t.Parallel()
	segs := []string{"j/k: move", "q: quit"}
	got := packSegments(segs, 80)
	if len(got) != 1 || got[0] != "j/k: move  q: quit" {
		t.Errorf("packSegments at width 80 = %v, want single joined line", got)
	}
}

func TestPackSegments_empty(t *testing.T) {
	t.Parallel()
	if got := packSegments(nil, 40); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("packSegments(nil) = %v, want [\"\"]", got)
	}
}

// TestWrapWordsToWidth verifies prose wrapping: breaks on spaces only,
// preserves blank lines, and collapses runs of spaces.
func TestWrapWordsToWidth(t *testing.T) {
	t.Parallel()
	got := wrapWordsToWidth("the quick brown fox  jumps", 10)
	want := []string{"the quick", "brown fox", "jumps"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapWordsToWidth = %v, want %v", got, want)
	}
}

func TestWrapWordsToWidth_preservesBlankLines(t *testing.T) {
	t.Parallel()
	got := wrapWordsToWidth("a\n\nb", 40)
	want := []string{"a", "", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("wrapWordsToWidth = %v, want %v", got, want)
	}
}

func TestWrapWordsToWidth_degenerateWidthFloored(t *testing.T) {
	t.Parallel()
	got := wrapWordsToWidth("some words here", 0)
	for _, line := range got {
		if len(line) > 8 {
			t.Errorf("line %q exceeds the floored width 8", line)
		}
	}
}

// TestFooterMain_wrapsOnNarrowTerminal drives the dashboard footer at a
// narrow width: every visible line must fit the terminal and the loading
// indicator must only appear while loading.
func TestFooterMain_wrapsOnNarrowTerminal(t *testing.T) {
	t.Parallel()
	m := &dashboardModel{width: 50}

	idle := containsLinesWithin(t, m.footerMain(), 50)
	m.loading = true
	loading := containsLinesWithin(t, m.footerMain(), 50)

	if idle == loading {
		t.Fatal("idle and loading footers must differ at narrow width too")
	}
	if !strings.Contains(loading, "refreshing") {
		t.Error("loading footer must carry the refreshing indicator")
	}
	if strings.Contains(idle, "refreshing") {
		t.Error("idle footer must not carry the refreshing indicator")
	}
}

// containsLinesWithin asserts every styled line in s fits width (after
// stripping ANSI escapes) and returns the visible text for follow-up
// assertions.
func containsLinesWithin(t *testing.T, s string, width int) string {
	t.Helper()
	var visible []string
	inEscape := false
	var cur strings.Builder
	flush := func() {
		visible = append(visible, cur.String())
		cur.Reset()
	}
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
		if c == '\n' {
			flush()
			continue
		}
		cur.WriteByte(c)
	}
	flush()
	for i, line := range visible {
		if len(line) > width {
			t.Errorf("footer line %d = %q (%d cols) exceeds terminal width %d", i, line, len(line), width)
		}
	}
	return strings.Join(visible, "\n")
}
