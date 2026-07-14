package tui

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/runner"
)

// BenchmarkFooterMain measures the per-View() cost of the dashboard footer.
// footerMain is called from viewMain once per Bubble Tea View() render —
// at every keypress and at every 5s refresh tick. The Sprintf path is
// reflection-based and allocates one buffer per call.
func BenchmarkFooterMain(b *testing.B) {
	cases := []struct {
		name    string
		loading bool
	}{
		{"idle", false},
		{"loading", true},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			m := &dashboardModel{loading: tc.loading}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = m.footerMain()
			}
		})
	}
}

// BenchmarkViewMain measures the full main-view render path with a small
// realistic status set. This is the integrated hot path the TUI hammers
// on every refresh tick + every keypress while the user navigates.
func BenchmarkViewMain(b *testing.B) {
	m := &dashboardModel{
		statuses: []runner.RunnerStatus{
			{
				Instance:              "runner-1",
				Host:                  "host1.example",
				Repo:                  "o/r1",
				Mode:                  "container",
				Local:                 "running",
				ContainerImageBuild:   "ok (deadbeef)",
				Remote:                "online",
				Labels:                "self-hosted,linux,x64",
				ContainerImage:        "gh-sr/agentic-runner:2.320.0",
				ContainerImageRevision: "abc123",
			},
		},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = m.viewMain()
	}
}