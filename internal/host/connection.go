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
	if cb, err := knownHostsCallback(); err == nil {
		hostKeyCallback = cb
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s@%s: %w", user, addr, err)
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

func knownHostsCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	return knownhosts.New(path)
}
