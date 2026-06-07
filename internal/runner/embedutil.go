package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/hostshell"
)

// writeRemoteHeredocFile uploads content to a regular (non-executable) file at path
// on the remote host. The parent directory is created if missing, and the content is
// normalised via embedTextForRemoteWrite before being interpolated into the heredoc
// so callers don't have to remember CRLF→LF or GHSR_EOF escaping.
//
// path is single-quoted with posixSingleQuote so it is safe for paths with spaces or
// shell metacharacters; the helper should be the single point of truth for these
// writes so future changes (a different transport, signed uploads, etc.) only touch
// one location.
func writeRemoteHeredocFile(h *host.Host, path, content string) error {
	quoted := hostshell.PosixSingleQuote(path)
	cmd := fmt.Sprintf(`mkdir -p "$(dirname %s)"
cat > %s << 'GHSR_EOF'
%s
GHSR_EOF`,
		quoted, quoted, embedTextForRemoteWrite(content),
	)
	if _, err := h.Run(cmd); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// writeRemoteHeredocExecutable uploads content and chmod +x in a single logical
// operation. The two shell invocations are intentional: keeping the write and chmod
// separate (rather than concatenating them inside one heredoc) makes a chmod failure
// attributable to the chmod step rather than conflating it with the file write.
//
// If the file write succeeds and chmod fails, the file is left on disk (and
// executable-from-the-helper perspective it is a normal file). The error from chmod
// is wrapped with the file name so callers can surface it without losing context.
func writeRemoteHeredocExecutable(h *host.Host, path, content string) error {
	if err := writeRemoteHeredocFile(h, path, content); err != nil {
		return err
	}
	quoted := hostshell.PosixSingleQuote(path)
	if _, err := h.Run(fmt.Sprintf("chmod +x %s", quoted)); err != nil {
		return fmt.Errorf("chmod +x %s: %w", path, err)
	}
	return nil
}

// formatEmptyRemoteFile returns the shell command used to truncate a remote file to
// zero bytes via `: > path`. This is the empty-file counterpart of the heredoc write
// used for optional content (e.g. apt-packages-extra.txt when no extras are set).
func formatEmptyRemoteFile(path string) string {
	return fmt.Sprintf(": > %s", hostshell.PosixSingleQuote(path))
}

// joinExtraPackages formats a sorted, deduplicated apt package list as a newline-
// separated body suitable for embedding inside the heredoc. The trailing newline is
// preserved so package lines stay on their own when the resulting file is read back.
func joinExtraPackages(extraSorted []string) string {
	return strings.Join(extraSorted, "\n")
}
