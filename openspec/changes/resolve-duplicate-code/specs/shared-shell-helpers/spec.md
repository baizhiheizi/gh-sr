## ADDED Requirements

### Requirement: Plist XML escaping is centralized

The codebase SHALL provide a single `hostshell.PlistEscape` function for XML/plist string escaping (`&`, `"`, `<`, `>`). All launchd plist generation in `internal/autostart` and `internal/diskschedule` MUST call this function instead of local duplicates.

#### Scenario: Autostart plist generation uses shared escape

- **WHEN** `autostart.LaunchdPlist` embeds user-controlled paths in plist XML
- **THEN** it MUST escape values via `hostshell.PlistEscape`

#### Scenario: Disk schedule plist generation uses shared escape

- **WHEN** `diskschedule` generates a LaunchAgent plist for disk prune
- **THEN** it MUST escape values via `hostshell.PlistEscape`

### Requirement: Launchd bootout script is not duplicated

The launchd bootout/unload shell script MUST be defined once in `internal/autostart` as an exported `LaunchdBootoutScript` function. Other packages MUST NOT inline an equivalent script template.

#### Scenario: Disk schedule uninstall uses shared bootout script

- **WHEN** `diskschedule.uninstallLaunchd` removes the disk-prune LaunchAgent
- **THEN** it MUST execute the script produced by `autostart.LaunchdBootoutScript` with the label passed through `hostshell.PosixSingleQuote`

### Requirement: POSIX disk scripts use a consistent strict-mode header

All POSIX shell scripts generated in `internal/runner/disk.go` that reference a runner instance directory MUST begin with `set -e` followed by the `posixRunnerDirVar` assignment, via a shared `posixScriptHeader` helper.

#### Scenario: Dir-size script includes strict header

- **WHEN** `buildDirSizesPOSIXScript` is rendered for an instance
- **THEN** the script MUST start with `set -e` and the runner dir variable assignment

#### Scenario: Clear and remove scripts match dir-size header

- **WHEN** `clearWorkTempPOSIX` or `removeDirTreePOSIX` scripts are rendered
- **THEN** they MUST use the same header helper as the dir-size script
