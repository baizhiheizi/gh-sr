package testutil

import (
	"errors"
	"testing"
)

func TestMockExecutor_RunFn(t *testing.T) {
	t.Parallel()
	m := &MockExecutor{
		RunFn: func(cmd string) (string, error) {
			return "out:" + cmd, nil
		},
	}
	got, err := m.Run("echo hi")
	if err != nil {
		t.Fatal(err)
	}
	if got != "out:echo hi" {
		t.Fatalf("got %q", got)
	}
	if len(m.Calls) != 1 || m.Calls[0] != "echo hi" {
		t.Fatalf("calls=%v", m.Calls)
	}
}

func TestMockExecutor_ResponsesSequence(t *testing.T) {
	t.Parallel()
	m := &MockExecutor{Responses: []string{"a", "b"}}
	if out, err := m.Run("1"); err != nil || out != "a" {
		t.Fatalf("first: out=%q err=%v", out, err)
	}
	if out, err := m.Run("2"); err != nil || out != "b" {
		t.Fatalf("second: out=%q err=%v", out, err)
	}
	if out, err := m.Run("3"); err != nil || out != "" {
		t.Fatalf("third: out=%q err=%v", out, err)
	}
}

func TestMockExecutor_RunErr(t *testing.T) {
	t.Parallel()
	errSentinel := errors.New("fail")
	m := &MockExecutor{RunErr: errSentinel}
	_, err := m.Run("cmd")
	if !errors.Is(err, errSentinel) {
		t.Fatalf("got %v", err)
	}
}

func TestMockExecutor_Upload(t *testing.T) {
	t.Parallel()
	m := &MockExecutor{}
	if err := m.Upload("/local", "/remote"); err != nil {
		t.Fatal(err)
	}
	if !m.UploadCalled || m.LastUpload.Local != "/local" || m.LastUpload.Remote != "/remote" {
		t.Fatalf("upload state: %+v", m)
	}
}
