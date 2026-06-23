package agentic

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestFailureCollector_EmptyReturnsNil(t *testing.T) {
	t.Parallel()
	var c failureCollector
	got := c.wait()
	if got != nil {
		t.Errorf("empty collector must return nil, got %#v", got)
	}
}

func TestFailureCollector_AppendAndWait(t *testing.T) {
	t.Parallel()
	var c failureCollector
	c.append(PrereqFailure{Name: "a", Severity: SeverityError, Message: "m1"})
	c.append(PrereqFailure{Name: "b", Severity: SeverityWarning, Message: "m2"})

	got := c.wait()
	if len(got) != 2 {
		t.Fatalf("expected 2 failures, got %d (%#v)", len(got), got)
	}
	if got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("submission order not preserved: got %#v", got)
	}
}

func TestFailureCollector_SpawnRunsClosure(t *testing.T) {
	t.Parallel()
	var c failureCollector
	var ran atomic.Bool
	c.spawn(func() { ran.Store(true) })
	if !c.wait_run(&ran) {
		t.Errorf("spawned closure did not run before wait returned")
	}
}

// wait_run is a small helper used by TestFailureCollector_SpawnRunsClosure so the
// test does not need to reach into c.failures to verify the closure ran.
func (c *failureCollector) wait_run(ran *atomic.Bool) bool {
	c.wg.Wait()
	return ran.Load()
}

func TestFailureCollector_ConcurrentAppendPreservesSubmissionOrder(t *testing.T) {
	t.Parallel()
	var c failureCollector
	const N = 200
	var wgStart sync.WaitGroup
	wgStart.Add(1)
	for i := 0; i < N; i++ {
		i := i
		c.spawn(func() {
			// Make every goroutine block on wgStart so they all enter
			// the critical section at the same time and exercise the
			// mutex, not just goroutine launch ordering.
			wgStart.Wait()
			c.append(PrereqFailure{Name: fmt.Sprintf("n%03d", i)})
		})
	}
	wgStart.Done() // release them all at once

	got := c.wait()
	if len(got) != N {
		t.Fatalf("expected %d failures, got %d", N, len(got))
	}
	// Every index must appear exactly once. A regression that drops the
	// mutex (or moves append outside it) would manifest as duplicate or
	// missing names.
	seen := make(map[string]int, N)
	for _, f := range got {
		seen[f.Name]++
	}
	if len(seen) != N {
		t.Errorf("expected %d distinct names, got %d (counts: %v)", N, len(seen), seen)
	}
	for i := 0; i < N; i++ {
		name := fmt.Sprintf("n%03d", i)
		if seen[name] != 1 {
			t.Errorf("name %q seen %d times, want 1", name, seen[name])
		}
	}
}

func TestFailureCollector_WaitBlocksUntilAllGoroutinesFinish(t *testing.T) {
	t.Parallel()
	var c failureCollector
	var counter atomic.Int64
	const N = 50
	for i := 0; i < N; i++ {
		c.spawn(func() {
			counter.Add(1)
		})
	}
	c.wait()
	// If wait() did not actually block, counter would still be < N here
	// in any schedule where some goroutines have not been scheduled yet.
	// On a typical CI box this is a strong assertion; combined with the
	// race detector it also catches use-after-wait on the slice header.
	if got := counter.Load(); got != N {
		t.Errorf("counter = %d, want %d (wait() may not have blocked)", got, N)
	}
}
