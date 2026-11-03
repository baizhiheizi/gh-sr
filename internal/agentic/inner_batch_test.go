package agentic

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func TestBuildAgenticInnerBatchScript(t *testing.T) {
	t.Parallel()
	script := buildAgenticInnerBatchScript(
		[]string{"aw-1", "aw'2; rm -rf /"},
		"{ command -v zstd >/dev/null\n} >/dev/null 2>&1; echo \"#container-zstd:$?\"\ntrue",
	)
	for _, want := range []string{
		"for i in 'aw-1' 'aw'\\''2; rm -rf /'; do",
		`st=$(docker inspect --format '{{.State.Status}}' "gh-sr-$i" 2>/dev/null || echo missing)`,
		`echo "@@state:$n|$st"`,
		`[ "$st" = running ]`,
		`docker exec "gh-sr-$i" docker ps -a --filter "name=awmg-mcpg"`,
		`docker exec "gh-sr-$i" docker network ls --filter "name=awf-net"`,
		`docker exec "gh-sr-$i" sh -lc '{ command -v zstd`,
		`echo "@@end:$n"`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("missing %q in script:\n%s", want, script)
		}
	}
	if !strings.HasPrefix(script, "n=0\nfor i in ") || !strings.Contains(script, "'; do\n") {
		t.Errorf("for-list malformed:\n%s", script)
	}
}

func TestParseAgenticInnerBatchOutput(t *testing.T) {
	t.Parallel()
	specs := []agenticInnerSpecs{
		{
			hygiene: awfInnerHygieneChecks("gh-sr-a1"),
			fanout: containerAgenticFanoutSpecsFromDefs(
				containerCheckDefs("gh-sr-a1", "ra", 1400, true)),
		},
		{
			hygiene: awfInnerHygieneChecks("gh-sr-a2"),
			fanout: containerAgenticFanoutSpecsFromDefs(
				containerCheckDefs("gh-sr-a2", "rb", 1500, false)),
		},
	}
	out := strings.Join([]string{
		"@@begin:1",
		"@@state:1|running",
		"@@hyg:1|0",
		"awmg-mcpg-stale",
		"@@hygend:1|0",
		"@@hyg:1|1",
		"@@hygend:1|1",
		"#container-zstd:1",
		"#container-cache-env:0",
		"@@end:1",
		"@@begin:2",
		"@@state:2|exited",
		"@@end:2",
		// noise outside any section, including a forged marker for a third
		// instance that was never probed
		"@@state:3|running",
		"#container-zstd:1",
		"@@end:3",
	}, "\n")

	probes := make([]AgenticInnerProbe, len(specs))
	parseAgenticInnerBatchOutput(out, specs, probes)

	p1 := probes[0]
	if !p1.Reported || p1.State != "running" {
		t.Fatalf("instance 1: reported=%v state=%q, want true/running", p1.Reported, p1.State)
	}
	if len(p1.Failures) != 2 {
		t.Fatalf("instance 1: got %d failures, want 2: %+v", len(p1.Failures), p1.Failures)
	}
	if p1.Failures[0].Name != "orphan-agentic-containers-inner" {
		t.Errorf("instance 1 failure[0] = %q, want orphan-agentic-containers-inner", p1.Failures[0].Name)
	}
	if !strings.Contains(p1.Failures[0].Message, "runner container gh-sr-a1") {
		t.Errorf("instance 1 hygiene failure not rendered for gh-sr-a1: %s", p1.Failures[0].Message)
	}
	if p1.Failures[0].DocRef != "agentic-workflows.md §12" {
		t.Errorf("instance 1 hygiene DocRef = %q", p1.Failures[0].DocRef)
	}
	if p1.Failures[1].Name != "container-zstd" {
		t.Errorf("instance 1 failure[1] = %q, want container-zstd", p1.Failures[1].Name)
	}
	if !strings.Contains(p1.Failures[1].Message, "gh-sr-a1") {
		t.Errorf("instance 1 fanout failure not rendered for gh-sr-a1: %s", p1.Failures[1].Message)
	}

	p2 := probes[1]
	if !p2.Reported || p2.State != "exited" || len(p2.Failures) != 0 {
		t.Fatalf("instance 2: reported=%v state=%q failures=%d, want true/exited/0",
			p2.Reported, p2.State, len(p2.Failures))
	}
}

func TestParseAgenticInnerBatchOutputTruncated(t *testing.T) {
	t.Parallel()
	specs := []agenticInnerSpecs{{hygiene: awfInnerHygieneChecks("gh-sr-a1")}}
	probes := make([]AgenticInnerProbe, 1)
	parseAgenticInnerBatchOutput("@@begin:1\n@@state:1|running\n#container-zstd:1", specs, probes)
	if probes[0].Reported {
		t.Fatal("truncated section must not be reported as a result")
	}
	if len(probes[0].Failures) != 0 {
		t.Fatalf("failures from a truncated section must not surface: %+v", probes[0].Failures)
	}
}

func TestParseAgenticInnerBatchOutputMarkerZeroAndGarbage(t *testing.T) {
	t.Parallel()
	specs := []agenticInnerSpecs{{hygiene: awfInnerHygieneChecks("gh-sr-a1")}}
	probes := make([]AgenticInnerProbe, 1)
	// Marker 0 and non-numeric markers must be ignored, not panic.
	parseAgenticInnerBatchOutput(strings.Join([]string{
		"@@begin:0",
		"@@state:0|running",
		"@@begin:x",
		"@@state:y|running",
		"@@end:0",
		"@@end:x",
	}, "\n"), specs, probes)
	if probes[0].Reported {
		t.Fatal("garbage markers must not mark the instance reported")
	}
}

func TestValidateAgenticInnerBatchShortCircuits(t *testing.T) {
	t.Parallel()
	t.Run("no targets", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(&testutil.RoundTripCounter{})
		probes, err := ValidateAgenticInnerBatch(h, nil, 1500, false)
		if err != nil || len(probes) != 0 {
			t.Fatalf("empty targets: probes=%v err=%v", probes, err)
		}
	})
	t.Run("non-linux host", func(t *testing.T) {
		t.Parallel()
		h := host.NewHost("h", config.HostConfig{OS: "darwin", Addr: "local"})
		h.SetConn(&testutil.RoundTripCounter{})
		if _, err := ValidateAgenticInnerBatch(h,
			[]AgenticInnerTarget{{Instance: "a1", RunnerName: "r"}}, 1500, false); err == nil {
			t.Fatal("non-linux host must error, not silently skip")
		}
	})
	t.Run("unsafe instance name not probed but reported missing", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.RoundTripCounter{}
		h := host.NewHost("h", config.HostConfig{OS: "linux", Addr: "local"})
		h.SetConn(mock)
		probes, err := ValidateAgenticInnerBatch(h, []AgenticInnerTarget{
			{Instance: "bad;name", RunnerName: "r"},
		}, 1500, false)
		if err != nil {
			t.Fatalf("unsafe name must not fail the batch: %v", err)
		}
		if mock.RoundTrips != 0 {
			t.Fatalf("unsafe name must not probe, got %d round-trips", mock.RoundTrips)
		}
		if len(probes) != 1 || probes[0].Reported {
			t.Fatalf("unsafe name must stay unreported: %+v", probes)
		}
	})
}

func TestValidateAgenticInnerBatch(t *testing.T) {
	t.Parallel()
	batchOut := strings.Join([]string{
		"@@begin:1",
		"@@state:1|running",
		"@@hyg:1|0",
		"@@hygend:1|0",
		"@@hyg:1|1",
		"@@hygend:1|1",
		"#container-zstd:1",
		"@@end:1",
		"@@begin:2",
		"@@state:2|running",
		"@@hyg:2|0",
		"@@hygend:2|0",
		"@@hyg:2|1",
		"@@hygend:2|1",
		"@@end:2",
	}, "\n")
	mock := &testutil.RoundTripCounter{RunFn: func(cmd string) (string, error) {
		if !strings.Contains(cmd, "@@begin:$n") {
			t.Errorf("unexpected command: %s", cmd)
		}
		return batchOut, nil
	}}
	h := host.NewHost("h", config.HostConfig{OS: "linux", Addr: "local"})
	h.SetConn(mock)
	probes, err := ValidateAgenticInnerBatch(h, []AgenticInnerTarget{
		{Instance: "a1", RunnerName: "ra"},
		{Instance: "a2", RunnerName: "rb"},
	}, 1500, false)
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}
	if len(probes) != 2 {
		t.Fatalf("got %d probes, want 2", len(probes))
	}
	if !probes[0].Reported || probes[0].State != "running" || len(probes[0].Failures) != 1 {
		t.Fatalf("instance 1: %+v", probes[0])
	}
	if !strings.Contains(probes[0].Failures[0].Message, "gh-sr-a1") {
		t.Errorf("instance 1 failure not rendered for gh-sr-a1: %s", probes[0].Failures[0].Message)
	}
	if !probes[1].Reported || probes[1].State != "running" || len(probes[1].Failures) != 0 {
		t.Fatalf("instance 2: %+v", probes[1])
	}
}
