package installation

import "testing"

func TestIsSHA(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid 40-char lowercase hex", "abc123def456789012345678901234567890abcd", true},
		{"all zeros", "0000000000000000000000000000000000000000", true},
		{"all f's", "ffffffffffffffffffffffffffffffffffffffff", true},
		{"too short", "abc123", false},
		{"too long", "abc123def456789012345678901234567890abcde", false},
		{"empty string", "", false},
		{"branch name main", "main", false},
		{"branch name with slashes", "feature/my-branch", false},
		{"uppercase hex rejected", "ABC123DEF456789012345678901234567890ABCD", false},
		{"mixed case rejected", "abc123DEF456789012345678901234567890abcd", false},
		{"non-hex character g", "gbc123def456789012345678901234567890abcd", false},
		{"39 chars", "abc123def456789012345678901234567890123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSHA(tt.input); got != tt.want {
				t.Errorf("isSHA(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
