package runner

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

// newHostWithOS returns a Host with only the OS field set; the other
// HostConfig fields are zero-valued, which is sufficient for dispatch tests.
func newHostWithOS(os string) *host.Host {
	return &host.Host{HostConfig: config.HostConfig{OS: os}}
}

// runOnHostOS: dispatch correctness
func TestRunOnHostOS_dispatchesWindows(t *testing.T) {
	t.Parallel()
	called := false
	got, err := runOnHostOS(newHostWithOS("windows"),
		func() (string, error) { called = true; return "win", nil },
		func() (string, error) { return "posix", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("windows callback was not invoked")
	}
	if got != "win" {
		t.Fatalf("got %q, want %q", got, "win")
	}
}

func TestRunOnHostOS_dispatchesLinux(t *testing.T) {
	t.Parallel()
	called := false
	got, err := runOnHostOS(newHostWithOS("linux"),
		func() (string, error) { return "win", nil },
		func() (string, error) { called = true; return "posix", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("posix callback was not invoked")
	}
	if got != "posix" {
		t.Fatalf("got %q, want %q", got, "posix")
	}
}

func TestRunOnHostOS_dispatchesDarwin(t *testing.T) {
	t.Parallel()
	called := false
	got, err := runOnHostOS(newHostWithOS("darwin"),
		func() (string, error) { return "win", nil },
		func() (string, error) { called = true; return "posix", nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("posix callback was not invoked for darwin")
	}
	if got != "posix" {
		t.Fatalf("got %q, want %q", got, "posix")
	}
}

// runOnHostOS: explicit-error footgun guard
func TestRunOnHostOS_errorsOnUnknownOS(t *testing.T) {
	t.Parallel()
	// These are the values that previously fell into the `default:` POSIX
	// branch silently. They must now surface an error so future h.OS
	// additions or typos don't get treated as POSIX.
	cases := []string{"freebsd", "openbsd", "illumos", "", "Windows", "LINUX", "solaris"}
	for _, os := range cases {
		os := os
		t.Run(os, func(t *testing.T) {
			t.Parallel()
			winCalled, posixCalled := false, false
			got, err := runOnHostOS(newHostWithOS(os),
				func() (string, error) { winCalled = true; return "win", nil },
				func() (string, error) { posixCalled = true; return "posix", nil },
			)
			if err == nil {
				t.Fatalf("expected error for unsupported OS %q, got nil", os)
			}
			if winCalled {
				t.Fatalf("windows callback invoked for %q", os)
			}
			if posixCalled {
				t.Fatalf("posix callback invoked for %q (the footgun we are fixing)", os)
			}
			if got != "" {
				t.Fatalf("expected zero value, got %q", got)
			}
			if !strings.Contains(err.Error(), "unsupported host OS") {
				t.Fatalf("error %q should mention 'unsupported host OS'", err)
			}
			if !strings.Contains(err.Error(), os) {
				t.Fatalf("error %q should include the OS value %q", err, os)
			}
		})
	}
}

// runOnHostOS: callback error propagation
func TestRunOnHostOS_propagatesCallbackError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	got, err := runOnHostOS(newHostWithOS("windows"),
		func() (string, error) { return "", sentinel },
		func() (string, error) { return "posix", nil },
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected %v, got %v", sentinel, err)
	}
	if got != "" {
		t.Fatalf("expected zero value on error, got %q", got)
	}
}

// runOnHostOS: slice return shape (proves generics carry the type through)
func TestRunOnHostOS_worksWithSliceReturn(t *testing.T) {
	t.Parallel()
	got, err := runOnHostOS(newHostWithOS("linux"),
		func() ([]string, error) { return []string{"w1"}, nil },
		func() ([]string, error) { return []string{"p1", "p2"}, nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "p1" || got[1] != "p2" {
		t.Fatalf("got %v, want [p1 p2]", got)
	}
}
