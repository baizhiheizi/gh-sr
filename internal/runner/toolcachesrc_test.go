package runner

import (
	"strconv"
	"strings"
	"testing"

	"github.com/an-lee/gh-sr/internal/config"
	"github.com/an-lee/gh-sr/internal/host"
	"github.com/an-lee/gh-sr/internal/testutil"
)

func newToolcacheHost(t *testing.T, mock *testutil.MockExecutor) *host.Host {
	t.Helper()
	h := host.NewHost("h", config.HostConfig{Addr: "runner@vps", OS: "linux", Arch: "amd64"})
	h.SetConn(mock)
	return h
}

// TestContainerToolcacheExtraFileWithBlobs pins the build-context manifest
// contract: a cached source appends exactly one fifth column (the blob name)
// with every column to its left materialized (unset complete pads to "-",
// strip prints 0) so the bake loop's positional read cannot shift fields;
// entries without a cache keep the padded 4-column line, and with no blobs at
// all the output is byte-identical to containerToolcacheExtraFile (the bake
// must be reproducible regardless of host cache state).
func TestContainerToolcacheExtraFileWithBlobs(t *testing.T) {
	t.Parallel()
	entries := []config.ContainerToolcacheEntry{
		{URL: "https://example.com/flutter.tar.xz", Dir: "flutter"},
		{URL: "https://example.com/jdk.tar.gz", Dir: "jdk-temurin-17", Strip: 1},
	}
	cached := containerToolcacheExtraFile(entries)

	if got := containerToolcacheExtraFileWithBlobs(entries, nil); got != cached {
		t.Fatalf("no blobs must render byte-identically to the 4-column form:\ngot  %q\nwant %q", got, cached)
	}

	keyFlutter := toolcacheSourceKey("https://example.com/flutter.tar.xz")
	blobs := map[int]string{0: keyFlutter}
	got := containerToolcacheExtraFileWithBlobs(entries, blobs)
	if !strings.Contains(got, "https://example.com/flutter.tar.xz flutter - 0 "+keyFlutter+"\n") {
		t.Fatalf("cached entry must carry the blob name as its fifth column with complete/strip padded, got:\n%q", got)
	}
	if !strings.Contains(got, "https://example.com/jdk.tar.gz jdk-temurin-17 - 1\n") {
		t.Fatalf("uncached entry must keep the padded 4-column line, got:\n%q", got)
	}
	if strings.Count(got, keyFlutter) != 1 {
		t.Fatalf("blob name must appear exactly once, got:\n%q", got)
	}
}

// TestContainerToolcacheExtraFileColumnsAlign is the regression test for the
// 2026-10-18 x1-linux build failure: a cached blob key appended without
// left-padding shifted into the strip column, so the bake ran
// `tar --strip-components=fdf5a2a9ccdd284a` (GNU tar: "Invalid number of
// elements") while bypassing the blob and re-downloading the tarball from
// github.com — 8 connect timeouts before the tar error. This test re-parses
// every rendered line with the bake loop's exact positional semantics
// (`read -r url dir complete strip local` plus its "-" / empty defaults) and
// asserts each field lands where the Dockerfile expects, across every
// complete × strip × blob combination.
func TestContainerToolcacheExtraFileColumnsAlign(t *testing.T) {
	t.Parallel()
	type shape struct {
		name     string
		entry    config.ContainerToolcacheEntry
		cached   bool
		wantBlob bool // entry is expected to consume a context blob
	}
	shapes := []shape{
		{"bare", config.ContainerToolcacheEntry{URL: "https://e.com/a.tar.gz", Dir: "a"}, false, false},
		{"bare cached", config.ContainerToolcacheEntry{URL: "https://e.com/b.tar.gz", Dir: "b"}, true, true},
		{"complete only", config.ContainerToolcacheEntry{URL: "https://e.com/c.tar.gz", Dir: "c", Complete: "c/x.complete"}, false, false},
		{"complete only cached", config.ContainerToolcacheEntry{URL: "https://e.com/d.tar.gz", Dir: "d", Complete: "d/x.complete"}, true, true},
		{"strip only", config.ContainerToolcacheEntry{URL: "https://e.com/e.tar.gz", Dir: "e", Strip: 1}, false, false},
		{"strip only cached", config.ContainerToolcacheEntry{URL: "https://e.com/f.tar.gz", Dir: "f", Strip: 1}, true, true},
		{"complete+strip", config.ContainerToolcacheEntry{URL: "https://e.com/g.tar.gz", Dir: "g", Complete: "g.complete", Strip: 2}, false, false},
		{"complete+strip cached", config.ContainerToolcacheEntry{URL: "https://e.com/h.tar.gz", Dir: "h", Complete: "h.complete", Strip: 2}, true, true},
	}
	entries := make([]config.ContainerToolcacheEntry, len(shapes))
	blobs := make(map[int]string)
	for i, s := range shapes {
		entries[i] = s.entry
		if s.cached {
			blobs[i] = toolcacheSourceKey(s.entry.URL)
		}
	}
	rendered := containerToolcacheExtraFileWithBlobs(entries, blobs)

	byDir := make(map[string]shape)
	for _, s := range shapes {
		byDir[s.entry.Dir] = s
	}
	for _, line := range strings.Split(strings.TrimSuffix(rendered, "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields) > 5 {
			t.Fatalf("line must have 2–5 columns, got %d: %q", len(fields), line)
		}
		s := byDir[fields[1]]
		if s.name == "" {
			t.Fatalf("unexpected dir column in %q", line)
		}
		// read -r: trailing variables are set to empty strings.
		var url, dir, complete, strip, local string
		url, dir = fields[0], fields[1]
		if len(fields) > 2 {
			complete = fields[2]
		}
		if len(fields) > 3 {
			strip = fields[3]
		}
		if len(fields) > 4 {
			local = fields[4]
		}
		if url != s.entry.URL {
			t.Fatalf("%s: url column shifted: %q", s.name, line)
		}
		// Bake-loop complete semantics: empty or "-" → "<dir>.complete".
		if complete == "" || complete == "-" {
			complete = dir + ".complete"
		}
		if want := s.entry.Complete; want != "" && complete != want {
			t.Fatalf("%s: complete column landed as %q, want %q (line %q)", s.name, complete, want, line)
		}
		// Bake-loop strip semantics: empty → 0; anything else must be
		// numeric (a hex blob key here previously became
		// --strip-components=<16 hex chars>).
		if strip == "" {
			strip = "0"
		}
		if want := strconv.Itoa(s.entry.Strip); strip != want {
			t.Fatalf("%s: strip column landed as %q, want %q (line %q)", s.name, strip, want, line)
		}
		for _, r := range strip {
			if r < '0' || r > '9' {
				t.Fatalf("%s: strip column must be numeric, got %q (line %q)", s.name, strip, line)
			}
		}
		// Bake-loop local semantics: empty → "-" (WAN fallback); a cached
		// entry must reference its own blob, not another column's value.
		if local == "" {
			local = "-"
		}
		if s.wantBlob && local != toolcacheSourceKey(s.entry.URL) {
			t.Fatalf("%s: cached entry must name its blob in column 5, got %q (line %q)", s.name, local, line)
		}
		if !s.wantBlob && local != "-" {
			t.Fatalf("%s: uncached entry must not reference a blob, got %q (line %q)", s.name, local, line)
		}
	}
}

// TestSyncToolcacheSources pins the pre-build source-cache script: one SSH
// round-trip that skips already-cached blobs, downloads missing ones with the
// bake's retry policy, renames atomically, links blobs into the build
// context, and reports per-entry CACHED/FETCHED keys the caller maps back to
// entry indices. A MISS (failed download) must not produce a blob — the
// Dockerfile then falls back to its own curl.
func TestSyncToolcacheSources(t *testing.T) {
	t.Parallel()
	entries := []config.ContainerToolcacheEntry{
		{URL: "https://example.com/flutter.tar.xz", Dir: "flutter"},
		{URL: "https://example.com/jdk.tar.gz", Dir: "jdk-temurin-17"},
		{URL: "https://example.com/broken.tar.gz", Dir: "broken"},
	}
	var script string
	mock := &testutil.MockExecutor{RunFn: func(cmd string) (string, error) {
		script = cmd
		// flutter already cached, jdk freshly fetched, broken failed.
		return "READY\n" +
			"CACHED " + toolcacheSourceKey("https://example.com/flutter.tar.xz") + "\n" +
			"FETCHED " + toolcacheSourceKey("https://example.com/jdk.tar.gz") + "\n" +
			"MISS " + toolcacheSourceKey("https://example.com/broken.tar.gz") + "\n", nil
	}}
	blobs := syncToolcacheSources(newToolcacheHost(t, mock), entries)

	if len(blobs) != 2 || blobs[0] == "" || blobs[1] == "" || blobs[2] != "" {
		t.Fatalf("got blobs %v, want entries 0 and 1 present, 2 absent", blobs)
	}
	for _, want := range []string{
		// The cache dir must be DOUBLE-quoted: single quotes would make the
		// shell create a literal "$HOME" directory instead of expanding it.
		`mkdir -p "` + toolcacheSrcDir + `" /tmp/gh-sr-agentic-runner-build/toolcache-blobs`,
		": > /tmp/gh-sr-agentic-runner-build/toolcache-blobs/.keep",
		`if [ -s "$HOME/.gh-sr/toolcache-src/` + toolcacheSourceKey("https://example.com/flutter.tar.xz") + `" ]`,
		"curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 1800 'https://example.com/jdk.tar.gz'",
		// Atomic rename: a partial download must never look complete.
		".tmp",
		"mv -f ",
		// Context link: hardlink preferred, copy fallback across filesystems.
		"cp -l ",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q:\n%s", want, script)
		}
	}
	// Guard the regression directly: no cache path may be single-quoted, which
	// would park the whole cache in a directory literally named "$HOME".
	if strings.Contains(script, "'"+toolcacheSrcDir) || strings.Contains(script, "'$HOME/.gh-sr/toolcache-src/") {
		t.Errorf("cache paths must be double-quoted so $HOME expands:\n%s", script)
	}
}

// TestSyncToolcacheSourcesEmptyList pins the no-bake contract: the context's
// toolcache-blobs/ dir (with .keep) must still be prepared, because the
// Dockerfile COPYs it unconditionally and COPY fails on a missing source.
func TestSyncToolcacheSourcesEmptyList(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{}
	blobs := syncToolcacheSources(newToolcacheHost(t, mock), nil)
	if len(blobs) != 0 {
		t.Fatalf("got %v, want empty", blobs)
	}
	if len(mock.Calls) != 1 {
		t.Fatalf("expected exactly one command, got %v", mock.Calls)
	}
	if got := mock.Calls[0]; !strings.Contains(got, "mkdir -p /tmp/gh-sr-agentic-runner-build/toolcache-blobs") ||
		!strings.Contains(got, ": > /tmp/gh-sr-agentic-runner-build/toolcache-blobs/.keep") {
		t.Fatalf("empty list must still prepare the blobs dir, got %q", got)
	}
}

// TestSyncToolcacheSourcesRunError pins the best-effort contract: an SSH
// failure degrades to "no blobs" (the Dockerfile curls) instead of failing
// the build.
func TestSyncToolcacheSourcesRunError(t *testing.T) {
	t.Parallel()
	mock := &testutil.MockExecutor{RunFn: func(string) (string, error) {
		return "", assertCalledError()
	}}
	blobs := syncToolcacheSources(newToolcacheHost(t, mock), []config.ContainerToolcacheEntry{
		{URL: "https://example.com/x.tar.gz", Dir: "x"},
	})
	if len(blobs) != 0 {
		t.Fatalf("SSH failure must yield no blobs, got %v", blobs)
	}
}

// TestAgenticRunnerDockerfileToolcacheBakeLocalBlobs pins the bake's
// blob-preference contract in the Dockerfile: the blobs dir is COPYed before
// the manifest, a present fifth-field blob is copied instead of downloaded,
// and the curl fallback (with its full retry/timeout policy) stays for
// everything else.
func TestAgenticRunnerDockerfileToolcacheBakeLocalBlobs(t *testing.T) {
	t.Parallel()
	for _, want := range []string{
		"COPY toolcache-blobs/ /tmp/toolcache-blobs/",
		`[ -n "$local" ] || local="-"`,
		`if [ "$local" != "-" ] && [ -s "/tmp/toolcache-blobs/$local" ]; then`,
		`cp "/tmp/toolcache-blobs/$local" /tmp/toolcache.tar;`,
		`curl -fsSL --retry 8 --retry-all-errors --retry-delay 20 --connect-timeout 20 --speed-limit 10240 --speed-time 60 --max-time 1800 "$url" -o /tmp/toolcache.tar;`,
		`while read -r url dir complete strip local; do`,
	} {
		if !strings.Contains(agenticRunnerDockerfile, want) {
			t.Fatalf("Dockerfile bake missing %q in:\n%s", want, agenticRunnerDockerfile)
		}
	}
	// The blobs COPY must precede the bake RUN so the loop can read them.
	copyIdx := strings.Index(agenticRunnerDockerfile, "COPY toolcache-blobs/")
	runIdx := strings.Index(agenticRunnerDockerfile, "while read -r url dir complete strip local")
	if copyIdx < 0 || runIdx < 0 || copyIdx > runIdx {
		t.Fatalf("toolcache-blobs COPY must precede the bake loop (copy=%d, loop=%d)", copyIdx, runIdx)
	}
}

// TestAgenticRunnerDockerfileToolcacheBakeDoesNotShipBlobs guards the image
// size of the host-side source cache. COPY lands the (GBs-scale) source
// tarballs into a persisted layer, and deleting them in a later layer cannot
// reclaim layer bytes — a single-stage bake would ship a second full copy of
// every tarball next to the extracted tool cache. The bake must therefore run
// in a throwaway stage that the final image only inherits the extracted
// /home/runner/.toolcache from.
func TestAgenticRunnerDockerfileToolcacheBakeDoesNotShipBlobs(t *testing.T) {
	t.Parallel()

	// The final stage must pull the tool cache across from the bake stage.
	if !strings.Contains(agenticRunnerDockerfile,
		"COPY --from=toolcache-bake /home/runner/.toolcache/ /home/runner/.toolcache/") {
		t.Fatalf("final image must carry the baked tool cache across from the bake stage, got:\n%s", agenticRunnerDockerfile)
	}
	if !strings.Contains(agenticRunnerDockerfile, "FROM base-with-tools AS toolcache-bake") {
		t.Fatalf("the bake (and its tarballs) must live in a throwaway stage, got:\n%s", agenticRunnerDockerfile)
	}

	// Everything from the final FROM onwards is what ships: the blobs COPY must
	// not appear there, or the tarballs come back into the image.
	finalFrom := strings.Index(agenticRunnerDockerfile, "\nFROM base-with-tools\n")
	if finalFrom < 0 {
		t.Fatalf("final image stage not found, got:\n%s", agenticRunnerDockerfile)
	}
	final := agenticRunnerDockerfile[finalFrom:]
	if strings.Contains(final, "COPY toolcache-blobs/") || strings.Contains(final, "/tmp/toolcache-blobs/") {
		t.Errorf("the shipped stage must not COPY or reference the source tarballs, got:\n%s", final)
	}

	// The bake stage must build its own tool-cache root: COPY --from fails on a
	// missing source, and an empty manifest never creates one.
	bakeStage := agenticRunnerDockerfile[strings.Index(agenticRunnerDockerfile, "FROM base-with-tools AS toolcache-bake"):finalFrom]
	if !strings.Contains(bakeStage, "mkdir -p /home/runner/.toolcache") {
		t.Errorf("bake stage must create the tool-cache root before the loop, got:\n%s", bakeStage)
	}
}
