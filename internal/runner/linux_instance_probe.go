package runner

import (
	"fmt"
	"strings"

	"github.com/an-lee/gh-sr/internal/autostart"
	"github.com/an-lee/gh-sr/internal/host"
)

type linuxInstanceProbeResult struct {
	dirExists bool
	svcSh     bool
	kind      autostart.Kind
}

// linuxInstanceProbe reports correlated Linux runner state in one SSH
// round-trip. includeDir adds the directory-existence marker used by orphan
// cleanup; the svc.sh and systemd-unit markers are always probed together
// (same win-class as the svc.sh + autostart fold in PR #361 and the
// orphan-plan probe in PR #358).
//
// Markers emitted by the shell on separate lines:
//
//	D - $DIR exists (orphan cleanup only; includeDir=true)
//	S - $DIR/svc.sh is present
//	U - user-level systemd unit is installed ($HOME/.config/systemd/user/...service)
//	Y - system-level systemd unit is installed (/etc/systemd/system/...service)
//
// The markers are independent: an instance with both svc.sh and a user unit
// emits both S and U. The Go-side parser maps the markers back to the
// (dirExists, svcSh, kind) tuple the call sites expect. (The Status path no
// longer runs this probe — it runs the nativeStatusProbeBody one-shot, which
// answers state AND version in the same single round trip — so the former V
// version marker is gone with its only reader.)
func linuxInstanceProbe(h *host.Host, instance string, includeDir bool) (linuxInstanceProbeResult, error) {
	var result linuxInstanceProbeResult

	san, err := autostart.SanitizeInstance(instance)
	if err != nil {
		return result, err
	}
	base := autostart.ServiceBasename(san)
	dir := h.RunnerDir(instance)
	userPath := fmt.Sprintf(`"$HOME/.config/systemd/user/%s.service"`, base)
	sysPath := fmt.Sprintf(`/etc/systemd/system/%s.service`, base)

	dirProbe := ""
	if includeDir {
		dirProbe = fmt.Sprintf(`if [ -d %s ]; then echo D; fi; `, dir)
	}
	// dir contains a literal $HOME prefix. Keep it unquoted so the remote shell
	// expands it; the instance segment was sanitized above.
	script := fmt.Sprintf(
		`%sif [ -f %s/svc.sh ]; then echo S; fi; `+
			`if [ -f %s ]; then echo U; `+
			`elif [ -f %s ]; then echo Y; `+
			`fi`,
		dirProbe, dir, userPath, sysPath,
	)
	out, err := h.Run(script)
	if err != nil {
		return result, err
	}
	// SplitSeq avoids the upfront []string allocation that strings.Split makes
	// for the entire marker output before iteration. linuxInstanceProbe runs
	// once per orphan cleanup pass (and per start/stop svc/autostart probe),
	// so the alloc drop compounds (see BenchmarkLinuxInstanceProbe).
	for line := range strings.SplitSeq(out, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "D":
			result.dirExists = true
		case trimmed == "S":
			result.svcSh = true
		case trimmed == "U":
			result.kind = autostart.KindSystemdUser
		case trimmed == "Y":
			result.kind = autostart.KindSystemdSystem
		}
	}
	return result, nil
}
