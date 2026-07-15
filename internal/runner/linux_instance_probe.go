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
// cleanup; the svc.sh and systemd-unit markers are always included.
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
	for _, line := range strings.Split(out, "\n") {
		switch strings.TrimSpace(line) {
		case "D":
			result.dirExists = true
		case "S":
			result.svcSh = true
		case "U":
			result.kind = autostart.KindSystemdUser
		case "Y":
			result.kind = autostart.KindSystemdSystem
		}
	}
	return result, nil
}
