package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/runner"
)

// Key-press construction helpers. bubbletea v2 models key events as
// tea.KeyPressMsg (a uv.Key): Code carries the rune or a Key* constant and
// Mod carries modifiers. String() — the value the dashboard's Update
// switches on — is derived from those fields, so tests build the exact
// events a terminal would deliver ("q", "ctrl+c", "enter", "shift+g", …).

func keyChar(c rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: c} }

// keyNamed builds a special-key press (tea.KeyEnter, tea.KeyEscape, …).
func keyNamed(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

func keyCombo(mod tea.KeyMod, c rune) tea.KeyPressMsg { return tea.KeyPressMsg{Mod: mod, Code: c} }

func sendKey(m *dashboardModel, msg tea.KeyPressMsg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

func sendMsg(m *dashboardModel, msg tea.Msg) tea.Cmd {
	_, cmd := m.Update(msg)
	return cmd
}

// updateTestModel builds a dashboardModel with the standard fixture: two
// hosts, two runners (one org-scoped, one repo-scoped) and two matching
// status rows. Every Update test starts from this state so the assertions
// exercise the state machine, not fixture assembly.
func updateTestModel(t *testing.T) *dashboardModel {
	t.Helper()
	return &dashboardModel{
		opts: DashboardOpts{ConfigPath: "runners.yml", EnvPath: ".env"},
		cfg: &config.Config{
			Hosts: map[string]config.HostConfig{
				"alpha": {Addr: "alpha.example"},
				"bravo": {Addr: "bravo.example"},
			},
			Runners: []config.RunnerConfig{
				{Name: "r1", Repo: "o/r1"},
				{Name: "r2", Org: "my-org"},
			},
		},
		statuses: []runner.RunnerStatus{
			{Instance: "runner-1", Host: "alpha", Repo: "o/r1", Mode: "container", Local: "running", Remote: "online"},
			{Instance: "runner-2", Host: "bravo", Repo: "o/r2", Mode: "container", Local: "stopped", Remote: "offline"},
		},
		panel: panelMain,
	}
}

// TestUpdate_mainPanelQuit pins the exit keys: q and ctrl+c on the main
// panel must return a command that produces tea.QuitMsg. A regression that
// drops either binding leaves users stuck in the alternate screen (their
// only escape is killing the process).
func TestUpdate_mainPanelQuit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"q quits", keyChar('q')},
		{"ctrl+c quits", keyCombo(tea.ModCtrl, 'c')},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := updateTestModel(t)
			cmd := sendKey(m, tc.key)
			if cmd == nil {
				t.Fatal("quit key returned nil cmd; the user has no exit path")
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("quit key cmd produced %T, want tea.QuitMsg", cmd())
			}
		})
	}
}

// TestUpdate_mainPanelNavigation pins cursor movement and its bounds:
// j/down advance, k/up retreat, and both clamp at the list edges. A lost
// clamp either panics (index out of range in selectedInstance callers) or
// leaves the cursor on an invisible row.
func TestUpdate_mainPanelNavigation(t *testing.T) {
	t.Parallel()
	t.Run("j and down advance, k and up retreat", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		sendKey(m, keyChar('j'))
		if m.cursor != 1 {
			t.Errorf("after j, cursor = %d, want 1", m.cursor)
		}
		sendKey(m, keyNamed(tea.KeyDown))
		if m.cursor != 1 {
			t.Errorf("after down at last row, cursor = %d, want 1 (clamped)", m.cursor)
		}
		sendKey(m, keyChar('k'))
		sendKey(m, keyNamed(tea.KeyUp))
		if m.cursor != 0 {
			t.Errorf("after k then up, cursor = %d, want 0 (clamped)", m.cursor)
		}
	})
	t.Run("enter opens action menu, esc-free panels unaffected", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.panel != panelActionMenu {
			t.Errorf("enter moved panel to %v, want panelActionMenu", m.panel)
		}
		if m.menuCursor != 0 {
			t.Errorf("enter reset menuCursor to %d, want 0", m.menuCursor)
		}
	})
	t.Run("enter with no rows is a no-op", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.statuses = nil
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.panel != panelMain {
			t.Errorf("enter with no statuses moved panel to %v, want panelMain", m.panel)
		}
	})
}

// TestUpdate_mainPanelShortcuts pins the single-key panel openers: r
// refreshes statuses, h loads host metrics, g and f open the global and
// filter menus, and ? toggles the help overlay (twice, back to off).
func TestUpdate_mainPanelShortcuts(t *testing.T) {
	t.Parallel()
	t.Run("r requests a refresh", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		cmd := sendKey(m, keyChar('r'))
		if !m.loading {
			t.Error("r must set loading while the refresh is in flight")
		}
		if cmd == nil {
			t.Error("r must return the refresh cmd")
		}
	})
	t.Run("h requests host metrics", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		cmd := sendKey(m, keyChar('h'))
		if !m.metricsLoading {
			t.Error("h must set metricsLoading")
		}
		if cmd == nil {
			t.Error("h must return the metrics cmd")
		}
	})
	t.Run("g opens global menu", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		sendKey(m, keyChar('g'))
		if m.panel != panelGlobalMenu || m.menuCursor != 0 {
			t.Errorf("g moved to panel=%v cursor=%d, want panelGlobalMenu/0", m.panel, m.menuCursor)
		}
	})
	t.Run("f opens filter menu", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		sendKey(m, keyChar('f'))
		if m.panel != panelFilterMenu || m.menuCursor != 0 {
			t.Errorf("f moved to panel=%v cursor=%d, want panelFilterMenu/0", m.panel, m.menuCursor)
		}
	})
	t.Run("? toggles help overlay", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		sendKey(m, keyChar('?'))
		if !m.showHelp {
			t.Error("? must enable showHelp")
		}
		sendKey(m, keyChar('?'))
		if m.showHelp {
			t.Error("second ? must disable showHelp")
		}
	})
}

// TestUpdate_busyGate pins the busy modal: while an op runs on the main
// panel, only q / ctrl+c get through (to quit); every other key is
// swallowed so a stray keypress cannot start a second op mid-flight. '?'
// is deliberately still live (checked before the gate).
func TestUpdate_busyGate(t *testing.T) {
	t.Parallel()
	t.Run("q still quits while busy", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy = true
		cmd := sendKey(m, keyChar('q'))
		if cmd == nil {
			t.Fatal("q while busy returned nil cmd; user cannot exit a hung op")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("q while busy produced %T, want tea.QuitMsg", cmd())
		}
	})
	t.Run("other keys are swallowed", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy = true
		for _, key := range []tea.KeyPressMsg{keyChar('r'), keyChar('h'), keyChar('g'), keyChar('f'), keyNamed(tea.KeyEnter), keyChar('j')} {
			if cmd := sendKey(m, key); cmd != nil {
				t.Errorf("key %q while busy returned a cmd; it must be swallowed", key.String())
			}
		}
		if m.panel != panelMain {
			t.Errorf("busy panel drifted to %v, want panelMain", m.panel)
		}
	})
	t.Run("? still toggles help while busy", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy = true
		sendKey(m, keyChar('?'))
		if !m.showHelp {
			t.Error("? while busy must still toggle help")
		}
	})
}

// TestUpdate_actionMenu walks the runner action menu end to end: nav
// bounds across the seven items, esc back, and one enter per action.
// Actions 0–5 flip to the busy state with the right op label and hand back
// a command (never executed here — the ops hit SSH); action 6 routes to
// the y/n remove confirmation instead.
func TestUpdate_actionMenu(t *testing.T) {
	t.Parallel()
	t.Run("nav clamps at both ends", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelActionMenu
		for i := 0; i < 20; i++ {
			sendKey(m, keyChar('j'))
		}
		if m.menuCursor != len(actionMenuLabels)-1 {
			t.Errorf("menuCursor after overshoot = %d, want %d", m.menuCursor, len(actionMenuLabels)-1)
		}
		for i := 0; i < 20; i++ {
			sendKey(m, keyChar('k'))
		}
		if m.menuCursor != 0 {
			t.Errorf("menuCursor after undershoot = %d, want 0", m.menuCursor)
		}
	})
	t.Run("esc returns to main", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelActionMenu
		sendKey(m, keyNamed(tea.KeyEscape))
		if m.panel != panelMain || m.menuCursor != 0 {
			t.Errorf("esc moved to panel=%v cursor=%d, want panelMain/0", m.panel, m.menuCursor)
		}
	})
	for _, tc := range []struct {
		cursor int
		op     string
	}{
		{0, "setup"}, {1, "up"}, {2, "down"}, {3, "restart"}, {4, "update"}, {5, "logs"},
	} {
		t.Run("enter on "+tc.op, func(t *testing.T) {
			t.Parallel()
			m := updateTestModel(t)
			m.panel = panelActionMenu
			m.menuCursor = tc.cursor
			cmd := sendKey(m, keyNamed(tea.KeyEnter))
			if m.panel != panelMain {
				t.Errorf("%s left panel at %v, want panelMain", tc.op, m.panel)
			}
			if !m.busy || m.busyOp != tc.op {
				t.Errorf("%s busy=%v busyOp=%q, want busy/%q", tc.op, m.busy, m.busyOp, tc.op)
			}
			if m.toast != "" {
				t.Errorf("%s must clear the toast, got %q", tc.op, m.toast)
			}
			if cmd == nil {
				t.Errorf("%s must return its op cmd", tc.op)
			}
		})
	}
	t.Run("enter on remove asks for confirmation", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelActionMenu
		m.menuCursor = 6
		if cmd := sendKey(m, keyNamed(tea.KeyEnter)); cmd != nil {
			t.Error("remove entry must only open the confirm panel, not run")
		}
		if m.panel != panelConfirmRemove {
			t.Errorf("panel = %v, want panelConfirmRemove", m.panel)
		}
		if m.confirmRemoveInst != "runner-1" {
			t.Errorf("confirmRemoveInst = %q, want runner-1 (the selected row)", m.confirmRemoveInst)
		}
	})
	t.Run("enter with no selected row falls back to main", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.statuses = nil
		m.panel = panelActionMenu
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.panel != panelMain {
			t.Errorf("panel = %v, want panelMain", m.panel)
		}
		if m.busy {
			t.Error("no op must start without a selected row")
		}
	})
}

// TestUpdate_confirmPanels pins both y/n confirmations: y runs the op
// through the busy path (the command is not executed here), n/esc cancel
// back to the main panel without touching anything.
func TestUpdate_confirmPanels(t *testing.T) {
	t.Parallel()
	t.Run("remove: y runs the removal", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelConfirmRemove
		m.confirmRemoveInst = "runner-2"
		cmd := sendKey(m, keyChar('y'))
		if m.panel != panelMain || m.menuCursor != 0 {
			t.Errorf("y left panel=%v cursor=%d, want panelMain/0", m.panel, m.menuCursor)
		}
		if !m.busy || m.busyOp != "remove" {
			t.Errorf("y busy=%v busyOp=%q, want busy/remove", m.busy, m.busyOp)
		}
		if cmd == nil {
			t.Error("y must return the remove cmd")
		}
	})
	t.Run("remove: n and esc cancel", func(t *testing.T) {
		t.Parallel()
		for _, key := range []tea.KeyPressMsg{keyChar('n'), keyNamed(tea.KeyEscape)} {
			m := updateTestModel(t)
			m.panel = panelConfirmRemove
			sendKey(m, key)
			if m.panel != panelMain || m.busy {
				t.Errorf("%q cancel left panel=%v busy=%v, want panelMain/false", key.String(), m.panel, m.busy)
			}
		}
	})
	t.Run("cleanup: y runs the cleanup", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelConfirmCleanup
		cmd := sendKey(m, keyChar('y'))
		if !m.busy || m.busyOp != "cleanup" {
			t.Errorf("y busy=%v busyOp=%q, want busy/cleanup", m.busy, m.busyOp)
		}
		if m.panel != panelMain {
			t.Errorf("y left panel at %v, want panelMain", m.panel)
		}
		if cmd == nil {
			t.Error("y must return the cleanup cmd")
		}
	})
	t.Run("cleanup: n and esc cancel", func(t *testing.T) {
		t.Parallel()
		for _, key := range []tea.KeyPressMsg{keyChar('n'), keyNamed(tea.KeyEscape)} {
			m := updateTestModel(t)
			m.panel = panelConfirmCleanup
			sendKey(m, key)
			if m.panel != panelMain || m.busy {
				t.Errorf("%q cancel left panel=%v busy=%v, want panelMain/false", key.String(), m.panel, m.busy)
			}
		}
	})
}

// TestUpdate_globalMenu exercises every entry of the global menu (g). The
// ops-, doctor-, validate- and editor-backed commands are asserted by
// state transition and non-nil command only — they are never executed, so
// the tests stay hermetic.
func TestUpdate_globalMenu(t *testing.T) {
	t.Parallel()
	t.Run("esc returns to main", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		sendKey(m, keyNamed(tea.KeyEscape))
		if m.panel != panelMain {
			t.Errorf("panel = %v, want panelMain", m.panel)
		}
	})
	t.Run("doctor arms the busy doctor op", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		cmd := sendKey(m, keyNamed(tea.KeyEnter)) // cursor 0
		if !m.busy || m.busyOp != "doctor" {
			t.Errorf("busy=%v busyOp=%q, want busy/doctor", m.busy, m.busyOp)
		}
		if cmd == nil {
			t.Error("doctor must return its cmd")
		}
	})
	t.Run("host metrics refreshes", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		m.menuCursor = 1
		cmd := sendKey(m, keyNamed(tea.KeyEnter))
		if !m.metricsLoading {
			t.Error("host metrics entry must set metricsLoading")
		}
		if cmd == nil {
			t.Error("host metrics entry must return the metrics cmd")
		}
	})
	t.Run("cleanup asks for confirmation", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		m.menuCursor = 2
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.panel != panelConfirmCleanup {
			t.Errorf("panel = %v, want panelConfirmCleanup", m.panel)
		}
	})
	t.Run("show configuration opens the scroll panel", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		m.menuCursor = 3
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.panel != panelScroll || m.scrollTitle != "Configuration" {
			t.Errorf("panel=%v title=%q, want panelScroll/Configuration", m.panel, m.scrollTitle)
		}
		if m.scrollRaw == "" {
			t.Error("configuration scroll content must not be empty")
		}
	})
	t.Run("validate arms the busy validate op", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		m.menuCursor = 4
		cmd := sendKey(m, keyNamed(tea.KeyEnter))
		if !m.busy || m.busyOp != "validate" {
			t.Errorf("busy=%v busyOp=%q, want busy/validate", m.busy, m.busyOp)
		}
		if cmd == nil {
			t.Error("validate must return its cmd")
		}
	})
	t.Run("edit config with a missing file reports the error", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.opts.ConfigPath = filepath.Join(t.TempDir(), "missing.yml")
		m.panel = panelGlobalMenu
		m.menuCursor = 5
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.busy {
			t.Error("a missing config file must not arm the editor")
		}
		if !strings.Contains(m.lastErr, "config file") {
			t.Errorf("lastErr = %q, want a config-file error", m.lastErr)
		}
	})
	t.Run("edit config with an existing file arms the editor", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.opts.ConfigPath = filepath.Join(t.TempDir(), "runners.yml")
		if err := os.WriteFile(m.opts.ConfigPath, []byte("hosts: {}\n"), 0o600); err != nil {
			t.Fatalf("seed config: %v", err)
		}
		m.panel = panelGlobalMenu
		m.menuCursor = 5
		cmd := sendKey(m, keyNamed(tea.KeyEnter))
		if !m.busy {
			t.Error("editor must be armed for an existing config file")
		}
		if cmd == nil {
			t.Error("editor entry must return its exec cmd")
		}
	})
	t.Run("edit env bootstraps a missing env file and arms the editor", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.opts.EnvPath = filepath.Join(t.TempDir(), "nested", "env")
		m.panel = panelGlobalMenu
		m.menuCursor = 6
		cmd := sendKey(m, keyNamed(tea.KeyEnter))
		if !m.busy {
			t.Error("editor must be armed after ensureEnvFile bootstrap")
		}
		if cmd == nil {
			t.Error("editor entry must return its exec cmd")
		}
		if _, err := os.Stat(m.opts.EnvPath); err != nil {
			t.Errorf("env file not bootstrapped: %v", err)
		}
	})
	t.Run("filter entries open the pickers", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		m.menuCursor = 7
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.panel != panelFilterHost || len(m.filterHostChoices) != 2 {
			t.Errorf("panel=%v choices=%v, want panelFilterHost with both hosts", m.panel, m.filterHostChoices)
		}
		m2 := updateTestModel(t)
		m2.panel = panelGlobalMenu
		m2.menuCursor = 8
		sendKey(m2, keyNamed(tea.KeyEnter))
		if m2.panel != panelFilterRepo || len(m2.filterRepoChoices) != 2 {
			t.Errorf("panel=%v choices=%v, want panelFilterRepo with both scopes", m2.panel, m2.filterRepoChoices)
		}
	})
	t.Run("clear filters resets and refreshes", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.tuiHostFilter = "alpha"
		m.tuiRepoFilter = "org:my-org"
		m.panel = panelGlobalMenu
		m.menuCursor = 9
		cmd := sendKey(m, keyNamed(tea.KeyEnter))
		if m.tuiHostFilter != "" || m.tuiRepoFilter != "" {
			t.Errorf("filters after clear: host=%q repo=%q, want both empty", m.tuiHostFilter, m.tuiRepoFilter)
		}
		if !m.loading {
			t.Error("clearing filters must trigger a refresh (loading)")
		}
		if cmd == nil {
			t.Error("clearing filters must return the refresh cmd")
		}
	})
	t.Run("nav clamps at the last entry", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelGlobalMenu
		for i := 0; i < 50; i++ {
			sendKey(m, keyChar('j'))
		}
		if m.menuCursor != len(globalMenuLabels)-1 {
			t.Errorf("menuCursor = %d, want %d", m.menuCursor, len(globalMenuLabels)-1)
		}
	})
}

// TestUpdate_filterPanels covers the f-menu and both pick lists: esc back,
// enter opens the matching picker (or clears), and a picked value becomes
// the live filter and triggers a refresh. The empty-list enter path must
// fall back to main rather than index out of range.
func TestUpdate_filterPanels(t *testing.T) {
	t.Parallel()
	t.Run("filter menu opens host picker", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelFilterMenu
		sendKey(m, keyNamed(tea.KeyEnter)) // cursor 0 → host
		if m.panel != panelFilterHost || len(m.filterHostChoices) != 2 {
			t.Errorf("panel=%v choices=%v, want panelFilterHost/2", m.panel, m.filterHostChoices)
		}
	})
	t.Run("filter menu opens repo picker", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelFilterMenu
		m.menuCursor = 1
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.panel != panelFilterRepo || len(m.filterRepoChoices) != 2 {
			t.Errorf("panel=%v choices=%v, want panelFilterRepo/2", m.panel, m.filterRepoChoices)
		}
	})
	t.Run("filter menu clears all filters", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.tuiHostFilter = "alpha"
		m.tuiRepoFilter = "o/r1"
		m.panel = panelFilterMenu
		m.menuCursor = 2
		cmd := sendKey(m, keyNamed(tea.KeyEnter))
		if m.tuiHostFilter != "" || m.tuiRepoFilter != "" {
			t.Errorf("filters after clear: host=%q repo=%q", m.tuiHostFilter, m.tuiRepoFilter)
		}
		if !m.loading || cmd == nil {
			t.Error("clear must set loading and return the refresh cmd")
		}
	})
	t.Run("picking a host applies the filter", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelFilterHost
		m.filterHostChoices = []string{"alpha", "bravo"}
		m.menuCursor = 1
		cmd := sendKey(m, keyNamed(tea.KeyEnter))
		if m.tuiHostFilter != "bravo" {
			t.Errorf("tuiHostFilter = %q, want bravo", m.tuiHostFilter)
		}
		if m.panel != panelMain || !m.loading || cmd == nil {
			t.Errorf("after pick: panel=%v loading=%v cmd=%v, want panelMain/true/non-nil", m.panel, m.loading, cmd)
		}
	})
	t.Run("picking a repo applies the filter", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelFilterRepo
		m.filterRepoChoices = []string{"org:my-org", "o/r1"}
		sendKey(m, keyNamed(tea.KeyEnter))
		if m.tuiRepoFilter != "org:my-org" {
			t.Errorf("tuiRepoFilter = %q, want org:my-org", m.tuiRepoFilter)
		}
	})
	t.Run("enter on an empty list falls back to main", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelFilterHost
		m.filterHostChoices = nil
		if cmd := sendKey(m, keyNamed(tea.KeyEnter)); cmd != nil {
			t.Error("empty pick list must not return a cmd")
		}
		if m.panel != panelMain {
			t.Errorf("panel = %v, want panelMain", m.panel)
		}
	})
	t.Run("pick lists clamp and esc back", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelFilterHost
		m.filterHostChoices = []string{"alpha"}
		sendKey(m, keyChar('j'))
		if m.menuCursor != 0 {
			t.Errorf("menuCursor after j on a 1-item list = %d, want 0", m.menuCursor)
		}
		sendKey(m, keyNamed(tea.KeyEscape))
		if m.panel != panelMain || m.menuCursor != 0 {
			t.Errorf("esc moved to panel=%v cursor=%d, want panelMain/0", m.panel, m.menuCursor)
		}
	})
}

// TestUpdate_hostMetricsPanel pins the host-metrics overlay: esc/q return
// to the main panel and drop the metrics, r re-requests them, and any
// other key is inert.
func TestUpdate_hostMetricsPanel(t *testing.T) {
	t.Parallel()
	for _, key := range []tea.KeyPressMsg{keyNamed(tea.KeyEscape), keyChar('q')} {
		m := updateTestModel(t)
		m.panel = panelHostMetrics
		m.hostMetrics = []host.HostMetrics{{Name: "alpha"}}
		sendKey(m, key)
		if m.panel != panelMain || m.hostMetrics != nil {
			t.Errorf("%q left panel=%v metrics=%v, want panelMain/cleared", key.String(), m.panel, m.hostMetrics)
		}
	}
	t.Run("r refreshes", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelHostMetrics
		if cmd := sendKey(m, keyChar('r')); cmd == nil {
			t.Error("r must return the metrics cmd")
		}
		if !m.metricsLoading {
			t.Error("r must set metricsLoading")
		}
	})
}

// TestUpdate_scrollKeys drives the scroll panel through Update for the
// full key matrix (line, page, home/end and both exit keys) with bounds
// assertions against the wrapped line count.
func TestUpdate_scrollKeys(t *testing.T) {
	t.Parallel()
	newScrolled := func(t *testing.T) *dashboardModel {
		t.Helper()
		m := updateTestModel(t)
		m.width, m.height = 80, 24 // page = height-6 = 18
		m.setScroll("Logs", strings.Repeat("line\n", 50))
		return m
	}
	t.Run("line keys clamp at both ends", func(t *testing.T) {
		t.Parallel()
		m := newScrolled(t)
		last := len(m.scrollLines) - 1
		for i := 0; i < last+5; i++ {
			sendKey(m, keyChar('j'))
		}
		if m.scrollOff != last {
			t.Errorf("scrollOff after overshoot = %d, want %d", m.scrollOff, last)
		}
		for i := 0; i < last+5; i++ {
			sendKey(m, keyChar('k'))
		}
		if m.scrollOff != 0 {
			t.Errorf("scrollOff after undershoot = %d, want 0", m.scrollOff)
		}
	})
	t.Run("page keys", func(t *testing.T) {
		t.Parallel()
		m := newScrolled(t)
		last := len(m.scrollLines) - 1
		sendKey(m, keyCombo(tea.ModCtrl, 'd'))
		if m.scrollOff != min(18, last) {
			t.Errorf("scrollOff after ctrl+d = %d, want %d", m.scrollOff, min(18, last))
		}
		sendKey(m, keyCombo(tea.ModCtrl, 'u'))
		if m.scrollOff != 0 {
			t.Errorf("scrollOff after ctrl+u = %d, want 0", m.scrollOff)
		}
	})
	t.Run("home and end keys", func(t *testing.T) {
		t.Parallel()
		m := newScrolled(t)
		last := len(m.scrollLines) - 1
		sendKey(m, keyNamed(tea.KeyEnd))
		if m.scrollOff != last {
			t.Errorf("end: scrollOff = %d, want %d", m.scrollOff, last)
		}
		sendKey(m, keyChar('G'))
		if m.scrollOff != last {
			t.Errorf("G: scrollOff = %d, want %d", m.scrollOff, last)
		}
		sendKey(m, keyNamed(tea.KeyHome))
		if m.scrollOff != 0 {
			t.Errorf("home: scrollOff = %d, want 0", m.scrollOff)
		}
	})
	t.Run("esc and q exit and clear", func(t *testing.T) {
		t.Parallel()
		for _, key := range []tea.KeyPressMsg{keyNamed(tea.KeyEscape), keyChar('q')} {
			m := newScrolled(t)
			sendKey(m, key)
			if m.panel != panelMain || m.scrollLines != nil || m.scrollRaw != "" || m.scrollOff != 0 {
				t.Errorf("%q left panel=%v lines=%v raw=%q off=%d, want panelMain and cleared scroll state",
					key.String(), m.panel, m.scrollLines != nil, m.scrollRaw, m.scrollOff)
			}
		}
	})
}

// TestUpdate_refreshMsgs covers every async message the dashboard
// consumes: status refresh (error and success with cursor clamp), the
// 5-second tick in its two modes, op completion, the log/doctor/validate
// loaders, host metrics delivery, and the editor reload error path.
func TestUpdate_refreshMsgs(t *testing.T) {
	t.Parallel()
	t.Run("status error surfaces in lastErr", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		sendMsg(m, statusRefreshedMsg{err: errors.New("ssh refused")})
		if m.loading {
			t.Error("a finished refresh must clear loading")
		}
		if m.lastErr != "ssh refused" {
			t.Errorf("lastErr = %q, want the refresh error", m.lastErr)
		}
		if m.statuses == nil {
			t.Error("a failed refresh must keep the previous statuses")
		}
	})
	t.Run("status success replaces statuses and clamps the cursor", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.cursor = 10 // past the end of the incoming list
		m.lastErr = "stale"
		sendMsg(m, statusRefreshedMsg{statuses: []runner.RunnerStatus{
			{Instance: "new-1", Host: "alpha"},
			{Instance: "new-2", Host: "bravo"},
		}})
		if m.lastErr != "" {
			t.Errorf("lastErr = %q, want cleared", m.lastErr)
		}
		if len(m.statuses) != 2 || m.statuses[0].Instance != "new-1" {
			t.Errorf("statuses not replaced: %v", m.statuses)
		}
		if m.cursor != 1 {
			t.Errorf("cursor = %d, want clamped to 1", m.cursor)
		}
	})
	t.Run("tick while busy re-arms without refreshing", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy = true
		cmd := sendMsg(m, tickMsg(time.Now()))
		if cmd == nil {
			t.Error("tick while busy must re-arm the ticker")
		}
		if m.loading {
			t.Error("tick while busy must not start a refresh")
		}
	})
	t.Run("tick on the main panel re-arms and refreshes", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		cmd := sendMsg(m, tickMsg(time.Now()))
		if cmd == nil {
			t.Error("tick must return a batched refresh+tick cmd")
		}
	})
	t.Run("tick outside the main panel only re-arms", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.panel = panelScroll
		cmd := sendMsg(m, tickMsg(time.Now()))
		if cmd == nil {
			t.Error("tick outside main must re-arm the ticker")
		}
	})
	t.Run("op success shows a toast and refreshes", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy, m.busyOp = true, "up"
		cmd := sendMsg(m, opDoneMsg{op: "up"})
		if m.busy || m.busyOp != "" {
			t.Errorf("busy=%v busyOp=%q, want cleared", m.busy, m.busyOp)
		}
		if m.toast != "up complete" {
			t.Errorf("toast = %q, want \"up complete\"", m.toast)
		}
		if !m.loading || cmd == nil {
			t.Error("op completion must refresh statuses")
		}
	})
	t.Run("op failure surfaces the error without a toast", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.toast = "stale"
		sendMsg(m, opDoneMsg{op: "up", err: errors.New("docker dead")})
		if m.lastErr != "docker dead" {
			t.Errorf("lastErr = %q, want the op error", m.lastErr)
		}
		if m.toast != "" {
			t.Errorf("toast = %q, want cleared on failure", m.toast)
		}
	})
	t.Run("logs open the scroll panel", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy, m.busyOp = true, "logs"
		sendMsg(m, logLoadedMsg{text: "hello\nworld"})
		if m.panel != panelScroll || m.scrollTitle != "Logs" {
			t.Errorf("panel=%v title=%q, want panelScroll/Logs", m.panel, m.scrollTitle)
		}
		if m.scrollRaw != "hello\nworld" {
			t.Errorf("scrollRaw = %q, want the log text", m.scrollRaw)
		}
	})
	t.Run("log errors keep the main panel", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy, m.busyOp = true, "logs"
		sendMsg(m, logLoadedMsg{err: errors.New("no such container")})
		if m.panel != panelMain {
			t.Errorf("panel = %v, want panelMain on log error", m.panel)
		}
		if m.lastErr != "no such container" {
			t.Errorf("lastErr = %q, want the log error", m.lastErr)
		}
	})
	t.Run("doctor output opens the scroll panel", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy = true
		sendMsg(m, doctorDoneMsg{out: "all good"})
		if m.panel != panelScroll || m.scrollTitle != "Doctor" || m.scrollRaw != "all good" {
			t.Errorf("panel=%v title=%q raw=%q, want panelScroll/Doctor/all good", m.panel, m.scrollTitle, m.scrollRaw)
		}
	})
	t.Run("validate results land in toast or lastErr", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy = true
		sendMsg(m, validateDoneMsg{})
		if m.toast != "Configuration is valid." || m.busy {
			t.Errorf("toast=%q busy=%v, want valid-toast and cleared busy", m.toast, m.busy)
		}
		m2 := updateTestModel(t)
		m2.busy = true
		sendMsg(m2, validateDoneMsg{err: errors.New("bad yaml")})
		if m2.lastErr != "bad yaml" {
			t.Errorf("lastErr = %q, want the validate error", m2.lastErr)
		}
	})
	t.Run("host metrics open their panel", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.metricsLoading = true
		sendMsg(m, hostMetricsMsg{metrics: []host.HostMetrics{{Name: "alpha"}}})
		if m.panel != panelHostMetrics || m.metricsLoading {
			t.Errorf("panel=%v loading=%v, want panelHostMetrics/false", m.panel, m.metricsLoading)
		}
		if len(m.hostMetrics) != 1 || m.hostMetricsCur != 0 {
			t.Errorf("metrics=%v cur=%d, want 1 metric at cursor 0", m.hostMetrics, m.hostMetricsCur)
		}
	})
	t.Run("editor failure prefixes the error", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.busy = true
		sendMsg(m, editorDoneMsg{err: errors.New("vi crashed")})
		if m.lastErr != "editor: vi crashed" {
			t.Errorf("lastErr = %q, want \"editor: vi crashed\"", m.lastErr)
		}
	})
}

// TestUpdate_windowSizeResize pins the resize contract: width/height are
// stored, and an open scroll panel re-wraps its cached raw text at the new
// width without re-fetching — while a closed scroll panel stays untouched.
func TestUpdate_windowSizeResize(t *testing.T) {
	t.Parallel()
	t.Run("scroll panel re-wraps on resize", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		m.width, m.height = 40, 24
		m.setScroll("Logs", strings.Repeat("x", 100))
		if got := len(m.scrollLines); got != 3 { // wrap width 36 → 36+36+28
			t.Fatalf("initial wrap = %d lines, want 3", got)
		}
		_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		if got := len(m.scrollLines); got != 2 { // wrap width 96 → 96+4
			t.Errorf("re-wrapped lines = %d, want 2", got)
		}
		if m.scrollRaw != strings.Repeat("x", 100) {
			t.Error("re-wrap must reuse the cached raw text")
		}
	})
	t.Run("main panel resize only stores the size", func(t *testing.T) {
		t.Parallel()
		m := updateTestModel(t)
		_, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		if m.width != 120 || m.height != 40 {
			t.Errorf("width=%d height=%d, want 120/40", m.width, m.height)
		}
		if m.scrollLines != nil {
			t.Error("resize must not fabricate scroll content")
		}
	})
}

// TestView_panels renders every panel and asserts on visible text (ANSI
// stripped) plus the AltScreen flag on every return path — a panel that
// forgets the flag renders into the main screen buffer, a regression the
// newAltView helper was introduced to prevent.
func TestView_panels(t *testing.T) {
	t.Parallel()
	build := func(t *testing.T, mutate func(m *dashboardModel)) *dashboardModel {
		t.Helper()
		m := updateTestModel(t)
		m.width, m.height = 200, 50 // wide enough that no column is dropped or truncated
		if mutate != nil {
			mutate(m)
		}
		return m
	}
	assert := func(t *testing.T, m *dashboardModel, want []string, notWant ...string) {
		t.Helper()
		v := m.View()
		if !v.AltScreen {
			t.Errorf("%v panel view lost the AltScreen flag", m.panel)
		}
		for _, s := range want {
			if !containsVisible(v.Content, s) {
				t.Errorf("%v panel view missing visible text %q", m.panel, s)
			}
		}
		for _, s := range notWant {
			if containsVisible(v.Content, s) {
				t.Errorf("%v panel view must not contain %q", m.panel, s)
			}
		}
	}
	t.Run("main loading", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.loading = true; m.statuses = nil })
		assert(t, m, []string{"Loading..."})
	})
	t.Run("main error", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.statuses = nil; m.lastErr = "ssh refused" })
		assert(t, m, []string{"Error: ssh refused"})
	})
	t.Run("main table", func(t *testing.T) {
		t.Parallel()
		m := build(t, nil)
		assert(t, m, []string{"gh sr dashboard", "2 runners", "INSTANCE", "HOST", "REPO", "runner-1", "runner-2"})
	})
	t.Run("main with filters", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) {
			m.statuses = nil
			m.tuiHostFilter = "alpha"
			m.tuiRepoFilter = "o/r1"
		})
		assert(t, m, []string{"host=alpha", "repo=o/r1"}, "(none)")
	})
	t.Run("main without filters", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.statuses = nil })
		assert(t, m, []string{"Filters: (none)"})
	})
	t.Run("main busy and toast", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.busy, m.busyOp, m.toast = true, "up", "up complete" })
		assert(t, m, []string{"up in progress", "up complete"})
	})
	t.Run("main help overlay", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.showHelp = true })
		assert(t, m, []string{"— Help —", "j/k navigate rows"})
	})
	t.Run("action menu marks only the selected item", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.panel = panelActionMenu; m.menuCursor = 1 })
		assert(t, m, []string{"Runner actions", "Instance: runner-1", "> Start (up)"}, "> Setup")
	})
	t.Run("global menu", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.panel = panelGlobalMenu })
		assert(t, m, []string{"Global menu", "> Doctor", "Filter by repo…"})
	})
	t.Run("filter menu", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.panel = panelFilterMenu })
		assert(t, m, []string{"Filters", "> Filter by host…", "Clear all filters"})
	})
	t.Run("filter pickers", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.panel = panelFilterHost; m.filterHostChoices = []string{"alpha", "bravo"} })
		assert(t, m, []string{"Pick filter", "> alpha", "bravo"})
		m2 := build(t, func(m *dashboardModel) { m.panel = panelFilterRepo; m.filterRepoChoices = nil })
		assert(t, m2, []string{"(no values)"})
	})
	t.Run("confirm panels", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) { m.panel = panelConfirmCleanup })
		assert(t, m, []string{"Confirm cleanup", "Remove offline self-hosted runners"})
		m2 := build(t, func(m *dashboardModel) { m.panel = panelConfirmRemove; m.confirmRemoveInst = "runner-2" })
		assert(t, m2, []string{"Confirm remove", "runner-2"})
	})
	t.Run("scroll panel window and hints", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) {
			m.width, m.height = 80, 12
			m.setScroll("Logs", strings.Repeat("line\n", 20))
			m.scrollOff = 2
		})
		v := m.View()
		if !containsVisible(v.Content, "Logs") {
			t.Error("scroll view missing its title")
		}
		if !containsVisible(v.Content, "lines 3–") {
			t.Errorf("scrolled scroll view missing the range hint, got: %q", v.Content)
		}
	})
	t.Run("host metrics panels", func(t *testing.T) {
		t.Parallel()
		m := build(t, func(m *dashboardModel) {
			m.panel = panelHostMetrics
			m.metricsLoading = true
			m.hostMetrics = nil
		})
		assert(t, m, []string{"Loading metrics..."})
		m2 := build(t, func(m *dashboardModel) { m.panel = panelHostMetrics })
		assert(t, m2, []string{"No hosts found."})
		m3 := build(t, func(m *dashboardModel) {
			m.panel = panelHostMetrics
			m.hostMetrics = []host.HostMetrics{{Name: "alpha", CPUPercent: 42.5, MemUsedMiB: 1024, MemTotalMiB: 4096, DiskUsedGiB: 10, DiskTotalGiB: 100, Load1: 1.5}}
		})
		assert(t, m3, []string{"Host Metrics", "1 host", "alpha", "r: refresh"})
	})
}
