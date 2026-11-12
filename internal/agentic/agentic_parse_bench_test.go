package agentic

import "testing"

// BenchmarkParseDockerChainOutput mirrors the docker-chain parser benchmark
// (the one that powers ValidateContainerPrereqs's docker CLI / daemon /
// privileged probe trio). It also switched from strings.Split to SplitSeq
// for the same allocation reason.
func BenchmarkParseDockerChainOutput(b *testing.B) {
	specs := dockerChainSpecs("privileged")
	b.Run("clean", func(b *testing.B) {
		in := "#docker-cli:0\n#docker-daemon:0\n#docker-privileged:0\n"
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = parseDockerChainOutput(in, specs)
		}
	})
	b.Run("cli_missing", func(b *testing.B) {
		in := "#docker-cli:127\n#docker-daemon:0\n#docker-privileged:0\n"
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = parseDockerChainOutput(in, specs)
		}
	})
}
