package autostart

import "testing"

func TestSanitizeInstance(t *testing.T) {
	t.Parallel()
	got, err := SanitizeInstance("ci-1")
	if err != nil || got != "ci-1" {
		t.Fatalf("ci-1: got %q err %v", got, err)
	}
	got, err = SanitizeInstance("my_runner-2")
	if err != nil || got != "my-runner-2" {
		t.Fatalf("my_runner-2: got %q err %v", got, err)
	}
	_, err = SanitizeInstance("---")
	if err == nil {
		t.Fatal("expected error for empty after trim")
	}
}
