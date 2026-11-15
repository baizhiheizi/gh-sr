// Package session owns the process-wide wiring every gh-sr entry point
// shares: bootstrap the env file, resolve the config path, load the config,
// resolve the GitHub token, and build the clients derived from them. Every
// entry point — cobra commands and the TUI dashboard — loads one Session and
// asks it for a Manager, a GitHub client, or a doctor input, so the error
// policy lives here instead of drifting per call site.
//
// The policy, in one place:
//
//   - Bootstrap and path-resolution failures are fatal: without them nothing
//     downstream can run. Load returns them.
//   - A config-load failure is recorded (ConfigErr) rather than fatal:
//     doctor must still run to report it as a FAIL row; commands that cannot
//     proceed without a config surface it themselves.
//   - A token failure is recorded (TokenErr) rather than fatal: doctor
//     reports "no token" and degrades its API-backed checks; GitHub() then
//     returns nil, while Manager() — whose callers cannot operate without
//     credentials — surfaces the error.
package session

import (
	"fmt"
	"io"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/doctor"
	"github.com/an-lee/gh-sr/internal/ops"
	"github.com/an-lee/gh-sr/internal/runner"
)

// Session is the resolved wiring of one gh-sr process: paths, loaded config,
// and the lazily-resolved GitHub token with its derived clients.
type Session struct {
	// ConfigPath is the resolved runners.yml path; EnvPath the resolved
	// ~/.gh-sr/env path. Both survive for commands that re-read or edit them
	// (doctor's existence checks, the TUI's edit-then-reload cycle).
	ConfigPath string
	EnvPath    string
	// Config is the loaded config (nil when loading failed). ConfigErr is the
	// load error, kept for callers that degrade gracefully instead of
	// aborting. The pair mirrors config.LoadFromPath's contract.
	Config    *config.Config
	ConfigErr error
	// GhSrVersion is the CLI version stamp (Makefile -X main.version) the
	// Manager needs for container image revision and Docker image labels.
	GhSrVersion string

	token    string
	TokenErr error
}

// Load bootstraps the env file and resolves the session's paths and config.
// cfgFile is the --config flag value ("" = env override or ~/.gh-sr/runners.yml).
// ghSrVersion is the CLI version stamp for the derived Manager. Fatal-path
// errors (bootstrap, path resolution) are returned; a config-load failure is
// recorded on the Session for graceful degradation.
func Load(cfgFile, ghSrVersion string) (*Session, error) {
	if err := config.BootstrapEnv(); err != nil {
		return nil, err
	}
	cfgPath, err := config.ResolveConfigPath(cfgFile)
	if err != nil {
		return nil, err
	}
	envPath, err := config.EnvFilePath()
	if err != nil {
		return nil, err
	}
	s := &Session{
		ConfigPath:  cfgPath,
		EnvPath:     envPath,
		GhSrVersion: ghSrVersion,
	}
	s.Config, s.ConfigErr = config.LoadFromPath(cfgPath)
	return s, nil
}

// Token resolves and memoizes the GitHub token for the process lifetime.
// The first error sticks: a mid-session `gh auth logout` is not re-probed.
func (s *Session) Token() (string, error) {
	if s.token != "" || s.TokenErr != nil {
		return s.token, s.TokenErr
	}
	s.token, s.TokenErr = config.ResolveToken(s.Config)
	return s.token, s.TokenErr
}

// GitHub returns the GitHub API client, or nil when no token can be
// resolved. Callers that degrade without credentials (doctor's API-backed
// checks) rely on the nil; callers that cannot proceed should use Manager
// and surface its error instead.
func (s *Session) GitHub() *runner.GitHubClient {
	tok, err := s.Token()
	if err != nil {
		return nil
	}
	return runner.NewGitHubClient(tok)
}

// Manager builds the runner Manager wired to w. It errors iff the token
// cannot be resolved — runner operations are load-bearing on credentials —
// and works from whatever config loaded (nil config yields a Manager with
// zero derived fields, the native-only and unit-test shape).
func (s *Session) Manager(w io.Writer) (*runner.Manager, error) {
	tok, err := s.Token()
	if err != nil {
		return nil, err
	}
	m := runner.NewManagerFromConfig(tok, s.Config)
	m.Out = w
	m.GhSrVersion = s.GhSrVersion
	return m, nil
}

// DoctorInput assembles doctor.Run's input from the session. Every caller
// goes through here — no call site re-assembles the individual fields by
// hand, which is what let three entry points drift into three different
// token-error policies.
func (s *Session) DoctorInput(filterHost, filterRepo string, strict, checkLockfiles bool) doctor.RunInput {
	return doctor.RunInput{
		ConfigPath:     s.ConfigPath,
		EnvPath:        s.EnvPath,
		Config:         s.Config,
		ConfigErr:      s.ConfigErr,
		GitHub:         s.GitHub(),
		FilterHost:     filterHost,
		FilterRepo:     filterRepo,
		Strict:         strict,
		CheckLockfiles: checkLockfiles,
	}
}

// DoctorFix runs doctor; when fix is set and checks failed with a loadable
// config, runs `gh sr setup` to attempt fixes, re-runs doctor, and returns
// the process exit code. The loop itself is doctorFixLoop (unit tested with
// injected steps); this wiring is the only untested shell.
func DoctorFix(w io.Writer, s *Session, fix bool, filterHost, filterRepo string, strict, checkLockfiles bool) int {
	return doctorFixLoop(w, strict, s.ConfigErr == nil, fix,
		func() doctor.Result {
			return doctor.Run(w, s.DoctorInput(filterHost, filterRepo, strict, checkLockfiles))
		},
		func() error {
			mgr, err := s.Manager(w)
			if err != nil {
				return fmt.Errorf("cannot create runner manager: %w", err)
			}
			return ops.Setup(w, s.Config, mgr, filterHost, filterRepo, nil)
		},
	)
}

// doctorFixLoop is the doctor --fix state machine, with doctor.Run and
// ops.Setup injected as steps so the exit-code contract is unit testable:
// run doctor → on failures with a loadable config, attempt the fix → re-run
// doctor → exit non-zero if the re-run still fails (or strict is set).
// A runSetup error reports as a failed fix attempt; the loop never aborts
// on it — the re-run decides.
func doctorFixLoop(w io.Writer, strict, cfgReady, fix bool, runDoctor func() doctor.Result, runSetup func() error) int {
	res := runDoctor()
	if fix && res.Fail > 0 && cfgReady {
		fmt.Fprintln(w, "\n--- Running `gh sr setup` to attempt fixes ---")
		if err := runSetup(); err != nil {
			fmt.Fprintf(w, "\nfix attempt completed with errors: %v\n", err)
		} else {
			fmt.Fprintln(w, "\nfix attempt completed successfully.")
		}
		fmt.Fprintln(w, "\n--- Re-running doctor after fix ---")
		res = runDoctor()
	}
	return doctor.ExitCode(res, strict)
}
