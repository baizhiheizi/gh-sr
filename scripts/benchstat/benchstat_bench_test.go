package benchstat

import (
	"os"
	"testing"
)

// realisticBenchTxt mimics a `go test -bench=. -benchmem -count=1` snippet from
// the gh-sr repo (6 benchmarks × 1 package). Used by the ParseFile / full
// pipeline benchmarks below.
const realisticBenchTxt = `goos: linux
goarch: amd64
pkg: github.com/an-lee/gh-sr/internal/ops
cpu: AMD Ryzen AI 9 HX 370
BenchmarkLoad_Small-24         	   10000	     23456 ns/op	    4096 B/op	      56 allocs/op
BenchmarkLoad_Medium-24        	    2000	     78901 ns/op	   16384 B/op	     213 allocs/op
BenchmarkLoad_Large-24         	     200	   3123456 ns/op	  234567 B/op	    3145 allocs/op
BenchmarkSave_Small-24         	   20000	     12345 ns/op	    2048 B/op	      28 allocs/op
BenchmarkSave_Medium-24        	    5000	     45678 ns/op	    8192 B/op	      96 allocs/op
BenchmarkSave_Large-24         	     500	    876543 ns/op	   65536 B/op	     780 allocs/op
PASS
ok  	github.com/an-lee/gh-sr/internal/ops	12.345s
`

const realisticBenchTxtHead = `goos: linux
goarch: amd64
pkg: github.com/an-lee/gh-sr/internal/ops
cpu: AMD Ryzen AI 9 HX 370
BenchmarkLoad_Small-24         	   10000	     24500 ns/op	    4200 B/op	      58 allocs/op
BenchmarkLoad_Medium-24        	    2000	     81000 ns/op	   16500 B/op	     220 allocs/op
BenchmarkLoad_Large-24         	     200	   3250000 ns/op	  240000 B/op	    3200 allocs/op
BenchmarkSave_Small-24         	   20000	     11900 ns/op	    2050 B/op	      27 allocs/op
BenchmarkSave_Medium-24        	    5000	     46000 ns/op	    8200 B/op	      95 allocs/op
BenchmarkSave_Large-24         	     500	    910000 ns/op	   68000 B/op	     820 allocs/op
BenchmarkEnrichWithGitHubStatus_Small-24        	    3000	     12000 ns/op	    4096 B/op	      28 allocs/op
BenchmarkEnrichWithGitHubStatus_Medium-24       	     500	    65000 ns/op	   16384 B/op	     120 allocs/op
BenchmarkFormatBytesHuman-24    	   50000	      450 ns/op	      32 B/op	       8 allocs/op
PASS
ok  	github.com/an-lee/gh-sr/internal/ops	12.345s
`

func writeTempFile(tb testing.TB, body string) string {
	tb.Helper()
	dir := tb.TempDir()
	path := dir + "/bench.txt"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		tb.Fatalf("write %s: %v", path, err)
	}
	return path
}

// BenchmarkParseFile measures cold-path parsing of a realistic bench output.
func BenchmarkParseFile(b *testing.B) {
	path := writeTempFile(b, realisticBenchTxt)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ParseFile(path)
	}
}

// BenchmarkRenderMarkdown measures the markdown-rendering step that runs on
// every PR via .github/workflows/bench-compare.yml. Setup (parsing + comparing)
// is excluded via ResetTimer; only the renderer is measured.
func BenchmarkRenderMarkdown(b *testing.B) {
	basePath := writeTempFile(b, realisticBenchTxt)
	headPath := writeTempFile(b, realisticBenchTxtHead)
	base, err := ParseFile(basePath)
	if err != nil {
		b.Fatal(err)
	}
	head, err := ParseFile(headPath)
	if err != nil {
		b.Fatal(err)
	}
	rows := Compare(base, head, DefaultThresholds())
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = RenderMarkdown(rows, "main", "PR")
	}
}

// BenchmarkFullPipeline exercises parse → compare → render end-to-end — the
// same shape the bench-compare CI workflow runs on every PR.
func BenchmarkFullPipeline(b *testing.B) {
	basePath := writeTempFile(b, realisticBenchTxt)
	headPath := writeTempFile(b, realisticBenchTxtHead)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		base, err := ParseFile(basePath)
		if err != nil {
			b.Fatal(err)
		}
		head, err := ParseFile(headPath)
		if err != nil {
			b.Fatal(err)
		}
		rows := Compare(base, head, DefaultThresholds())
		_ = RenderMarkdown(rows, "main", "PR")
	}
}
