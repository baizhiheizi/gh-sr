package probe

import (
	"fmt"
	"strings"
	"testing"
)

// capture records every dispatch a parser makes, in order — the test surface
// for the whole grammar below.
type capture struct {
	frame    []int
	frameEnd []int
	values   []string
	captured []string
	lines    []string
}

func (c *capture) handlers() Handlers {
	return Handlers{
		Frame:    func(n int) { c.frame = append(c.frame, n) },
		FrameEnd: func(n int) { c.frameEnd = append(c.frameEnd, n) },
		Value: func(n int, marker, tail string) {
			c.values = append(c.values, fmt.Sprintf("%d %s %q", n, marker, tail))
		},
		Captured: func(n int, marker string, k int, text string) {
			c.captured = append(c.captured, fmt.Sprintf("%d %s %d %q", n, marker, k, text))
		},
		Line: func(n int, line string) { c.lines = append(c.lines, fmt.Sprintf("%d %q", n, line)) },
	}
}

func TestWriterEmitsProtocolLines(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	w := NewWriter(&b)
	w.Frame("@@begin", 1)
	w.Value("@@state", 1, "$st")
	w.Value("S", 1, `$(du -sb "$storage" 2>/dev/null | cut -f1)`)
	w.CaptureOpen("@@hyg", 2, 0)
	w.CaptureClose("@@hygend", 2, 1)
	w.FrameEnd("@@end", 3)
	want := strings.Join([]string{
		`printf '%s\n' "@@begin:1"`,
		`printf '%s\n' "@@state:1|$st"`,
		`printf '%s\n' "S:1|$(du -sb "$storage" 2>/dev/null | cut -f1)"`,
		`printf '%s\n' "@@hyg:2|0"`,
		`printf '%s\n' "@@hygend:2|1"`,
		`printf '%s\n' "@@end:3"`,
		"",
	}, "\n")
	if b.String() != want {
		t.Errorf("script lines:\n%s\nwant:\n%s", b.String(), want)
	}
}

// TestParseBatchStream pins the framed protocol end to end: per-target
// sections, captured windows, plain lines inside the frame, and the noise
// the parser must swallow (out-of-range indices, malformed markers, lines
// outside any frame).
func TestParseBatchStream(t *testing.T) {
	t.Parallel()
	out := strings.Join([]string{
		"@@begin:1",
		"@@state:1|running",
		"@@hyg:1|0",
		"orphan-1a",
		"orphan-1b",
		"@@hygend:1|0",
		"#container-zstd:1",
		"noise line",
		"@@end:1",
		// target 2: capture never closes, fanout tags still flow
		"@@begin:2",
		"@@state:2|exited",
		"@@hyg:2|0",
		"unclosed",
		"#container-zstd:0",
		"@@end:2",
		// malformed and out-of-range noise
		"@@begin:x",
		"@@begin:0",
		"@@begin:3|tail",
		"@@begin:99",
		"@@state:3|forged",
		"@@end:3",
		"@@state:99|forged",
		"",
	}, "\n")
	c := &capture{}
	p := Parser{
		Frame:    "@@begin",
		FrameEnd: "@@end",
		Targets:  2,
		Sections: Sections{"@@state": "", "@@hyg": "@@hygend"},
		Handlers: c.handlers(),
	}
	p.Parse(out)

	if len(c.frame) != 2 || c.frame[0] != 1 || c.frame[1] != 2 {
		t.Fatalf("frame dispatches: %v, want [1 2]", c.frame)
	}
	if len(c.frameEnd) != 2 || c.frameEnd[0] != 1 || c.frameEnd[1] != 2 {
		t.Fatalf("frameEnd dispatches: %v, want [1 2]", c.frameEnd)
	}
	if len(c.values) != 2 || c.values[0] != `1 @@state "running"` || c.values[1] != `2 @@state "exited"` {
		t.Fatalf("value dispatches: %v", c.values)
	}
	if len(c.captured) != 1 || c.captured[0] != `1 @@hyg 0 "orphan-1a\norphan-1b\n"` {
		t.Fatalf("captured dispatches: %v", c.captured)
	}
	// Target 2's fanout tag sits inside its armed capture window, so it is
	// buffered and never dispatched; the capture dies with the unclosed
	// window at the frame end. Target 1's plain lines dispatch in order.
	if len(c.lines) != 2 || c.lines[0] != `1 "#container-zstd:1"` || c.lines[1] != `1 "noise line"` {
		t.Fatalf("line dispatches: %v", c.lines)
	}
}

// TestParseCapturedEdges covers the capture contract: the close line's
// sub-index is reported, a negative open sub-index never arms capture, a
// close without an armed capture dispatches nothing, a malformed close line
// is consumed without ending the capture, and a captured window interrupted
// by the next frame delivers nothing.
func TestParseCapturedEdges(t *testing.T) {
	t.Parallel()
	t.Run("close reports its own sub-index and tail up to first pipe", func(t *testing.T) {
		t.Parallel()
		c := &capture{}
		p := Parser{
			Frame: "@@b", FrameEnd: "@@e", Targets: 1,
			Sections: Sections{"@@h": "@@he"},
			Handlers: c.handlers(),
		}
		p.Parse("@@b:1\n@@h:1|0\nline with |pipe\n@@he:1|7\n@@e:1")
		if len(c.captured) != 1 || c.captured[0] != `1 @@h 7 "line with |pipe\n"` {
			t.Fatalf("captured: %v", c.captured)
		}
	})
	t.Run("negative open sub-index leaves capture unarmed", func(t *testing.T) {
		t.Parallel()
		c := &capture{}
		p := Parser{
			Frame: "@@b", FrameEnd: "@@e", Targets: 1,
			Sections: Sections{"@@h": "@@he"},
			Handlers: c.handlers(),
		}
		p.Parse("@@b:1\n@@h:1\nnot captured\n@@he:1|0\n@@e:1")
		if len(c.captured) != 0 {
			t.Fatalf("captured: %v, want none", c.captured)
		}
		if len(c.lines) != 1 || c.lines[0] != `1 "not captured"` {
			t.Fatalf("lines while unarmed: %v", c.lines)
		}
	})
	t.Run("malformed close line neither closes nor buffers", func(t *testing.T) {
		t.Parallel()
		c := &capture{}
		p := Parser{
			Frame: "@@b", FrameEnd: "@@e", Targets: 1,
			Sections: Sections{"@@h": "@@he"},
			Handlers: c.handlers(),
		}
		p.Parse("@@b:1\n@@h:1|0\nkept\n@@he:x\nmore\n@@he:1|0\n@@e:1")
		if len(c.captured) != 1 || c.captured[0] != `1 @@h 0 "kept\nmore\n"` {
			t.Fatalf("captured: %v", c.captured)
		}
	})
	t.Run("frame move drops an unclosed capture", func(t *testing.T) {
		t.Parallel()
		c := &capture{}
		p := Parser{
			Frame: "@@b", FrameEnd: "@@e", Targets: 2,
			Sections: Sections{"@@h": "@@he"},
			Handlers: c.handlers(),
		}
		p.Parse("@@b:1\n@@h:1|0\nlost\n@@b:2\n@@e:2")
		if len(c.captured) != 0 {
			t.Fatalf("captured: %v, want none", c.captured)
		}
		if len(c.frameEnd) != 1 || c.frameEnd[0] != 2 {
			t.Fatalf("frameEnd: %v, want [2]", c.frameEnd)
		}
	})
}

// TestParseMissingFrameEnd pins the missing-section-is-failure contract: a
// block opened but never closed dispatches no FrameEnd, and its late
// sections are ignored once the parser has moved on.
func TestParseMissingFrameEnd(t *testing.T) {
	t.Parallel()
	c := &capture{}
	p := Parser{
		Frame: "@@m", Targets: 2,
		Sections: Sections{"@@st": ""},
		Handlers: c.handlers(),
	}
	p.Parse("@@m:1\n@@st:1|running\n@@m:2\n@@st:2|stopped")
	if len(c.frameEnd) != 0 {
		t.Errorf("frameEnd: %v, want none (protocol has no end marker)", c.frameEnd)
	}
	if len(c.values) != 2 {
		t.Errorf("values: %v, want both", c.values)
	}
}

func TestParseFrameless(t *testing.T) {
	t.Parallel()
	c := &capture{}
	p := Parser{
		Targets:  1,
		Sections: Sections{"S": "", "K": "", "D": ""},
		Handlers: c.handlers(),
	}
	p.Parse(strings.Join([]string{
		"S:1|running|image:latest",
		"K:1|1",
		"H:1|/root",
		"D:1|1024",
		"K:2|1",
		"S:running",
	}, "\n"))
	want := []string{`1 S "running|image:latest"`, `1 K "1"`, `1 D "1024"`}
	if len(c.values) != len(want) {
		t.Fatalf("values: %v, want %v", c.values, want)
	}
	for i, w := range want {
		if c.values[i] != w {
			t.Errorf("values[%d] = %q, want %q", i, c.values[i], w)
		}
	}
}

// TestParseFrameEndWithoutOpen makes an explicit end marker for a target
// that never opened a no-op (the Reported contract must not fire on a
// forged end line).
func TestParseFrameEndWithoutOpen(t *testing.T) {
	t.Parallel()
	c := &capture{}
	p := Parser{
		Frame: "@@b", FrameEnd: "@@e", Targets: 1,
		Handlers: c.handlers(),
	}
	p.Parse("@@e:1\n@@e:0\n@@e:x")
	if len(c.frameEnd) != 0 {
		t.Errorf("frameEnd: %v, want none", c.frameEnd)
	}
}

func TestParseTrimsCarriageReturns(t *testing.T) {
	t.Parallel()
	c := &capture{}
	p := Parser{
		Frame: "@@m", Targets: 1,
		Sections: Sections{"@@st": ""},
		Handlers: c.handlers(),
	}
	p.Parse("@@m:1\r\n@@st:1|running\r\n")
	if len(c.values) != 1 || c.values[0] != `1 @@st "running"` {
		t.Errorf("values: %v", c.values)
	}
}

// TestParseMarkerGrammar pins the tail rules: the first '|' separates the
// index from the tail (later pipes belong to the value), and frame markers
// with a tail are malformed rather than partially honoured.
func TestParseMarkerGrammar(t *testing.T) {
	t.Parallel()
	if n, tail, ok := markerTail("@@st:12|a|b", "@@st:"); !ok || n != 12 || tail != "a|b" {
		t.Errorf("markerTail pipes: n=%d tail=%q ok=%v", n, tail, ok)
	}
	if n, tail, ok := markerTail("@@st:12", "@@st:"); !ok || n != 12 || tail != "" {
		t.Errorf("markerTail no pipe: n=%d tail=%q ok=%v", n, tail, ok)
	}
	if _, _, ok := markerTail("@@st:x|a", "@@st:"); ok {
		t.Error("markerTail garbage index must fail")
	}
	if _, _, ok := markerTail("other:12|a", "@@st:"); ok {
		t.Error("markerTail missing prefix must fail")
	}
	if n := markerNum("@@begin:12|junk", "@@begin:"); n != -1 {
		t.Errorf("markerNum with tail = %d, want -1", n)
	}
	if n := markerNum("@@begin:12", "@@begin:"); n != 12 {
		t.Errorf("markerNum = %d, want 12", n)
	}
	if n := markerNum("@@begin: 12 ", "@@begin:"); n != 12 {
		t.Errorf("markerNum trimmed = %d, want 12", n)
	}
}
