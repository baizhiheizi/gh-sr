package testutil

import (
	"errors"
	"testing"
)

func TestRoundTripCounter_CountsRuns(t *testing.T) {
	t.Parallel()
	m := &RoundTripCounter{}
	if _, err := m.Run("echo hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Run("echo there"); err != nil {
		t.Fatal(err)
	}
	if m.RoundTrips != 2 {
		t.Fatalf("RoundTrips = %d, want 2", m.RoundTrips)
	}
	m.Reset()
	if m.RoundTrips != 0 {
		t.Fatalf("RoundTrips after Reset = %d, want 0", m.RoundTrips)
	}
}

func TestRoundTripCounter_RunFn(t *testing.T) {
	t.Parallel()
	m := &RoundTripCounter{RunFn: func(cmd string) (string, error) {
		return "out:" + cmd, nil
	}}
	got, err := m.Run("echo hi")
	if err != nil {
		t.Fatal(err)
	}
	if got != "out:echo hi" {
		t.Fatalf("got %q", got)
	}
	if m.RoundTrips != 1 {
		t.Fatalf("RoundTrips = %d, want 1", m.RoundTrips)
	}
}

func TestRoundTripCounter_RunFnError(t *testing.T) {
	t.Parallel()
	errSentinel := errors.New("fail")
	m := &RoundTripCounter{RunFn: func(string) (string, error) {
		return "", errSentinel
	}}
	if _, err := m.Run("cmd"); !errors.Is(err, errSentinel) {
		t.Fatalf("got %v", err)
	}
	// A failed call still cost a round trip.
	if m.RoundTrips != 1 {
		t.Fatalf("RoundTrips = %d, want 1", m.RoundTrips)
	}
}

func TestRoundTripCounter_UploadAndCloseAreNoOps(t *testing.T) {
	t.Parallel()
	m := &RoundTripCounter{}
	if err := m.Upload("/local", "/remote"); err != nil {
		t.Fatal(err)
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if m.RoundTrips != 0 {
		t.Fatalf("RoundTrips = %d, want 0", m.RoundTrips)
	}
}
