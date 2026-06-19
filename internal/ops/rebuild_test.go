package ops

import (
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
)

func TestPartitionRebuildTargets(t *testing.T) {
	t.Parallel()

	native := config.RunnerConfig{Name: "n", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeNative}
	emptyMode := config.RunnerConfig{Name: "d", Host: "h1", Repo: "o/r"} // defaults to native
	container := config.RunnerConfig{Name: "c", Host: "h1", Repo: "o/r", RunnerMode: config.RunnerModeContainer}

	tests := []struct {
		name          string
		in            []config.RunnerConfig
		wantContainer []string
		wantSkipped   []string
	}{
		{
			name:          "empty",
			in:            nil,
			wantContainer: nil,
			wantSkipped:   nil,
		},
		{
			name:          "all native",
			in:            []config.RunnerConfig{native, emptyMode},
			wantContainer: nil,
			wantSkipped:   []string{"n", "d"},
		},
		{
			name:          "all container",
			in:            []config.RunnerConfig{container},
			wantContainer: []string{"c"},
			wantSkipped:   nil,
		},
		{
			name:          "mixed order preserved per slice",
			in:            []config.RunnerConfig{container, native, container},
			wantContainer: []string{"c", "c"},
			wantSkipped:   []string{"n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotC, gotS := partitionRebuildTargets(tt.in)
			if len(gotC) != len(tt.wantContainer) {
				t.Fatalf("container len got %d want %d", len(gotC), len(tt.wantContainer))
			}
			for i, w := range tt.wantContainer {
				if gotC[i].Name != w {
					t.Errorf("container[%d] name got %q want %q", i, gotC[i].Name, w)
				}
			}
			if len(gotS) != len(tt.wantSkipped) {
				t.Fatalf("skipped len got %d want %d", len(gotS), len(tt.wantSkipped))
			}
			for i, w := range tt.wantSkipped {
				if gotS[i].Name != w {
					t.Errorf("skipped[%d] name got %q want %q", i, gotS[i].Name, w)
				}
			}
		})
	}
}
