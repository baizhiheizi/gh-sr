// Package fanout owns gh-sr's concurrent fan-out primitives: the indexed
// result collection and the serialised writer that the per-host and
// per-scope orchestrators share.
//
// The skeleton (one goroutine per item, a WaitGroup, results collected in
// item order) used to be hand-rolled in every orchestrator that needed one.
// Lock placement, ordering semantics and the error/progress interleaving then
// had to be kept in step across copies, and every concurrency fix had to be
// re-derived per copy. One implementation here keeps them uniform, and it
// lives in a leaf package so packages that must not depend on internal/ops
// (internal/doctor, internal/runner) can use it too.
package fanout

import (
	"io"
	"sync"
)

// Indexed runs fn concurrently for every item and returns the results in item
// order: results[i] holds fn's return value for items[i].
//
// fn runs in its own goroutine, so it must not share mutable state with the
// other items unless it synchronises that state itself (see LockedWriter).
// Writing to results[i] needs no synchronisation: every goroutine owns a
// distinct index.
//
// A panic inside fn is not recovered: it propagates and terminates the
// program, exactly as it did in the hand-rolled copies this replaced.
func Indexed[T, R any](items []T, fn func(i int, item T) R) []R {
	results := make([]R, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = fn(i, item)
		}()
	}
	wg.Wait()
	return results
}

// LockedWriter serialises concurrent writes to an underlying io.Writer, so
// interleaved output from parallel workers never tears.
type LockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// NewLockedWriter wraps w so concurrent writes to it are serialised.
func NewLockedWriter(w io.Writer) *LockedWriter {
	return &LockedWriter{w: w}
}

// Write implements io.Writer.
func (lw *LockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}
