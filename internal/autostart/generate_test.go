package autostart

import (
	"strings"
	"testing"
)

func TestSystemdUserUnit(t *testing.T) {
	t.Parallel()
	u := SystemdUserUnit("ci-1", "/home/u/.gh-sr/runners/ci-1")
	if !strings.Contains(u, "WorkingDirectory=/home/u/.gh-sr/runners/ci-1") {
		t.Fatal("missing WorkingDirectory")
	}
	if !strings.Contains(u, "ExecStart=/home/u/.gh-sr/runners/ci-1/run.sh") {
		t.Fatal("missing ExecStart")
	}
	if !strings.Contains(u, "Restart=always") {
		t.Fatal("missing Restart")
	}
	if !strings.Contains(u, "StartLimitIntervalSec=60") {
		t.Fatal("missing StartLimitIntervalSec")
	}
	if !strings.Contains(u, "StartLimitBurst=5") {
		t.Fatal("missing StartLimitBurst")
	}
	if !strings.Contains(u, "RestartPreventExitStatus=203") {
		t.Fatal("missing RestartPreventExitStatus")
	}
}

func TestSystemdSystemUnit(t *testing.T) {
	t.Parallel()
	u := SystemdSystemUnit("ci-1", "/home/u/.gh-sr/runners/ci-1", "u", "u")
	if !strings.Contains(u, "User=u") || !strings.Contains(u, "Group=u") {
		t.Fatal("missing User/Group")
	}
	if !strings.Contains(u, "RestartPreventExitStatus=203") {
		t.Fatal("missing RestartPreventExitStatus")
	}
}

func TestLaunchdPlist(t *testing.T) {
	t.Parallel()
	p := LaunchdPlist("ci-1", "/Users/u/.gh-sr/runners/ci-1")
	if !strings.Contains(p, "com.github.ghsr.runner.ci-1") {
		t.Fatal("missing label")
	}
	if !strings.Contains(p, "/Users/u/.gh-sr/runners/ci-1/run.sh") {
		t.Fatal("missing run.sh path")
	}
	if !strings.Contains(p, "<key>KeepAlive</key>") {
		t.Fatal("missing KeepAlive")
	}
}

func TestWindowsTaskName(t *testing.T) {
	t.Parallel()
	if WindowsTaskName("ci-1") != "ghsr-runner-ci-1" {
		t.Fatal(WindowsTaskName("ci-1"))
	}
}
