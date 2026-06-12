package host

import (
	"strings"

	"github.com/an-lee/gh-sr/internal/config"
)

// newMockHost returns a Host with mock executor pre-injected, bypassing SSH.
func newMockHost(name string, cfg config.HostConfig, mock Executor) *Host {
	h := NewHost(name, cfg)
	h.SetConn(mock)
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
