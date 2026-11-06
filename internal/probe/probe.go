// Package probe owns the tagged-stream wire protocol shared by the one-SSH-
// round-trip status probes (the cache server, the registry mirror, and the
// agentic inner-hygiene sweep).
//
// A remote shell script emits a single stream of tagged lines; this module
// renders those lines and decodes them back into per-target results. Emitting
// and parsing live in one module so the framing, the 1-based index alignment,
// CR handling, and the section contract cannot drift between adapters.
// Adapters declare their tag names and handle decoded lines; everything about
// the wire format is concentrated here.
package probe

import (
	"strconv"
	"strings"
)

// Line renders one tagged-stream line.
//
//   - tag includes its trailing colon, e.g. "@@state:" or "S:".
//   - index is the 1-based section index as written into the remote script;
//     pass a Go literal ("1"), a shell expression ("$n"), or "" for a flat
//     (global) tag with no index.
//   - tail is the optional payload. When index is non-empty the payload is
//     appended after "<index>|"; for a flat tag it is appended directly.
//
// Both index and tail are strings so emitters can embed shell expressions
// verbatim instead of formatting values into the script in Go.
func Line(tag, index, tail string) string {
	if index == "" {
		return tag + tail
	}
	if tail == "" {
		return tag + index
	}
	return tag + index + "|" + tail
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

// EachLine iterates the CR-trimmed lines of a tagged stream, handing each
// decoded line to fn. Line splitting and CR handling live here so the three
// parsers cannot diverge on them.
func EachLine(out string, fn func(Decoded)) {
	for raw := range strings.SplitSeq(out, "\n") {
		d, _ := Decode(raw)
		fn(d)
	}
}

// AtoiOrNeg parses a 1-based marker index, returning -1 on failure. -1 is
// the invalid sentinel because every index it parses is 1-based. Adapters use
// it for sub-indices carried inside a marker tail (e.g. the hygiene check
// number in "@@hyg:<n>|<k>").
func AtoiOrNeg(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return n
}
