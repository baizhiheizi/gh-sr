package testutil

// RoundTripCounter is a host.Executor that counts the remote command
// invocations an operation costs — the "SSH round trips" metric.
//
// Unlike MockExecutor it does not retain the command strings: a benchmark
// that runs the same operation b.N times would otherwise grow a slice of
// b.N commands and pay that allocation churn inside the measured window,
// inflating the allocs/op metric it is trying to report alongside the
// round-trip count.
//
// The round-trip count is the network/energy proxy for gh-sr: every remote
// invocation costs a network round trip, a remote shell start, and the
// remote CPU to run the probe. Cutting one round trip out of an operation
// removes all three, which no CPU-side microbenchmark can show.
type RoundTripCounter struct {
	// RunFn produces the command output, mirroring MockExecutor.RunFn.
	// When nil, every command returns an empty output and no error.
	RunFn func(cmd string) (string, error)

	// RoundTrips counts the Run calls since the last Reset.
	RoundTrips int
}

// Run records one round trip and returns the canned output.
func (c *RoundTripCounter) Run(cmd string) (string, error) {
	c.RoundTrips++
	if c.RunFn != nil {
		return c.RunFn(cmd)
	}
	return "", nil
}

// Upload is a no-op: the counter models command execution only.
func (c *RoundTripCounter) Upload(string, string) error { return nil }

// Close is a no-op: the counter owns no connection.
func (c *RoundTripCounter) Close() error { return nil }

// Reset zeroes the round-trip counter so the next operation can be counted
// on its own.
func (c *RoundTripCounter) Reset() { c.RoundTrips = 0 }
