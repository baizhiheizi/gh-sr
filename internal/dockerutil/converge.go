package dockerutil

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/host"
)

// ConvergeOpts describes one container's idempotent deployment. Service
// prefixes the user-facing progress lines ("cache", "registry"); Noun names
// the container in error messages ("cache container", "registry mirror
// container"). Deploy creates the container from scratch; AfterRemove, when
// set, runs best-effort cleanup right after a stale container is removed
// (errors are ignored — it must never fail the converge).
type ConvergeOpts struct {
	Container   string
	LayoutLabel string
	LayoutRev   string
	Service     string
	Noun        string
	AfterRemove func()
	Deploy      func() error
}

// Converge drives the idempotent container lifecycle both service packages
// share: a running container with the current layout label is a no-op, a
// stopped one is started, a missing one is created via Deploy, and one
// stamped with an older layout (e.g. deployed by an earlier gh-sr) is removed
// — running AfterRemove first if set — and recreated. This is the single
// tested home of that state machine; Ensure paths hand in their config and a
// Deploy callback instead of re-implementing the switch.
func Converge(w io.Writer, h *host.Host, opts ConvergeOpts) error {
	state, err := ContainerState(h, opts.Container)
	if err != nil {
		return err
	}
	if state != "" {
		current, err := LayoutCurrent(h, opts.Container, opts.LayoutLabel, opts.LayoutRev)
		if err != nil {
			return err
		}
		if !current {
			fmt.Fprintf(w, "  %s: %s was deployed by an older gh-sr layout; recreating...\n", opts.Service, opts.Container)
			if _, err := h.Run("docker rm -f " + opts.Container); err != nil {
				return fmt.Errorf("removing outdated %s: %w", opts.Noun, err)
			}
			if opts.AfterRemove != nil {
				opts.AfterRemove()
			}
			state = ""
		}
	}
	switch state {
	case "running":
		return nil
	case "exists":
		fmt.Fprintf(w, "  %s: starting %s...\n", opts.Service, opts.Container)
		if _, err := h.Run("docker start " + opts.Container); err != nil {
			return fmt.Errorf("starting %s: %w", opts.Noun, err)
		}
		return nil
	default:
		return opts.Deploy()
	}
}
