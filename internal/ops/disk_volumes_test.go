package ops

import (
	"errors"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func newDiskVolumesHost(t *testing.T, mock *testutil.MockExecutor) *host.Host {
	t.Helper()
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	return h
}

// TestMeasureSharedVolumes pins the volume-rows contract: every gh-sr-*
// volume becomes one row with its measured size, the sizes come from a single
// throwaway busybox run mounting all volumes at once, and the listing command
// filters to gh-sr-* volumes host-side (the grep runs in the remote shell).
func TestMeasureSharedVolumes(t *testing.T) {
	t.Parallel()
	var listCmd, runCmd string
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "docker volume ls"):
			listCmd = cmd
			// Post-grep output (the filter runs host-side).
			return "gh-sr-gradle-cache\ngh-sr-npm-cache\n", nil
		default:
			runCmd = cmd
			return "4321\t0\n87654321\t1\n", nil
		}
	}}
	entries := measureSharedVolumes(newDiskVolumesHost(t, mock), "h")
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(entries), entries)
	}
	if entries[0].Instance != "gh-sr-gradle-cache" || entries[0].TotalBytes != 4321 || entries[0].Mode != "volume" {
		t.Errorf("entry 0: %+v", entries[0])
	}
	if entries[1].Instance != "gh-sr-npm-cache" || entries[1].TotalBytes != 87654321 {
		t.Errorf("entry 1: %+v", entries[1])
	}
	if entries[0].Err != nil || entries[1].Err != nil {
		t.Errorf("measured rows must carry no error: %+v %+v", entries[0], entries[1])
	}
	if !strings.Contains(listCmd, "grep '^gh-sr-'") {
		t.Errorf("listing must filter to gh-sr-* volumes, got %q", listCmd)
	}
	// Both volumes must be mounted into ONE container, in listed order.
	for _, want := range []string{
		"-v 'gh-sr-gradle-cache:/v/0'",
		"-v 'gh-sr-npm-cache:/v/1'",
		"busybox du -sb /v/*",
	} {
		if !strings.Contains(runCmd, want) {
			t.Errorf("measure command missing %q:\n%s", want, runCmd)
		}
	}
}

// TestMeasureSharedVolumesNoVolumes pins the quiet paths: a Docker failure or
// an empty volume list yields no rows instead of errors (native-only hosts,
// hosts without the shared caches).
func TestMeasureSharedVolumesNoVolumes(t *testing.T) {
	t.Parallel()
	t.Run("docker fails", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(string) (string, error) {
			return "", errDiskVolumesTest
		}}
		if got := measureSharedVolumes(newDiskVolumesHost(t, mock), "h"); got != nil {
			t.Fatalf("got %+v, want nil", got)
		}
	})
	t.Run("no gh-sr volumes", func(t *testing.T) {
		t.Parallel()
		mock := &testutil.MockExecutor{RunFn: func(string) (string, error) {
			// Post-grep output: nothing matches the gh-sr-* filter.
			return "", nil
		}}
		if got := measureSharedVolumes(newDiskVolumesHost(t, mock), "h"); got != nil {
			t.Fatalf("got %+v, want nil", got)
		}
	})
}

// TestMeasureSharedVolumesDuFailure pins the degraded-report contract: when
// the busybox measurement fails, rows still appear with the error attached
// rather than failing the whole disk-usage run.
func TestMeasureSharedVolumesDuFailure(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		if strings.Contains(cmd, "docker volume ls") {
			return "gh-sr-pub-cache\n", nil
		}
		return "", errDiskVolumesTest
	}}
	entries := measureSharedVolumes(newDiskVolumesHost(t, mock), "h")
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1: %+v", len(entries), entries)
	}
	if entries[0].Err == nil {
		t.Errorf("row must carry the measurement error, got %+v", entries[0])
	}
	if entries[0].TotalBytes != 0 {
		t.Errorf("failed measurement must report 0 bytes, got %d", entries[0].TotalBytes)
	}
}

var errDiskVolumesTest = errors.New("disk volumes test error")
