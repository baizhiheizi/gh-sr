package fanout

import (
	"bytes"
	"sync"
	"testing"
)

func TestIndexed_PreservesItemOrder(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c", "d"}
	// Reverse the work order so a result collected out of order would show up.
	results := Indexed(items, func(i int, item string) string {
		return item
	})

	if len(results) != len(items) {
		t.Fatalf("got %d results, want %d", len(results), len(items))
	}
	for i, want := range items {
		if results[i] != want {
			t.Errorf("results[%d] = %q, want %q", i, results[i], want)
		}
	}
}

func TestIndexed_PassesIndexAndItem(t *testing.T) {
	t.Parallel()

	items := []int{10, 20, 30}
	results := Indexed(items, func(i int, item int) int { return i + item })

	for i, want := range []int{10, 21, 32} {
		if results[i] != want {
			t.Errorf("results[%d] = %d, want %d", i, results[i], want)
		}
	}
}

func TestIndexed_Empty(t *testing.T) {
	t.Parallel()

	results := Indexed([]string{}, func(int, string) string {
		t.Fatal("fn must not run for an empty item list")
		return ""
	})
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
}

func TestIndexed_RunsConcurrently(t *testing.T) {
	t.Parallel()

	const workers = 8
	items := make([]int, workers)
	for i := range items {
		items[i] = i
	}

	// Every worker waits for every other worker to start: the fan-out only
	// completes if the workers really run in parallel.
	var wg sync.WaitGroup
	wg.Add(workers)
	started := make(chan struct{}, workers)
	results := Indexed(items, func(i int, item int) int {
		started <- struct{}{}
		wg.Done()
		wg.Wait()
		return item
	})

	for i, want := range items {
		if results[i] != want {
			t.Errorf("results[%d] = %d, want %d", i, results[i], want)
		}
	}
	if len(started) != workers {
		t.Errorf("%d workers started, want %d", len(started), workers)
	}
}

func TestLockedWriter_Sequential(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := NewLockedWriter(&buf)
	n, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("wrote %d bytes, want 5", n)
	}
	if buf.String() != "hello" {
		t.Errorf("buf = %q, want %q", buf.String(), "hello")
	}
}

func TestLockedWriter_Concurrent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := NewLockedWriter(&buf)
	const workers = 10
	const writesPerWorker = 100
	expected := workers * writesPerWorker

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerWorker; j++ {
				lw.Write([]byte("x"))
			}
		}()
	}
	wg.Wait()

	if buf.Len() != expected {
		t.Errorf("buf.Len() = %d, want %d", buf.Len(), expected)
	}
}
