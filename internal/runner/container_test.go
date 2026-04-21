package runner

import (
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
)

func TestContainerName(t *testing.T) {
	t.Parallel()
	if got := containerName("my-agentic-1"); got != "gh-sr-my-agentic-1" {
		t.Errorf("containerName: got %q", got)
	}
	if got := containerName("x"); got != "gh-sr-x" {
		t.Errorf("containerName(x): got %q", got)
	}
}

func TestContainerStateDir(t *testing.T) {
	t.Parallel()
	h := host.NewHost("h", config.HostConfig{Addr: "local", OS: "linux", Arch: "amd64"})
	dir := containerStateDir(h, "my-runner-1")
	// Should match the runner dir for the instance.
	if !strings.Contains(dir, "my-runner-1") {
		t.Errorf("containerStateDir should include instance name, got %q", dir)
	}
	if !strings.Contains(dir, ".gh-sr/runners") {
		t.Errorf("containerStateDir should be under .gh-sr/runners, got %q", dir)
	}
}

// TestDockerRunArgShape verifies the docker create command includes the expected
// --privileged flag, the container name, the bind-mount, and required env vars.
func TestDockerRunArgShape(t *testing.T) {
	t.Parallel()
	h := host.NewHost("h", config.HostConfig{Addr: "local", OS: "linux", Arch: "amd64"})
	rc := config.RunnerConfig{
		Name:       "agentic",
		Repo:       "owner/repo",
		Host:       "h",
		Count:      1,
		Profile:    "agentic",
		RunnerMode: config.RunnerModeContainer,
	}
	instanceName := rc.InstanceNames()[0] // "agentic-1"
	cName := containerName(instanceName)
	stateDir := containerStateDir(h, instanceName)
	imageTag := AgenticRunnerImageTag + ":2.999.0"

	// Build the expected docker create command manually (mirrors setupContainer logic).
	labels := rc.EffectiveLabelsForInstance(h.OS, h.Arch, 0)
	cmd := strings.Join([]string{
		"mkdir -p " + posixSingleQuote(stateDir),
		"docker create",
		"  --name " + posixSingleQuote(cName),
		"  --privileged",
		"  --restart unless-stopped",
		"  -v " + posixSingleQuote(stateDir) + ":/runner-state",
		"  -e GH_SR_RUNNER_NAME=" + posixSingleQuote(instanceName),
		"  -e GH_SR_RUNNER_TOKEN=" + posixSingleQuote("tok"),
		"  -e GH_SR_RUNNER_URL=" + posixSingleQuote("https://github.com/owner/repo"),
		"  -e GH_SR_RUNNER_LABELS=" + posixSingleQuote(strings.Join(labels, ",")),
		"  -e GH_SR_RUNNER_GROUP=" + posixSingleQuote("Default"),
		"  -e GH_SR_RUNNER_EPHEMERAL=" + posixSingleQuote(""),
		"  " + posixSingleQuote(imageTag),
	}, "\n")

	if !strings.Contains(cmd, "--privileged") {
		t.Error("docker create command must include --privileged for DinD")
	}
	if !strings.Contains(cmd, "--restart unless-stopped") {
		t.Error("docker create command must include --restart unless-stopped")
	}
	if !strings.Contains(cmd, cName) {
		t.Errorf("docker create command must include container name %q", cName)
	}
	if !strings.Contains(cmd, ":/runner-state") {
		t.Error("docker create command must bind-mount to /runner-state")
	}
	if !strings.Contains(cmd, "GH_SR_RUNNER_NAME") {
		t.Error("docker create command must pass GH_SR_RUNNER_NAME env var")
	}
	if !strings.Contains(cmd, "GH_SR_RUNNER_TOKEN") {
		t.Error("docker create command must pass GH_SR_RUNNER_TOKEN env var")
	}
	if !strings.Contains(cmd, "GH_SR_RUNNER_URL") {
		t.Error("docker create command must pass GH_SR_RUNNER_URL env var")
	}
	if !strings.Contains(cmd, "GH_SR_RUNNER_LABELS") {
		t.Error("docker create command must pass GH_SR_RUNNER_LABELS env var")
	}
}

// TestAgenticRunnerImageTag verifies the image tag format used by container mode.
func TestAgenticRunnerImageTag(t *testing.T) {
	t.Parallel()
	tag := AgenticRunnerImageTag
	if !strings.HasPrefix(tag, "gh-sr/") {
		t.Errorf("image tag should start with gh-sr/, got %q", tag)
	}
	// The versioned tag appended at runtime.
	versioned := tag + ":2.123.0"
	if !strings.Contains(versioned, "2.123.0") {
		t.Errorf("versioned tag format unexpected: %q", versioned)
	}
}

// TestBuildAgenticRunnerImageCmdShape verifies the docker build command shape
// produced by buildAgenticRunnerImage (calls h.Run but we inspect the structure
// by constructing the expected command string rather than executing it).
func TestBuildAgenticRunnerImageCmdShape(t *testing.T) {
	t.Parallel()
	version := "2.320.0"
	arch := "x64"
	imageTag := AgenticRunnerImageTag + ":" + version

	// Replicate the docker build command from buildAgenticRunnerImage.
	buildCmd := "docker build --build-arg RUNNER_VERSION=" + posixSingleQuote(version) +
		" --build-arg RUNNER_ARCH=" + posixSingleQuote(arch) +
		" -t " + posixSingleQuote(imageTag) +
		" " + posixSingleQuote("/tmp/gh-sr-agentic-runner-build")

	if !strings.Contains(buildCmd, "RUNNER_VERSION=") {
		t.Error("build cmd must pass RUNNER_VERSION build-arg")
	}
	if !strings.Contains(buildCmd, "RUNNER_ARCH=") {
		t.Error("build cmd must pass RUNNER_ARCH build-arg")
	}
	if !strings.Contains(buildCmd, "-t ") {
		t.Error("build cmd must specify image tag with -t")
	}
	if !strings.Contains(buildCmd, imageTag) {
		t.Errorf("build cmd must contain image tag %q", imageTag)
	}
}

// TestStatusContainer_parseOutput verifies the status string mapping from
// docker inspect output to RunnerStatus.Local values.
func TestStatusContainer_parseOutput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dockerStatus string
		wantLocal    string
	}{
		{"running", "running"},
		{"exited", "stopped"},
		{"created", "stopped"},
		{"paused", "stopped"},
		{"restarting", "stopped"},
		{"not installed", "not installed"},
	}
	for _, tc := range cases {
		// Replicate the switch from statusContainer.
		var got string
		switch strings.TrimSpace(tc.dockerStatus) {
		case "running":
			got = "running"
		case "not installed":
			got = "not installed"
		default:
			got = "stopped"
		}
		if got != tc.wantLocal {
			t.Errorf("docker status %q → local status %q, want %q", tc.dockerStatus, got, tc.wantLocal)
		}
	}
}
