## ADDED Requirements

### Requirement: Column widths are computed from a single helper

The TUI package SHALL provide `computeColumnWidths(headers []string, rows [][]string) []int` that returns the maximum rendered width per column (header length vs cell lengths). All table views in `internal/tui` MUST use this helper instead of inline width loops.

#### Scenario: Host metrics table uses shared width helper

- **WHEN** `PrintHostMetricsTable` or `FormatHostMetricsTo` renders a metrics table
- **THEN** column widths MUST be computed via `computeColumnWidths`

#### Scenario: Runner status table uses shared width helper

- **WHEN** `PrintStatusTable` or the dashboard runner list renders column widths
- **THEN** column widths MUST be computed via `computeColumnWidths` (directly or via rows built from status cells)

#### Scenario: Dashboard host-metrics panel uses shared width helper

- **WHEN** the dashboard host-metrics overlay renders its table
- **THEN** column widths MUST be computed via `computeColumnWidths`

### Requirement: Runner status row cells are derived from a single helper

The TUI package SHALL provide `runnerStatusCells(s runner.RunnerStatus) []string` returning nine cells in fixed order: Instance, Host, Repo, Mode, Image, Build, Local, GitHub status, Labels. Empty ContainerImage and ContainerImageBuild MUST be displayed as `"-"`. GitHub status MUST use the existing `formatGitHubStatus` logic.

#### Scenario: CLI status table matches dashboard cells

- **WHEN** the same `RunnerStatus` value is rendered via `PrintStatusTable` and the dashboard runner list
- **THEN** the nine cell values MUST be identical

#### Scenario: Empty image fields show dash

- **WHEN** a runner has empty `ContainerImage` or `ContainerImageBuild`
- **THEN** the corresponding table cell MUST display `"-"`

### Requirement: Dashboard tables are responsive to the terminal width

The dashboard runner table and host-metrics panel SHALL compute their column layout from the current terminal width (`tea.WindowSizeMsg`, defaulting to 80 before the first size event): columns truncate with an ellipsis down to per-column floors first, and are dropped (widest-first by configured priority) only when the floored widths still overflow. The rendered row width — content plus per-cell padding — MUST NOT exceed the terminal width for any terminal 30 columns or wider, as long as at least one column remains: a table that cannot fit MUST keep shedding columns rather than render wider than the terminal and hard-wrap into an unreadable mess.

#### Scenario: Narrow terminal drops optional columns

- **WHEN** the terminal is too narrow for all columns even at their truncation floors
- **THEN** the runner table MUST drop columns in the order LABELS, IMAGE, BUILD before shedding any status column (INSTANCE/HOST/REPO/MODE/LOCAL/GITHUB)

#### Scenario: A pinned-only remainder still fits

- **WHEN** every droppable column has already been removed and the remaining columns are all pinned (status or percentage cells)
- **THEN** the table MUST shed the remainder via a lower-priority emergency drop tier — REPO, MODE, HOST, INSTANCE for the runner table; LOAD AVG, DISK, MEMORY, HOST for host metrics — leaving the state signals (LOCAL/GITHUB, CPU) in place, because dropping preserves a column's status color whereas truncating strips it

#### Scenario: Cells truncate as plain text before styling

- **WHEN** a cell is truncated to its column width
- **THEN** truncation MUST happen on the unstyled cell text and append a single ellipsis, so ANSI escape sequences are never cut mid-sequence

#### Scenario: Status cells are never truncated

- **WHEN** the table is narrowed
- **THEN** MODE/BUILD/LOCAL/GITHUB (runner table) and CPU/MEMORY/DISK (host metrics) MUST keep their natural width, because truncating their text would also strip the status color

#### Scenario: CLI tables keep every column

- **WHEN** `gh sr status` or `gh sr host-metrics` prints to stdout
- **THEN** the table MUST render all columns at natural width (responsiveness applies to the interactive dashboard only)

### Requirement: Dashboard prose lines fit the terminal width

The footer key hints, the expanded help overlay, error messages, filter lines, empty-state notices, and the panel title count SHALL wrap (hints by segment packing, prose by word wrapping) so no line exceeds the terminal width. The scroll views (logs, doctor, config) SHALL re-wrap their content when the terminal is resized.

#### Scenario: Footer hints wrap on narrow terminals

- **WHEN** the footer hint segments do not fit one line
- **THEN** they MUST wrap onto additional lines rather than overflow, with the loading indicator remaining the only difference between the idle and loading footers

#### Scenario: A word wider than the line is broken

- **WHEN** a wrapped line contains a single token wider than the available width (a repo slug, hostname, URL, or file path carries no break opportunity)
- **THEN** it MUST be broken across lines on a rune boundary rather than emitted verbatim and overflowing

#### Scenario: The panel title count stays on the title line

- **WHEN** the panel title and its `<n> runners` / `<n> hosts` suffix fit on one line
- **THEN** the suffix MUST render right-aligned on that line — neither half may carry a margin, since a margin inserts a newline and pushes the suffix onto its own line — and MUST be dropped rather than wrapped when the two cannot share a line

#### Scenario: Counts are pluralised

- **WHEN** a panel renders its row count
- **THEN** the count MUST be pluralised (`1 runner`, `0 runners`)
