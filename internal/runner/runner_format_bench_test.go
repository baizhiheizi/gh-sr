package runner

import "testing"

// BenchmarkFormatContainerImageBuild measures the per-instance cost of the
// BUILD-cell formatter used by Manager.Status. Status is called once per TUI
// refresh tick (5s default) and once per instance within a host, so this is
// the per-cell hot path for the dashboard table.
func BenchmarkFormatContainerImageBuild(b *testing.B) {
	cases := []struct {
		name              string
		local, expected   string
		actual            string
	}{
		{"not installed", "not installed", "deadbeefcafebabe", "deadbeefcafebabe"},
		{"empty actual", "running", "deadbeef", ""},
		{"ok short", "running", "abc1234", "abc1234"},
		{"ok long", "running", "deadbeefcafebabe1234567890abcdef", "deadbeefcafebabe1234567890abcdef"},
		{"stale long", "running", "deadbeefcafebabe0000000000000000", "1234567890abcdef1234567890abcdef"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = formatContainerImageBuild(tc.local, tc.expected, tc.actual)
			}
		})
	}
}

// BenchmarkFormatContainerImageBuild_realisticMix exercises the realistic
// per-Status distribution across a 10-instance fleet: most instances report
// the matching-revision case, a few report the not-installed / empty-actual
// short-circuits. This is the shape Status() actually renders.
func BenchmarkFormatContainerImageBuild_realisticMix(b *testing.B) {
	cases := [][3]string{
		{"running", "deadbeefcafebabe1234567890abcdef", "deadbeefcafebabe1234567890abcdef"},
		{"running", "deadbeefcafebabe1234567890abcdef", "deadbeefcafebabe1234567890abcdef"},
		{"running", "1234567890abcdef0000000000000000", "1234567890abcdef0000000000000000"},
		{"not installed", "deadbeefcafebabe1234567890abcdef", "deadbeefcafebabe1234567890abcdef"},
		{"running", "deadbeefcafebabe1234567890abcdef", "deadbeefcafebabe1234567890abcdef"},
		{"stopped", "abcdef00000000000000000000000000", "abcdef00000000000000000000000000"},
		{"running", "deadbeefcafebabe1234567890abcdef", "deadbeefcafebabe1234567890abcdef"},
		{"running", "11112222333344445555666677778888", "99998888777766665555444433332222"},
		{"not installed", "deadbeef", ""},
		{"running", "deadbeefcafebabe1234567890abcdef", "deadbeefcafebabe1234567890abcdef"},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, c := range cases {
			_ = formatContainerImageBuild(c[0], c[1], c[2])
		}
	}
}