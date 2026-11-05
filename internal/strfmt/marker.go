package strfmt

import (
	"strconv"
	"strings"
)

// AtoiOrNeg parses s (after trimming surrounding whitespace) as an int,
// returning -1 on failure. -1 doubles as the callers' invalid-marker sentinel
// because every marker index it parses is 1-based.
//
// Used by:
//   - internal/agentic (markerNum)
//   - internal/registry (parseInspectBatchOutput @@m:)
func AtoiOrNeg(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return -1
	}
	return n
}

// SplitMarker parses "<prefix><n>|<tail>" into n and tail. ok is false when
// line lacks prefix or n is not a valid AtoiOrNeg number; the tail is absent
// (empty) when the line carries no '|'. Only the first '|' separates n from
// the tail — any later '|' stays inside tail.
//
// Used by:
//   - internal/agentic (inner batch @@state:/@@hyg:/@@hygend: markers)
//   - internal/registry (mirror inspect @@st:/@@health:/@@du: markers)
func SplitMarker(line, prefix string) (int, string, bool) {
	rest := strings.TrimPrefix(line, prefix)
	if rest == line {
		return -1, "", false
	}
	num := rest
	tail := ""
	if i := strings.IndexByte(rest, '|'); i >= 0 {
		num, tail = rest[:i], rest[i+1:]
	}
	n := AtoiOrNeg(num)
	if n < 0 {
		return -1, "", false
	}
	return n, tail, true
}
