// Package probe owns the tagged-stream wire protocol shared by the
// one-SSH-round-trip probes (the cache server, the registry mirror, and the
// agentic inner-hygiene sweep).
//
// A remote shell script emits a single stream of tagged lines; this module
// renders those lines and decodes them back into per-target results. Emitting
// and parsing live in one module so the framing, the 1-based index alignment,
// CR handling, and the section contract cannot drift between adapters.
// Adapters declare their tag names and handle decoded lines; everything about
// the wire format is concentrated here.
//
// Two wire shapes exist, and both are rendered here:
//
//	echo "<tag>:<index>[|<tail>]"     — the indexed probes (registry, agentic)
//	printf '<tag><format>\n' "<arg>"  — the flat cache probe
//
// In both shapes the index and the payload are written verbatim, so an
// adapter can embed a shell expression ("$n", "$out", "$(du -sb ...)") that the
// remote shell expands.
package probe

import (
	"iter"
	"strconv"
	"strings"
)

// Writer emits the protocol's lines into a remote script under construction.
//
// The writer owns the wire format: every line it appends is exactly the text
// the remote shell runs. Writing the marker straight into the script — rather
// than building a marker string and formatting it in — keeps the emit path
// allocation-free, so a probe's script costs no more to build than the
// literal shell text it wraps.
type Writer struct {
	b *strings.Builder
}

// NewWriter returns a Writer that appends to b.
func NewWriter(b *strings.Builder) *Writer { return &Writer{b: b} }

// Echo appends: echo "<tag><index>[|<tail>]" followed by a newline.
//
// index is the 1-based target index as written into the script. It is written
// verbatim, so it may be a shell expression such as "$n" that the remote
// shell expands. tail is the payload after the "|" separator and is omitted
// when empty.
//
// The tail is written verbatim too, so it may be a shell expression ("$out",
// "$(du -sb ...)") that the remote shell expands inside the double-quoted
// argument.
func (w *Writer) Echo(tag, index, tail string) {
	w.EchoInline(tag, index, tail)
	w.b.WriteByte('\n')
}

// EchoInline appends: echo "<tag><index>[|<tail>]" with no trailing newline,
// for a marker that shares a shell line with other commands.
func (w *Writer) EchoInline(tag, index, tail string) {
	w.b.WriteString("echo \"")
	w.marker(tag, index, tail)
	w.b.WriteByte('"')
}

// Printf appends: printf '<tag><format>\n' "<arg>" followed by a newline.
//
// format is the shell printf format for the value (normally "%s", substituted
// by the remote shell from arg); arg is the shell word holding the value. An
// empty arg omits the argument entirely, for a marker whose value is a
// literal.
func (w *Writer) Printf(tag, format, arg string) {
	w.PrintfInline(tag, format, arg)
	w.b.WriteByte('\n')
}

// PrintfInline appends: printf '<tag><format>\n' "<arg>" with no trailing
// newline, for a marker that shares a shell line with other commands.
func (w *Writer) PrintfInline(tag, format, arg string) {
	w.b.WriteString("printf '")
	w.b.WriteString(tag)
	w.b.WriteString(format)
	w.b.WriteString("\\n'")
	if arg != "" {
		w.b.WriteString(" \"")
		w.b.WriteString(arg)
		w.b.WriteByte('"')
	}
}

// marker writes one marker: the tag, then the 1-based index when the protocol
// is indexed, then the payload separated by "|". A flat marker (no index)
// carries its payload directly, with no separator.
func (w *Writer) marker(tag, index, tail string) {
	w.b.WriteString(tag)
	if index == "" {
		w.b.WriteString(tail)
		return
	}
	w.b.WriteString(index)
	if tail != "" {
		w.b.WriteByte('|')
		w.b.WriteString(tail)
	}
}

// Decoded is one line of a tagged stream after decoding. Tag keeps the
// trailing colon so adapters can switch on the same prefixed strings the
// shell scripts emit.
//
// Two shapes are recognized:
//   - indexed marker "@@<tag>:<n>[|<tail>]" — Index is the parsed 1-based
//     index, Tail is the payload after "<n>|", HasTail records whether that
//     payload was present. Begin/end markers use HasTail to reject a trailing
//     tail (the strict form).
//   - flat tag "<tag>:<payload>" — Index is 0, HasTail is false; adapters
//     read the payload from Body.
//
// Body is always the raw text after the tag's colon. Raw is the CR-trimmed
// source line. OK is false for plain payload lines that carry no tag
// separator, so adapters can capture them verbatim between markers.
type Decoded struct {
	Tag     string
	Index   int
	Tail    string
	HasTail bool
	Body    string
	Raw     string
	OK      bool
}

// Decode parses one raw line. It recognizes the canonical
// "@@<tag>:<n>[|<tail>]" marker and the flat "<tag>:<tail>" form used by the
// cache probe. Plain payload lines (no colon) return OK=false with Raw set.
func Decode(raw string) (Decoded, bool) {
	line := strings.TrimRight(raw, "\r")
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return Decoded{Raw: line}, false
	}
	d := Decoded{
		Tag:  line[:i+1],
		Body: line[i+1:],
		Raw:  line,
		OK:   true,
	}
	// "<n>|<tail>" wins over a plain "<n>". When the head is not a number the
	// whole body is a flat tag's payload (e.g. "S:running|image:1").
	if head, tail, found := strings.Cut(d.Body, "|"); found {
		if n := AtoiOrNeg(head); n >= 0 {
			d.Index, d.Tail, d.HasTail = n, tail, true
		}
	} else if n := AtoiOrNeg(d.Body); n >= 0 {
		d.Index = n
	}
	return d, true
}

// Lines returns an iterator over the decoded lines of a tagged stream. Line
// splitting and CR handling live here so the three parsers cannot diverge on
// them.
//
// The iterator is a range-over-func sequence: an adapter that only reads the
// decoded values keeps its parse state on the stack, so parsing stays
// allocation-free.
func Lines(out string) iter.Seq[Decoded] {
	return func(yield func(Decoded) bool) {
		for raw := range strings.SplitSeq(out, "\n") {
			d, _ := Decode(raw)
			if !yield(d) {
				return
			}
		}
	}
}

// AtoiOrNeg parses a 1-based marker index, returning -1 on failure. -1 is
// the invalid sentinel because every index it parses is 1-based. Adapters use
// it for sub-indices carried inside a marker tail (e.g. the hygiene check
// number in "@@hyg:<n>|<k>").
//
// The literal is validated before strconv sees it: strconv.Atoi allocates a
// *strconv.NumError for every non-numeric input, and a flat tag's payload is
// usually not a number, so validating first keeps the parse path
// allocation-free.
func AtoiOrNeg(s string) int {
	s = strings.TrimSpace(s)
	if !isIntegerLiteral(s) {
		return -1
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

// isIntegerLiteral reports whether s is an optionally signed run of decimal
// digits — the shape strconv.Atoi accepts without allocating.
func isIntegerLiteral(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' || s[0] == '+' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
