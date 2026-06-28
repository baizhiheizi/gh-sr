package host

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/hostshell/ps"
)

// DetectOS probes the remote host for its operating system and returns "linux", "darwin", or "windows".
func DetectOS(h *Host) (string, error) {
	out, err := h.Run(`uname -s 2>/dev/null || echo UNKNOWN`)
	if err == nil {
		switch strings.TrimSpace(strings.ToLower(out)) {
		case "linux":
			return "linux", nil
		case "darwin":
			return "darwin", nil
		}
	}

	// uname failed or returned something unexpected -- try PowerShell (Windows over SSH).
	psOut, psErr := h.Run(ps.CommandLine("[Environment]::OSVersion.Platform"))
	if psErr == nil && strings.Contains(strings.ToLower(strings.TrimSpace(psOut)), "win") {
		return "windows", nil
	}
	// Also try pwsh.
	psOut, psErr = h.Run(`pwsh.exe -NoProfile -NonInteractive -Command "[Environment]::OSVersion.Platform"`)
	if psErr == nil && strings.Contains(strings.ToLower(strings.TrimSpace(psOut)), "win") {
		return "windows", nil
	}

	if err != nil {
		return "", fmt.Errorf("detecting OS: uname failed: %w", err)
	}
	return "", fmt.Errorf("detecting OS: uname returned %q", out)
}

// DetectArch probes the remote host for its CPU architecture and returns "amd64" or "arm64".
func DetectArch(h *Host) (string, error) {
	out, err := h.Run(`uname -m 2>/dev/null || echo UNKNOWN`)
	if err == nil {
		return normalizeArch(strings.TrimSpace(out))
	}

	// Try PowerShell for Windows.
	psOut, psErr := h.Run(ps.CommandLine("$env:PROCESSOR_ARCHITECTURE"))
	if psErr == nil {
		return normalizeArch(strings.TrimSpace(psOut))
	}
	psOut, psErr = h.Run(`pwsh.exe -NoProfile -NonInteractive -Command "$env:PROCESSOR_ARCHITECTURE"`)
	if psErr == nil {
		return normalizeArch(strings.TrimSpace(psOut))
	}

	return "", fmt.Errorf("detecting arch: %w", err)
}

// DetectDockerAvailable checks if Docker is installed and the daemon is reachable on the host.
func DetectDockerAvailable(h *Host) bool {
	var cmd string
	switch h.OS {
	case "windows":
		cmd = `docker info --format "{{.ServerVersion}}" 2>$null`
		out, err := h.RunShell(cmd)
		return err == nil && strings.TrimSpace(out) != ""
	default:
		prefix := ""
		if h.OS == "darwin" {
			prefix = `export PATH="/usr/local/bin:/opt/homebrew/bin:$PATH"; `
		}
		cmd = prefix + `docker info --format '{{.ServerVersion}}' 2>/dev/null`
		out, err := h.Run(cmd)
		return err == nil && strings.TrimSpace(out) != ""
	}
}

func normalizeArch(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture %q (expected x86_64/amd64 or aarch64/arm64)", raw)
	}
}
