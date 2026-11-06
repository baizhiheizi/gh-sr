// Package probe owns the tagged-stream wire protocol shared by the
// one-SSH-round-trip probes (cache Inspect, registry Inspect, the agentic
// inner-hygiene batch): a remote script echoes marker lines that key captured
// values to 1-based target indices, and the parser maps them back per target.
//
// The grammar has exactly one line form:
//
//	<marker>:<n>[|<tail>]
//
// <marker> is a section's sigil ("@@st", "S"), <n> a 1-based target index,
// and the optional tail runs from the first '|' to end of line. Frame
// markers open (and optionally close) a target's block and carry no tail —
// a tail on a frame line marks the line malformed, and it is ignored.
// Sections are single-line (the tail is the value) or captured (the lines
// between the open marker and its close marker accumulate,
// newline-terminated; capture arms only when the open line carries a
// non-negative sub-index).
//
// Every marker-prefixed line is consumed by the parser, but only
// well-formed lines whose index matches the open frame dispatch to a
// handler; everything a handler sees is in [1, Targets]. A block opened but
// never closed dispatches no end — the missing-section-is-failure contract
// lives there. Owning emit and parse together keeps the two halves of the
// grammar from drifting: the protocol is tested once, here, and the probe
// packages stay adapters supplying probe bodies and result types.
package probe

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Writer emits the protocol's marker lines into a POSIX script under
// construction. Scripts are generated per call, so every target's block
// embeds its own literal 1-based index. value and k arguments are raw shell
// words (variable references, command substitutions, literals) that the
// remote shell expands inside the double-quoted printf argument.
type Writer struct {
	b *strings.Builder
}

// NewWriter returns a Writer appending to b.
func NewWriter(b *strings.Builder) *Writer { return &Writer{b: b} }

// Frame opens target n's block.
func (w *Writer) Frame(marker string, n int) { w.line(marker, n, "") }

// FrameEnd closes target n's block.
func (w *Writer) FrameEnd(marker string, n int) { w.line(marker, n, "") }

// Value emits a single-line section: the parser hands tail to target n.
func (w *Writer) Value(marker string, n int, value string) {
	w.line(marker, n, "|"+value)
}

// CaptureOpen arms a captured section for target n with sub-index k.
func (w *Writer) CaptureOpen(marker string, n, k int) {
	w.line(marker, n, "|"+strconv.Itoa(k))
}

// CaptureClose ends a captured section for target n with sub-index k.
func (w *Writer) CaptureClose(marker string, n, k int) {
	w.line(marker, n, "|"+strconv.Itoa(k))
}

// line writes `printf '%s\n' "<marker>:<n><tail>"`. printf (not echo) keeps
// the emitted line an argument to a fixed format, so backslashes or leading
// dashes in a remote variable can never reshape it.
func (w *Writer) line(marker string, n int, tail string) {
	fmt.Fprintf(w.b, "printf '%%s\\n' \"%s:%d%s\"\n", marker, n, tail)
}

// Sections maps a section's open marker to its shape: "" for a single-line
// section, or the close marker that ends a captured one.
type Sections map[string]string

// Handlers receive the parsed stream. Every n a handler sees was validated:
// it matches the open frame (frameless protocols target only 1) and lies in
// [1, Targets]. All fields are optional; nil handlers drop their dispatches.
type Handlers struct {
	// Frame runs when the frame marker opens target n.
	Frame func(n int)
	// FrameEnd runs when the end marker closes target n's block — its
	// sections are complete. A block opened but never closed dispatches
	// nothing here; that silence is the missing-section-is-failure signal.
	FrameEnd func(n int)
	// Value receives a single-line section's tail (after "<marker>:<n>|").
	Value func(n int, marker, tail string)
	// Captured receives a captured section at its close marker: k is the
	// close line's sub-index (-1 when absent), text the newline-terminated
	// concatenation of the lines captured since the open marker.
	Captured func(n int, marker string, k int, text string)
	// Line receives every other line inside an open block while no capture
	// is armed — probe-specific sub-grammars (e.g. fanout tags).
	Line func(n int, line string)
}

// Parser decodes one protocol variant of the tagged stream. Configure the
// fields and call Parse; a Parser is stateless between calls.
type Parser struct {
	// Frame is the frame-open marker; "" makes the whole stream target 1.
	Frame string
	// FrameEnd is the explicit frame-close marker ("" = blocks close only
	// when the next frame opens).
	FrameEnd string
	// Targets is the number of probed targets; indices outside
	// [1, Targets] are ignored.
	Targets int
	// Sections is the section table.
	Sections Sections
	// Handlers is the dispatch target.
	Handlers Handlers
}

// Parse walks out line by line and dispatches. Matching priority follows
// the protocol's precedence: frame markers, then — while a capture is
// armed — its close and open markers, then section markers (longest prefix
// first, so a marker that extends another cannot shadow it), then the
// capture buffer, then the block's plain lines. Any marker-prefixed line is
// consumed even when malformed or stale, so probe output can never leak
// into a capture buffer.
func (p *Parser) Parse(out string) {
	h := p.Handlers
	// cur is -1 while no block is open (or 1 from the start for a frameless
	// protocol), so a forged index-0 marker line can never match it.
	cur := -1
	if p.Frame == "" {
		cur = 1
	}
	opens := make([]string, 0, len(p.Sections))
	closes := make([]string, 0, len(p.Sections))
	for m, close := range p.Sections {
		opens = append(opens, m)
		if close != "" {
			closes = append(closes, close)
		}
	}
	sort.Slice(opens, func(i, j int) bool { return len(opens[i]) > len(opens[j]) })
	wire := make(map[string]string, len(opens))
	for _, m := range opens {
		wire[m] = m + ":"
	}
	closeWire := make([]string, len(closes))
	for i, cm := range closes {
		closeWire[i] = cm + ":"
	}

	var capOpen, capClose string
	var capBuf strings.Builder

	frameWire := p.Frame + ":"
	frameEndWire := p.FrameEnd + ":"

	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if p.Frame != "" && strings.HasPrefix(line, frameWire) {
			// Strict: a tail on a frame line is malformed and never moves
			// the frame or drops a capture.
			if n := markerNum(line, frameWire); n >= 1 && n <= p.Targets {
				cur = n
				capOpen, capClose = "", ""
				capBuf.Reset()
				if h.Frame != nil {
					h.Frame(n)
				}
			}
			continue
		}
		if p.FrameEnd != "" && strings.HasPrefix(line, frameEndWire) {
			if n := markerNum(line, frameEndWire); n == cur && cur >= 1 {
				if h.FrameEnd != nil {
					h.FrameEnd(n)
				}
				cur = -1
				capOpen, capClose = "", ""
			}
			continue
		}
		if capOpen != "" {
			if strings.HasPrefix(line, capClose+":") {
				m := capOpen
				if n, tail, ok := markerTail(line, capClose+":"); ok && n == cur {
					text := capBuf.String()
					k := atoiOrNeg(tail)
					capOpen, capClose = "", ""
					capBuf.Reset()
					if h.Captured != nil {
						h.Captured(n, m, k, text)
					}
				}
				continue
			}
			if strings.HasPrefix(line, capOpen+":") {
				// Restart: a fresh open re-arms (or disarms) capture with a
				// clean buffer, matching the open line's own sub-index rule.
				if n, tail, ok := markerTail(line, capOpen+":"); ok && n == cur {
					capBuf.Reset()
					if atoiOrNeg(tail) < 0 {
						capOpen, capClose = "", ""
					}
				}
				continue
			}
		}
		consumed := false
		for _, cw := range closeWire {
			// An unarmed capture's close markers are still protocol markers:
			// consumed, never dispatched, never buffered.
			if strings.HasPrefix(line, cw) {
				consumed = true
				break
			}
		}
		if consumed {
			continue
		}
		for _, m := range opens {
			if !strings.HasPrefix(line, wire[m]) {
				continue
			}
			consumed = true
			if n, tail, ok := markerTail(line, wire[m]); ok && n == cur {
				if close := p.Sections[m]; close == "" {
					if h.Value != nil {
						h.Value(n, m, tail)
					}
				} else {
					capBuf.Reset()
					if k := atoiOrNeg(tail); k >= 0 {
						capOpen, capClose = m, close
					} else {
						capOpen, capClose = "", ""
					}
				}
			}
			break
		}
		if consumed {
			continue
		}
		if capOpen != "" {
			capBuf.WriteString(line)
			capBuf.WriteString("\n")
			continue
		}
		if cur > 0 && h.Line != nil {
			h.Line(cur, line)
		}
	}
}

// markerTail splits "<prefix><n>|<tail>" into n and tail. ok is false when
// line lacks the prefix or n is not a valid index; the tail is absent
// (empty) when the line carries no '|'. Only the first '|' separates n from
// the tail — any later '|' stays inside tail.
func markerTail(line, prefix string) (n int, tail string, ok bool) {
	rest := strings.TrimPrefix(line, prefix)
	if rest == line {
		return -1, "", false
	}
	num := rest
	tail = ""
	if i := strings.IndexByte(rest, '|'); i >= 0 {
		num, tail = rest[:i], rest[i+1:]
	}
	n = atoiOrNeg(num)
	if n < 0 {
		return -1, "", false
	}
	return n, tail, true
}

// markerNum parses "<prefix><n>" strictly: a trailing "|<tail>" makes the
// line malformed (returns -1), so frame markers are never partially
// honoured.
func markerNum(line, prefix string) int {
	rest := strings.TrimPrefix(line, prefix)
	if rest == line {
		return -1
	}
	return atoiOrNeg(rest)
}

// atoiOrNeg parses s (trimmed) as an int, returning -1 on failure — the
// invalid-marker sentinel, since every protocol index is 1-based.
func atoiOrNeg(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return n
}
