package host

import (
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
)

// mockExecutor implements the Executor interface for testing SSH-dependent functions.
type mockExecutor struct {
	output    string
	runErr    error
	uploadErr error
	closeErr  error

	// runFn if set is called instead of returning static output.
	// It receives the command and returns (output, error) to allow
	// multi-call sequences to be tested.
	runFn func(cmd string) (string, error)

	uploadCalled bool
	lastUpload   struct {
		local, remote string
	}
}

func (m *mockExecutor) Run(cmd string) (string, error) {
	if m.runFn != nil {
		return m.runFn(cmd)
	}
	return m.output, m.runErr
}

func (m *mockExecutor) Upload(localPath, remotePath string) error {
	m.uploadCalled = true
	m.lastUpload.local = localPath
	m.lastUpload.remote = remotePath
	return m.uploadErr
}

func (m *mockExecutor) Close() error {
	return m.closeErr
}

// newMockHost returns a Host with a mock executor pre-injected, bypassing SSH.
func newMockHost(name string, cfg config.HostConfig, mock *mockExecutor) *Host {
	h := NewHost(name, cfg)
	h.conn = mock
	return h
}

// matchCmd returns output for commands that start with any of the given prefixes.
func matchCmd(responses map[string]string) func(cmd string) (string, error) {
	return func(cmd string) (string, error) {
		for prefix, output := range responses {
			if strings.HasPrefix(cmd, prefix) {
				return output, nil
			}
		}
		return "", nil
	}
}