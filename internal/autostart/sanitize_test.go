package autostart

import "testing"

func TestSanitizeInstance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		// Basic valid inputs pass through unchanged
		{"ci-1", "ci-1", false},
		{"runner", "runner", false},
		{"UPPER", "UPPER", false},
		{"123", "123", false},
		// Special characters replaced with dashes
		{"my_runner-2", "my-runner-2", false},
		{"hello world", "hello-world", false},
		{"a@b", "a-b", false},
		// Consecutive special chars collapse to a single dash
		{"a___b", "a-b", false},
		{"a..b", "a-b", false},
		{"a__b__c", "a-b-c", false},
		// Leading/trailing special chars are stripped
		{"_runner_", "runner", false},
		{".ci.", "ci", false},
		// Error cases: all chars sanitize away to empty
		{"---", "", true},
		{"___", "", true},
		{"", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got, err := SanitizeInstance(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("SanitizeInstance(%q): expected error, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("SanitizeInstance(%q): unexpected error: %v", tc.input, err)
				return
			}
			if got != tc.want {
				t.Errorf("SanitizeInstance(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
