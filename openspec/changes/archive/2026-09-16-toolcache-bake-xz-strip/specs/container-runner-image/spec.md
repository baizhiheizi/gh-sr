# container-runner-image Specification

## Purpose
Define the declarative, build-time customization surface of the locally built `gh-sr/agentic-runner` image: extra Debian packages and tool-cache bake entries — including which archive formats bake supports, how extraction paths and "already installed" markers are derived, and how bake-list changes interact with the image fingerprint and rebuild flow.

## ADDED Requirements

### Requirement: Toolcache bake entries pre-extract tool tarballs at image build time

`container_runner_image.toolcache` entries SHALL be processed during image build: each entry downloads its `url` and extracts the archive into `/home/runner/.toolcache/<dir>`, then writes the "already installed" marker at the entry's `complete` path (defaulting to `<dir>.complete`) so setup actions treat the tool as installed and skip runtime downloads.

#### Scenario: Baked tool skips runtime download

- **WHEN** an image containing a bake entry for a tool runs a workflow whose setup action probes the toolcache root for that tool
- **THEN** the setup action MUST find the extracted tool and marker and complete without downloading the tool from the network

#### Scenario: Runtime install fallback preserved

- **WHEN** a tool is not covered by any bake entry
- **THEN** setup actions MUST still install it at job runtime into the tool cache, unchanged

### Requirement: Bake supports gzip and xz tarballs

The bake step SHALL extract both `tar.gz` and `tar.xz` archives, selecting the decompressor by archive extension.

#### Scenario: xz archive entry

- **WHEN** a bake entry URL ends in `.tar.xz` and the image builds
- **THEN** the archive MUST be extracted correctly into the entry's dir without a decompressor error

### Requirement: Optional strip-components per entry

Each bake entry SHALL accept an optional `strip` field (default 0) passed to `tar --strip-components`, so archives with a top-level wrapper directory can extract as a bare tool dir.

#### Scenario: Wrapped archive extracts as bare tool dir

- **WHEN** a bake entry sets `strip: 1` for an archive whose top level is a single versioned directory, and the entry's `dir` is the consumer's probe path
- **THEN** the extracted tree MUST place the archive's contents directly under `/home/runner/.toolcache/<dir>` with no intermediate wrapper directory

#### Scenario: Legacy three-field lines unchanged

- **WHEN** an existing runners.yml defines bake entries with only `url`, `dir`, and `complete` fields
- **THEN** validation MUST accept them and the bake MUST behave exactly as before (strip 0)

### Requirement: Bake list validation

Config validation SHALL reject bake lists with more than the supported entry count, non-HTTPS or empty URLs, URLs beyond the length cap, dirs that are empty/absolute/contain path traversal or whitespace, duplicate dirs, and mismatched marker paths, with errors naming the offending entry index.

#### Scenario: Traversal attempt rejected

- **WHEN** a bake entry declares `dir: ../../etc`
- **THEN** config validation MUST fail with an error identifying the entry

### Requirement: Bake list changes fold into the image fingerprint

The container image fingerprint / layout revision SHALL incorporate the bake list so that adding, removing, or editing an entry marks existing deployments stale and prompts `gh sr rebuild`.

#### Scenario: Edited bake entry triggers rebuild prompt

- **WHEN** a runners.yml bake entry changes and `gh sr status` runs against a host running the previously built image
- **THEN** the BUILD column MUST report `stale` until the image is rebuilt
