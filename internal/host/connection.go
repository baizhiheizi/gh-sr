package host

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Executor abstracts command execution on a host. SSHConnection and
// LocalConnection both implement this interface.
type Executor interface {
	Run(cmd string) (string, error)
	Upload(localPath, remotePath string) error
	Close() error
}

type Connection struct {
	client *ssh.Client
}

func NewConnection(user, addr string) (*Connection, error) {
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}

	authMethods := collectAuthMethods()
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available (no agent, no keys)")
	}

	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	knownHostsPath := ""
	if cb, path, err := knownHostsCallback(); err == nil {
		hostKeyCallback = cb
		knownHostsPath = path
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}
	// Prefer host-key algorithms already pinned in known_hosts (like OpenSSH)
	// so verification uses the pinned key type instead of Go's default
	// RSA/ECDSA-first negotiation order.
	if knownHostsPath != "" {
		if algos := hostKeyAlgosForDial(knownHostsPath, addr); len(algos) > 0 {
			cfg.HostKeyAlgorithms = algos
		}
	}

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		host, _, splitErr := net.SplitHostPort(addr)
		if splitErr != nil {
			host = addr
		}
		return nil, fmt.Errorf("SSH dial %s@%s: %w%s", user, addr, err, hostKeyHint(err, user, host))
	}

	return &Connection{client: client}, nil
}

func (c *Connection) Run(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	return runWithCapture(cmd, func(stdout, stderr io.Writer) error {
		session.Stdout = stdout
		session.Stderr = stderr
		return session.Run(cmd)
	})
}

func (c *Connection) Upload(localPath, remotePath string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("creating SSH session for upload: %w", err)
	}
	defer session.Close()

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("opening local file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat local file: %w", err)
	}

	w, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("getting stdin pipe: %w", err)
	}

	go func() {
		defer w.Close()
		fmt.Fprintf(w, "C0644 %d %s\n", stat.Size(), filepath.Base(remotePath))
		io.Copy(w, f)
		fmt.Fprint(w, "\x00")
	}()

	dir := filepath.Dir(remotePath)
	return session.Run(fmt.Sprintf("scp -t '%s'", strings.ReplaceAll(dir, "'", "'\\''")))
}

func (c *Connection) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// HasSSHAuth reports whether any SSH client authentication method is available
// (ssh-agent or a readable default private key under ~/.ssh).
func HasSSHAuth() bool {
	return len(collectAuthMethods()) > 0
}

func collectAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	if agentAuth := sshAgentAuth(); agentAuth != nil {
		methods = append(methods, agentAuth)
	}

	for _, keyPath := range defaultKeyPaths() {
		if m := keyFileAuth(keyPath); m != nil {
			methods = append(methods, m)
		}
	}

	return methods
}

func sshAgentAuth() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

func keyFileAuth(path string) ssh.AuthMethod {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(signer)
}

func defaultKeyPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	sshDir := filepath.Join(home, ".ssh")
	return []string{
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_rsa"),
		filepath.Join(sshDir, "id_ecdsa"),
	}
}

func knownHostsCallback() (ssh.HostKeyCallback, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, "", err
	}
	return cb, path, nil
}

// hostKeyHint appends a remediation hint to host-key verification failures,
// which otherwise surface as a bare "knownhosts: key mismatch" / "key is
// unknown" from the Go SSH handshake.
func hostKeyHint(err error, user, host string) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "knownhosts: key mismatch"):
		return fmt.Sprintf(" — the host key presented by %s differs from its ~/.ssh/known_hosts entry; if the host was rebuilt or its key changed on purpose, verify it and run `ssh-keygen -R %s`, then connect once with plain `ssh` to pin the new key", host, host)
	case strings.Contains(msg, "knownhosts: key is unknown"):
		return fmt.Sprintf(" — %s is not in ~/.ssh/known_hosts; connect once with plain `ssh %s` to accept and pin its host key", host, user+"@"+host)
	}
	return ""
}
