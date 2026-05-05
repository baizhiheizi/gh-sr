package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

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
	if got := ContainerDockerName("my-agentic-1"); got != containerName("my-agentic-1") {
		t.Errorf("ContainerDockerName vs containerName: got %q", got)
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

func TestContainerRunnerImageTag(t *testing.T) {
	t.Parallel()
	base := AgenticRunnerImageTag + ":2.320.0"
	if got := ContainerRunnerImageTag("2.320.0", nil); got != base {
		t.Errorf("empty extras: got %q want %q", got, base)
	}
	if got := ContainerRunnerImageTag("2.320.0", []string{}); got != base {
		t.Errorf("empty slice: got %q want %q", got, base)
	}
	a := ContainerRunnerImageTag("2.320.0", []string{"sqlite3", "ffmpeg"})
	b := ContainerRunnerImageTag("2.320.0", []string{"ffmpeg", "sqlite3"})
	if a != b {
		t.Errorf("order should not matter: %q vs %q", a, b)
	}
	if want := base + "-x908d9db2"; a != want {
		t.Errorf("tag with extras: got %q want %q", a, want)
	}
	dup := ContainerRunnerImageTag("1.0.0", []string{"curl", "curl"})
	once := ContainerRunnerImageTag("1.0.0", []string{"curl"})
	if dup != once {
		t.Errorf("duplicates should be ignored: got %q vs %q", dup, once)
	}
}

func TestAgenticRunnerDockerWrapperEmbedsAwfBridgeHostMapping(t *testing.T) {
	t.Parallel()
	for _, needle := range []string{
		"needs_awf_agent_bridge_host",
		"AWF_HOST_DOCKER_INTERNAL_IP",
		`--add-host=host.docker.internal:"$AWF_HOST_DOCKER_INTERNAL_IP"`,
		"gh-aw-firewall/agent:",
		"GH_SR_DOCKER_WRAPPER_REAL",
	} {
		if !strings.Contains(agenticRunnerDockerWrapper, needle) {
			t.Fatalf("embedded docker-wrapper must contain %q", needle)
		}
	}
}

func TestAgenticRunnerDockerWrapperEmbedsMcpgSupervisorLogic(t *testing.T) {
	t.Parallel()
	for _, needle := range []string{
		"cleanup_mcpg_container",
		"mcpg_docker_child_pid",
		"gh-aw-mcpg-ghsr-",
		"docker_option_value",
		"is_mcpg_invocation",
		"mktemp -d",
		"<&0",
	} {
		if !strings.Contains(agenticRunnerDockerWrapper, needle) {
			t.Fatalf("embedded docker-wrapper must contain %q", needle)
		}
	}
}

func TestAgenticRunnerDockerWrapperStreamsMcpgConfig(t *testing.T) {
	t.Parallel()
	for _, forbidden := range []string{
		"stdin.json",
		">\"$mcpg_stdin\"",
	} {
		if strings.Contains(agenticRunnerDockerWrapper, forbidden) {
			t.Fatalf("embedded docker-wrapper must stream MCP config without temp-file stdin, found %q", forbidden)
		}
	}
}

func TestAgenticRunnerDockerWrapperRewritesGeneratedClaudeMCPConfig(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "mcp-servers.json")

	stdinLog := filepath.Join(tmp, "stdin-captured.txt")
	fakeDocker := filepath.Join(tmp, "docker")
	script := `#!/bin/bash
set -euo pipefail
cmd=${1:-}
shift || true
case "$cmd" in
rm)
	exit 0
	;;
run)
	prev=""
	for a in "$@"; do
		if [[ "$prev" == "--cidfile" ]]; then
			printf 'deadbeefcafe\n' >"$a"
			prev=""
			continue
		fi
		prev=""
		case "$a" in
		--cidfile) prev="--cidfile" ;;
		--cidfile=*) f="${a#*=}"; printf 'deadbeefcafe\n' >"$f" ;;
		esac
	done
	cat >"${FAKE_DOCKER_STDIN_LOG:?}"
	sleep 0.2
	printf '%s' '{"mcpServers":{"github":{"url":"http://host.docker.internal:80/mcp/github"},"safeoutputs":{"url":"http://host.docker.internal:80/mcp/safeoutputs"}}}' >"${FAKE_MCP_CONFIG_PATH:?}"
	sleep 2.2
	exit 0
	;;
*)
	exit 0
	;;
esac
`
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
	payload := `{"gateway":{"port":80,"domain":"host.docker.internal"}}`
	cmd := exec.Command("bash", "-c", `printf '%s' "$GHSR_PAYLOAD" | bash "$GHSR_WRAPPER" run -i --rm --network host ghcr.io/github/gh-aw-mcpg:1.0.0`)
	cmd.Env = append(os.Environ(),
		"GHSR_PAYLOAD="+payload,
		"GHSR_WRAPPER="+wrapper,
		"GH_SR_DOCKER_WRAPPER_REAL="+fakeDocker,
		"GH_SR_MCP_CONFIG_REWRITE_PATH="+configPath,
		"FAKE_MCP_CONFIG_PATH="+configPath,
		"FAKE_DOCKER_STDIN_LOG="+stdinLog,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wrapper generated config rewrite: %v\n%s", err, out)
	}
	combined := string(out)
	for _, needle := range []string{
		"[gh-sr:mcp-claude-urls] watcher_start",
		"[gh-sr:mcp-claude-urls] config_appeared",
		"[gh-sr:mcp-claude-urls] stable_file",
		"[gh-sr:mcp-claude-urls] rewrite_applied",
	} {
		if !strings.Contains(combined, needle) {
			t.Fatalf("expected docker-wrapper diagnostics missing %q; combined:\n%s", needle, out)
		}
	}
	if !strings.Contains(combined, "[gh-sr:mcp-claude-urls] watcher_exit") &&
		!strings.Contains(combined, "[gh-sr:mcp-claude-urls] watcher_stop") {
		t.Fatalf("expected watcher_exit or watcher_stop log; combined:\n%s", out)
	}
	if strings.Contains(combined, "[gh-sr:mcp-claude-urls] watcher_exit WARNING still_host_mcp_urls") {
		t.Fatalf("unexpected rewrite-failure log; combined:\n%s", out)
	}

	gotStdin, err := os.ReadFile(stdinLog)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotStdin) != payload {
		t.Fatalf("gateway stdin must remain schema-valid: got %q want %q", gotStdin, payload)
	}

	gotConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gotConfig), "host.docker.internal:80/mcp") {
		t.Fatalf("generated Claude MCP config was not rewritten:\n%s", gotConfig)
	}
	for _, want := range []string{
		"http://172.30.0.1:80/mcp/github",
		"http://172.30.0.1:80/mcp/safeoutputs",
	} {
		if !strings.Contains(string(gotConfig), want) {
			t.Fatalf("generated Claude MCP config missing %q:\n%s", want, gotConfig)
		}
	}
}

func TestAgenticRunnerDockerWrapperHeaderDocumentsConcurrencyVsShim(t *testing.T) {
	t.Parallel()
	for _, needle := range []string{
		"runner_mode: container",
		"/opt/gh-sr/docker-shim/docker",
		"[gh-sr:mcp-claude-urls]",
	} {
		if !strings.Contains(agenticRunnerDockerWrapper, needle) {
			t.Fatalf("embedded docker-wrapper header should mention %q", needle)
		}
	}
}

func TestAgenticRunnerEntrypointUsesBridgeDNSForInnerContainers(t *testing.T) {
	t.Parallel()

	if strings.Contains(agenticRunnerEntrypoint, `"dns": ["${DNSMASQ_LISTEN}", "8.8.8.8"]`) {
		t.Fatal("inner Docker containers must not use 127.0.0.1 as DNS; that loopback points at the child container")
	}
	if !strings.Contains(agenticRunnerEntrypoint, `"dns": ["${DOCKER0_IP}", "8.8.8.8"]`) {
		t.Fatalf("entrypoint should configure inner Docker DNS to the bridge IP, got:\n%s", agenticRunnerEntrypoint)
	}
}

// TestDockerWrapperInjection runs docker-wrapper.sh with GH_SR_DOCKER_WRAPPER_REAL=/bin/echo
// so argv transformations are observable without a Docker daemon.
func TestDockerWrapperInjection(t *testing.T) {
	t.Parallel()
	wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
	if _, err := os.Stat(wrapper); err != nil {
		t.Fatalf("docker-wrapper.sh: %v", err)
	}

	echoWrap := func(t *testing.T, args ...string) string {
		t.Helper()
		cmd := exec.Command("bash", append([]string{wrapper}, args...)...)
		cmd.Env = append(os.Environ(), "GH_SR_DOCKER_WRAPPER_REAL=/bin/echo")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bash %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	t.Run("mcpg_hostname", func(t *testing.T) {
		t.Parallel()
		got := echoWrap(t, "run", "-i", "--rm", "--network", "host", "ghcr.io/github/gh-aw-mcpg:1.0.0")
		// /bin/echo exits immediately; wrapper then runs cleanup (more echo lines).
		if !strings.Contains(got, "run --hostname gh-aw-mcpg") {
			t.Fatalf("got %q, missing hostname injection", got)
		}
		if !strings.Contains(got, "--cidfile") {
			t.Fatalf("got %q, missing --cidfile", got)
		}
		if !strings.Contains(got, "--name gh-aw-mcpg-ghsr-") {
			t.Fatalf("got %q, missing gh-sr MCP gateway name prefix", got)
		}
		if !strings.Contains(got, "ghcr.io/github/gh-aw-mcpg:1.0.0") {
			t.Fatalf("got %q, missing image", got)
		}
	})

	t.Run("mcpg_supervises_when_hostname_present", func(t *testing.T) {
		t.Parallel()
		args := []string{"run", "--hostname", "custom", "ghcr.io/github/gh-aw-mcpg:1"}
		got := echoWrap(t, args...)
		if strings.Contains(got, "--hostname gh-aw-mcpg") {
			t.Fatalf("got %q, should not duplicate hostname", got)
		}
		if !strings.Contains(got, "run --name gh-aw-mcpg-ghsr-") {
			t.Fatalf("got %q, missing supervised MCP gateway name", got)
		}
		if !strings.Contains(got, "--cidfile") {
			t.Fatalf("got %q, missing supervised MCP gateway cidfile", got)
		}
		if !strings.Contains(got, "--hostname custom") {
			t.Fatalf("got %q, missing caller hostname", got)
		}
	})

	t.Run("awf_agent_host_gateway", func(t *testing.T) {
		t.Parallel()
		got := echoWrap(t, "run", "--rm", "ghcr.io/github/gh-aw-firewall/agent:2.3.4")
		want := "run --add-host=host.docker.internal:172.30.0.1 --rm ghcr.io/github/gh-aw-firewall/agent:2.3.4"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("awf_agent_create", func(t *testing.T) {
		t.Parallel()
		got := echoWrap(t, "create", "ghcr.io/github/gh-aw-firewall/agent:edge")
		want := "create --add-host=host.docker.internal:172.30.0.1 ghcr.io/github/gh-aw-firewall/agent:edge"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("awf_agent_respects_gateway_env_override", func(t *testing.T) {
		t.Parallel()
		wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
		if _, err := os.Stat(wrapper); err != nil {
			t.Fatalf("docker-wrapper.sh: %v", err)
		}
		cmd := exec.Command("bash", wrapper, "run", "--rm", "ghcr.io/github/gh-aw-firewall/agent:9")
		cmd.Env = append(os.Environ(),
			"GH_SR_DOCKER_WRAPPER_REAL=/bin/echo",
			"GH_SR_AWF_BRIDGE_GATEWAY_IP=10.20.30.40",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bash wrapper: %v\n%s", err, out)
		}
		got := strings.TrimSpace(string(out))
		want := "run --add-host=host.docker.internal:10.20.30.40 --rm ghcr.io/github/gh-aw-firewall/agent:9"
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("awf_agent_skips_duplicate_add_host_equals", func(t *testing.T) {
		t.Parallel()
		args := []string{"run", "--add-host=host.docker.internal:172.17.0.1", "ghcr.io/github/gh-aw-firewall/agent:1"}
		got := echoWrap(t, args...)
		want := strings.Join(args, " ")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("awf_agent_skips_duplicate_add_host_two_arg", func(t *testing.T) {
		t.Parallel()
		args := []string{"run", "--add-host", "host.docker.internal:172.18.0.1", "ghcr.io/github/gh-aw-firewall/agent:1"}
		got := echoWrap(t, args...)
		want := strings.Join(args, " ")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("passthrough_non_aw", func(t *testing.T) {
		t.Parallel()
		args := []string{"run", "--rm", "alpine:latest", "sh", "-c", "true"}
		got := echoWrap(t, args...)
		want := strings.Join(args, " ")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("mcpg_takes_precedence_over_agent_image_name_collision", func(t *testing.T) {
		t.Parallel()
		// Hypothetical tag containing substring "agent" must not confuse parser;
		// only gh-aw-mcpg image constant is matched for hostname injection.
		got := echoWrap(t, "run", "ghcr.io/github/gh-aw-mcpg:agent-test")
		if !strings.Contains(got, "--hostname gh-aw-mcpg") {
			t.Fatalf("expected mcpg hostname injection, got %q", got)
		}
		if strings.Contains(got, "host.docker.internal:172.30.0.1") {
			t.Fatalf("did not expect AWF bridge host mapping for mcpg image, got %q", got)
		}
	})
}

func TestDockerWrapperMcpgDetachedRunDoesNotSupervise(t *testing.T) {
	t.Parallel()
	wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
	if _, err := os.Stat(wrapper); err != nil {
		t.Fatalf("docker-wrapper.sh: %v", err)
	}

	cmd := exec.Command(
		"bash",
		wrapper,
		"run",
		"--detach",
		"--name",
		"awmg-proxy",
		"ghcr.io/github/gh-aw-mcpg:v0.3.0",
	)
	cmd.Env = append(os.Environ(), "GH_SR_DOCKER_WRAPPER_REAL=/bin/echo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("detached mcpg wrapper run: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	if !strings.Contains(got, "run --hostname gh-aw-mcpg --detach --name awmg-proxy ghcr.io/github/gh-aw-mcpg:v0.3.0") {
		t.Fatalf("detached mcpg run should pass through with hostname only, got %q", got)
	}
	for _, forbidden := range []string{
		"--cidfile",
		"rm -f",
		"[gh-sr:mcp-claude-urls]",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("detached mcpg run must not use supervisor cleanup path; found %q in %q", forbidden, got)
		}
	}
}

func TestDockerWrapperMcpgCommandDetachArgStillSupervises(t *testing.T) {
	t.Parallel()
	wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
	if _, err := os.Stat(wrapper); err != nil {
		t.Fatalf("docker-wrapper.sh: %v", err)
	}

	cmd := exec.Command(
		"bash",
		wrapper,
		"run",
		"-i",
		"--rm",
		"ghcr.io/github/gh-aw-mcpg:v0.3.0",
		"helper",
		"-d",
	)
	cmd.Env = append(os.Environ(), "GH_SR_DOCKER_WRAPPER_REAL=/bin/echo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("foreground mcpg wrapper run: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	for _, want := range []string{
		"[gh-sr:mcp-claude-urls] watcher_start",
		"--cidfile",
		"rm -f",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("foreground mcpg run should still use supervisor path; missing %q in %q", want, got)
		}
	}
}

func TestDockerWrapperMcpgOptionValueWithDStillSupervises(t *testing.T) {
	t.Parallel()
	wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
	if _, err := os.Stat(wrapper); err != nil {
		t.Fatalf("docker-wrapper.sh: %v", err)
	}

	cmd := exec.Command(
		"bash",
		wrapper,
		"run",
		"--entrypoint",
		"-debug",
		"ghcr.io/github/gh-aw-mcpg:v0.3.0",
	)
	cmd.Env = append(os.Environ(), "GH_SR_DOCKER_WRAPPER_REAL=/bin/echo")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("foreground mcpg wrapper run: %v\n%s", err, out)
	}

	got := strings.TrimSpace(string(out))
	for _, want := range []string{
		"[gh-sr:mcp-claude-urls] watcher_start",
		"--cidfile",
		"rm -f",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("foreground mcpg run should not treat option values as detach flags; missing %q in %q", want, got)
		}
	}
}

// TestDockerWrapperMcpgSupervisedRunForwardsStdin verifies gh-aw's piped MCP JSON
// reaches the real docker child when the wrapper supervises docker run in the background.
func TestDockerWrapperMcpgSupervisedRunForwardsStdin(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	stdinLog := filepath.Join(tmp, "stdin-captured.txt")
	logPath := filepath.Join(tmp, "fake-docker.log")
	fakeDocker := filepath.Join(tmp, "docker")
	script := `#!/bin/bash
set -euo pipefail
LOG="${FAKE_DOCKER_LOG:?}"
STDIN_LOG="${FAKE_DOCKER_STDIN_LOG:?}"
echo "FULL:$*" >>"$LOG"
cmd=${1:-}
shift || true
case "$cmd" in
rm)
	echo "rm-line:$*" >>"$LOG"
	exit 0
	;;
run)
	prev=""
	for a in "$@"; do
		if [[ "$prev" == "--cidfile" ]]; then
			printf 'deadbeefcafe\n' >"$a"
			prev=""
			continue
		fi
		prev=""
		case "$a" in
		--cidfile) prev="--cidfile" ;;
		--cidfile=*) f="${a#*=}"; printf 'deadbeefcafe\n' >"$f" ;;
		esac
	done
	cat >"$STDIN_LOG"
	exit 0
	;;
*)
	exit 0
	;;
esac
`
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	payloadPath := filepath.Join(tmp, "mcp-config.json")
	jsonPayload := `{"gateway":{"port":80,"domain":"host.docker.internal"}}`
	if err := os.WriteFile(payloadPath, []byte(jsonPayload), 0o644); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
	if _, err := os.Stat(wrapper); err != nil {
		t.Fatalf("docker-wrapper.sh: %v", err)
	}

	cmd := exec.Command("bash", "-c", `cat "$GHSR_PAYLOAD" | bash "$GHSR_WRAPPER" run -i --rm --network host ghcr.io/github/gh-aw-mcpg:1.0.0`)
	cmd.Env = append(os.Environ(),
		"GHSR_PAYLOAD="+payloadPath,
		"GHSR_WRAPPER="+wrapper,
		"GH_SR_DOCKER_WRAPPER_REAL="+fakeDocker,
		"FAKE_DOCKER_LOG="+logPath,
		"FAKE_DOCKER_STDIN_LOG="+stdinLog,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wrapper stdin pipe: %v\n%s", err, out)
	}

	gotStdin, err := os.ReadFile(stdinLog)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotStdin) != jsonPayload {
		t.Fatalf("fake docker stdin: got %q want %q", gotStdin, jsonPayload)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(logData, []byte("run --hostname gh-aw-mcpg")) {
		t.Fatalf("expected supervised mcpg argv in fake docker log, got:\n%s", logData)
	}
	if !bytes.Contains(logData, []byte("gh-aw-mcpg-ghsr-")) {
		t.Fatalf("expected injected gateway name in fake docker log, got:\n%s", logData)
	}
}

// TestDockerWrapperMcpgSigtermRunsDockerRm verifies SIGTERM triggers docker rm -f cleanup
// against the recorded gateway name/cidfile (gh-aw stop_mcp_gateway.sh tracks wrapper PID).
func TestDockerWrapperMcpgSigtermRunsDockerRm(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "fake-docker.log")
	fakeDocker := filepath.Join(tmp, "docker")
	script := `#!/bin/bash
set -euo pipefail
LOG="${FAKE_DOCKER_LOG:?}"
echo "FULL:$*" >>"$LOG"
cmd=${1:-}
shift || true
case "$cmd" in
rm)
	echo "rm-line:$*" >>"$LOG"
	exit 0
	;;
run)
	prev=""
	for a in "$@"; do
		if [[ "$prev" == "--cidfile" ]]; then
			printf 'deadbeefcafe\n' >"$a"
			prev=""
			continue
		fi
		prev=""
		case "$a" in
		--cidfile) prev="--cidfile" ;;
		--cidfile=*) f="${a#*=}"; printf 'deadbeefcafe\n' >"$f" ;;
		esac
	done
	sleep 120
	exit 0
	;;
*)
	exit 0
	;;
esac
`
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join("agentic-runner-image", "docker-wrapper.sh")
	cmd := exec.Command("bash", wrapper, "run", "-i", "--rm", "--network", "host", "ghcr.io/github/gh-aw-mcpg:1.0.0")
	cmd.Env = append(os.Environ(),
		"GH_SR_DOCKER_WRAPPER_REAL="+fakeDocker,
		"FAKE_DOCKER_LOG="+logPath,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	waitErr := cmd.Wait()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok && exitErr.ExitCode() == 143 {
			// bash trap exit 143 is expected
		} else {
			t.Fatalf("wait: %v", waitErr)
		}
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("rm-line:")) {
		t.Fatalf("expected fake docker to receive rm cleanup, log:\n%s", data)
	}
	if !bytes.Contains(data, []byte("deadbeefcafe")) && !bytes.Contains(data, []byte("gh-aw-mcpg-ghsr-")) {
		t.Fatalf("expected rm to target container id or gh-aw-mcpg-ghsr name, log:\n%s", data)
	}
}

func TestAgenticRunnerDockerfileInstallsAWF(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"https://raw.githubusercontent.com/github/gh-aw-firewall/main/install.sh",
		"AWF_FORCE_BINARY=1",
		"awf --version",
	} {
		if !strings.Contains(agenticRunnerDockerfile, want) {
			t.Fatalf("Dockerfile should install and verify awf with %q, got:\n%s", want, agenticRunnerDockerfile)
		}
	}
}

func TestAgenticRunnerDockerfileDockerShimLayout(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"/opt/gh-sr/docker-shim/docker",
		"COPY docker-wrapper.sh /opt/gh-sr/docker-shim/docker",
		"/etc/profile.d/gh-sr-docker-shim.sh",
		"Defaults:runner secure_path=",
		"/opt/gh-sr/docker-shim:",
		"visudo -cf /etc/sudoers.d/runner-secure-path",
	} {
		if !strings.Contains(agenticRunnerDockerfile, want) {
			t.Fatalf("Dockerfile should contain %q, got:\n%s", want, agenticRunnerDockerfile)
		}
	}
	if strings.Contains(agenticRunnerDockerfile, "COPY docker-wrapper.sh /usr/local/bin/docker") {
		t.Fatal("Dockerfile should not install docker-wrapper at /usr/local/bin/docker")
	}
}

func TestAgenticRunnerEntrypointPrependsDockerShimPATH(t *testing.T) {
	t.Parallel()
	if !strings.Contains(agenticRunnerEntrypoint, "PATH=/opt/gh-sr/docker-shim:\\$PATH") {
		t.Fatalf("entrypoint RUNNER_ENV should prepend docker shim PATH, got:\n%s", agenticRunnerEntrypoint)
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
	ghVer := "vtest"
	rev := ContainerImageLayoutRevision(ghVer, nil)
	labelRev := posixSingleQuote(dockerLabelImageRevision + "=" + rev)
	labelCLI := posixSingleQuote(dockerLabelCLIVersion + "=" + ghVer)

	// Replicate the docker build command from buildAgenticRunnerImage.
	buildCmd := "docker build --build-arg RUNNER_VERSION=" + posixSingleQuote(version) +
		" --build-arg RUNNER_ARCH=" + posixSingleQuote(arch) +
		" --label " + labelRev +
		" --label " + labelCLI +
		" -t " + posixSingleQuote(imageTag) +
		" " + posixSingleQuote("/tmp/gh-sr-agentic-runner-build")

	if !strings.Contains(buildCmd, "RUNNER_VERSION=") {
		t.Error("build cmd must pass RUNNER_VERSION build-arg")
	}
	if !strings.Contains(buildCmd, "RUNNER_ARCH=") {
		t.Error("build cmd must pass RUNNER_ARCH build-arg")
	}
	if !strings.Contains(buildCmd, "--label ") {
		t.Error("build cmd must pass image revision labels")
	}
	if !strings.Contains(buildCmd, dockerLabelImageRevision) {
		t.Errorf("build cmd must reference label %q", dockerLabelImageRevision)
	}
	if !strings.Contains(buildCmd, rev) {
		t.Errorf("build cmd must contain layout revision %q", rev)
	}
	if !strings.Contains(buildCmd, "-t ") {
		t.Error("build cmd must specify image tag with -t")
	}
	if !strings.Contains(buildCmd, imageTag) {
		t.Errorf("build cmd must contain image tag %q", imageTag)
	}
}

func TestContainerRunnerImageExtraSorted(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		in    []string
		want  []string
		empty bool // if true, want nil not empty slice
	}{
		{
			name:  "nil",
			in:    nil,
			want:  nil,
			empty: true,
		},
		{
			name:  "empty slice",
			in:    []string{},
			want:  nil,
			empty: true,
		},
		{
			name:  "single item",
			in:    []string{"curl"},
			want:  []string{"curl"},
			empty: false,
		},
		{
			name:  "whitespace trimmed",
			in:    []string{"  git  ", "  curl  "},
			want:  []string{"curl", "git"},
			empty: false,
		},
		{
			name:  "empty strings filtered",
			in:    []string{"curl", "", "  ", "git"},
			want:  []string{"curl", "git"},
			empty: false,
		},
		{
			name:  "duplicates removed",
			in:    []string{"curl", "curl", "git"},
			want:  []string{"curl", "git"},
			empty: false,
		},
		{
			name:  "sorted ascending",
			in:    []string{"zlib", "curl", "ffmpeg"},
			want:  []string{"curl", "ffmpeg", "zlib"},
			empty: false,
		},
		{
			name:  "unsorted input, sorted output",
			in:    []string{"sqlite3", "ffmpeg", "curl"},
			want:  []string{"curl", "ffmpeg", "sqlite3"},
			empty: false,
		},
		{
			name:  "case sensitive dedup",
			in:    []string{"curl", "CURL", "Curl"},
			want:  []string{"CURL", "Curl", "curl"},
			empty: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := containerRunnerImageExtraSorted(tc.in)
			if tc.empty && got != nil {
				t.Fatalf("want nil, got %v", got)
			}
			if !tc.empty && len(got) != len(tc.want) {
				t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), tc.want, len(tc.want))
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("[%d]: got %q, want %q", i, got[i], w)
				}
			}
		})
	}
}

// TestParseContainerStatusInspectOutput verifies mapping from container+image inspect output.
func TestParseContainerStatusInspectOutput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line         string
		wantLocal    string
		wantImage    string
		wantImageRev string
	}{
		{"running|gh-sr/agentic-runner:2.320.0|sha256:abc|deadbeef", "running", "gh-sr/agentic-runner:2.320.0", "deadbeef"},
		{"running|gh-sr/agentic-runner:2.320.0-xa1b2c3d|sha256:x|", "running", "gh-sr/agentic-runner:2.320.0-xa1b2c3d", ""},
		{"exited|gh-sr/agentic-runner:1.0.0|sha256:1|rev1", "stopped", "gh-sr/agentic-runner:1.0.0", "rev1"},
		{"created|repo:tag|sha256:2|", "stopped", "repo:tag", ""},
		{"paused|x:y|sha256:3|r", "stopped", "x:y", "r"},
		{"restarting|x:y|sha256:4|r", "stopped", "x:y", "r"},
		{"not installed|||", "not installed", "", ""},
		{"not installed|a|b|c", "not installed", "", ""},
	}
	for _, tc := range cases {
		gotLocal, gotImage, gotRev := parseContainerStatusInspectOutput(tc.line)
		if gotLocal != tc.wantLocal || gotImage != tc.wantImage || gotRev != tc.wantImageRev {
			t.Errorf("line %q → (%q,%q,%q), want (%q,%q,%q)", tc.line, gotLocal, gotImage, gotRev, tc.wantLocal, tc.wantImage, tc.wantImageRev)
		}
	}
}

func TestContainerImageLayoutRevision_stable(t *testing.T) {
	t.Parallel()
	a := ContainerImageLayoutRevision("1.0.0", []string{"curl"})
	b := ContainerImageLayoutRevision("1.0.0", []string{"curl"})
	if a != b {
		t.Fatalf("expected stable revision, %q vs %q", a, b)
	}
	if len(a) != 12 {
		t.Fatalf("expected 12 hex chars, got %q len %d", a, len(a))
	}
	c := ContainerImageLayoutRevision("1.0.0", []string{"git"})
	if c == a {
		t.Fatal("different extras should change revision")
	}
}

func TestFormatContainerImageBuild(t *testing.T) {
	t.Parallel()
	if got := formatContainerImageBuild("not installed", "aaa", "bbb"); got != "-" {
		t.Errorf("not installed: got %q", got)
	}
	if got := formatContainerImageBuild("running", "aaa", ""); got != "?" {
		t.Errorf("missing label: got %q", got)
	}
	if got := formatContainerImageBuild("running", "abcd12345678", "abcd12345678"); got != "ok (abcd1234)" {
		t.Errorf("match: got %q", got)
	}
	if got := formatContainerImageBuild("running", "aaa", "bbbbbbbbbbbb"); got != "stale (bbbbbbbb)" {
		t.Errorf("stale: got %q", got)
	}
}
