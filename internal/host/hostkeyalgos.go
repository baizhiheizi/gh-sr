package host

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

// fallbackHostKeyAlgos mirrors x/crypto/ssh's default host-key algorithm
// preference (defaultHostKeyAlgos in x/crypto/ssh/common.go, with the insecure
// DSA variants dropped). When ~/.ssh/known_hosts pins keys for the dial
// target, those algorithms are ordered ahead of this tail.
//
// Go's default preference is RSA/ECDSA before Ed25519 — the opposite of what
// the OpenSSH client effectively does, because OpenSSH reorders
// HostKeyAlgorithms to prefer algorithms that already have a known_hosts
// entry. Without that reordering, a host whose ed25519 key is pinned (what a
// one-time `ssh` connect stores these days) but which also offers ecdsa/rsa
// host keys fails Go's verification with "knownhosts: key mismatch" even
// though it is exactly the host that was pinned: the client negotiates the
// unpinned ecdsa key and never offers the pinned ed25519 one.
var fallbackHostKeyAlgos = []string{
	ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSASHA512v01, ssh.CertAlgoRSAv01,
	ssh.CertAlgoECDSA256v01, ssh.CertAlgoECDSA384v01, ssh.CertAlgoECDSA521v01,
	ssh.CertAlgoED25519v01,
	ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSA,
	ssh.KeyAlgoED25519,
}

// pinnedKeyTypes are the known_hosts key types whose lines count as plain
// host pins. Lines with other types (or marker lines) are ignored so an
// unknown type can never leak into ClientConfig.HostKeyAlgorithms.
var pinnedKeyTypes = map[string]bool{
	ssh.KeyAlgoED25519:  true,
	ssh.KeyAlgoRSA:      true,
	ssh.KeyAlgoECDSA256: true,
	ssh.KeyAlgoECDSA384: true,
	ssh.KeyAlgoECDSA521: true,

	ssh.CertAlgoRSASHA256v01: true,
	ssh.CertAlgoRSASHA512v01: true,
	ssh.CertAlgoRSAv01:       true,
	ssh.CertAlgoECDSA256v01:  true,
	ssh.CertAlgoECDSA384v01:  true,
	ssh.CertAlgoECDSA521v01:  true,
	ssh.CertAlgoED25519v01:   true,
}

// hostKeyAlgosForDial returns the ClientConfig.HostKeyAlgorithms list to use
// when dialing addr against the known_hosts file at path: algorithms whose
// keys are pinned for the target host come first (in file order), then the
// fallback tail. It returns nil when nothing is pinned for the host, leaving
// the library default untouched.
func hostKeyAlgosForDial(path, addr string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	hostname, port := splitDialTarget(addr)

	var preferred []string
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		if strings.HasPrefix(fields[0], "@") {
			continue // @revoked / @cert-authority lines are not plain host pins
		}
		if !pinnedKeyTypes[fields[1]] {
			continue
		}
		if !hostPatternsMatch(fields[0], hostname, port) {
			continue
		}
		preferred = appendAlgos(preferred, keyTypeAlgos(fields[1])...)
	}
	if len(preferred) == 0 {
		return nil
	}
	return appendAlgos(preferred, fallbackHostKeyAlgos...)
}

// splitDialTarget splits a dial address into hostname and port, defaulting
// port 22 (mirroring the ":22" append in NewConnection).
func splitDialTarget(addr string) (hostname, port string) {
	if host, p, err := net.SplitHostPort(addr); err == nil {
		return host, p
	}
	return addr, "22"
}

// hostPatternsMatch reports whether any comma-separated pattern in field
// matches hostname. Hashed ("|1|…") patterns match the OpenSSH host spelling:
// the bare host for port 22, "[host]:port" otherwise. Negated patterns are
// OpenSSH exclusion syntax — a pin list containing them is not treated as a
// match (conservative: the algorithm preference is only an optimization).
func hostPatternsMatch(field, hostname, port string) bool {
	bracketed := hostname
	if port != "" && port != "22" {
		bracketed = "[" + hostname + "]:" + port
	}
	for _, pattern := range strings.Split(field, ",") {
		if pattern == "" || strings.HasPrefix(pattern, "!") {
			continue
		}
		if strings.HasPrefix(pattern, "|1|") {
			if hashedHostMatches(pattern, bracketed) {
				return true
			}
			continue
		}
		if pattern == bracketed {
			return true
		}
	}
	return false
}

// hashedHostMatches decodes an OpenSSH hashed known_hosts host pattern
// ("|1|base64(salt)|base64(hmac-sha256(salt, hostname))") and reports
// whether it matches hostname.
func hashedHostMatches(pattern, hostname string) bool {
	parts := strings.Split(pattern, "|")
	if len(parts) != 4 { // "", "1", salt, hash
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(hostname))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)) == parts[3]
}

// keyTypeAlgos maps a known_hosts key type to the host-key algorithms that
// present the same key. An ssh-rsa entry verifies against any RSA algorithm
// (prefer SHA-2 signatures; ssh-rsa alone means SHA-1), so all three are
// listed and the server picks the best one it supports.
func keyTypeAlgos(keyType string) []string {
	switch keyType {
	case ssh.KeyAlgoRSA:
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	case ssh.CertAlgoRSAv01, ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSASHA512v01:
		return []string{ssh.CertAlgoRSASHA512v01, ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSAv01}
	default:
		return []string{keyType}
	}
}

func appendAlgos(dst []string, algos ...string) []string {
	for _, a := range algos {
		seen := false
		for _, d := range dst {
			if d == a {
				seen = true
				break
			}
		}
		if !seen {
			dst = append(dst, a)
		}
	}
	return dst
}
