package security

import "testing"

func TestTokenEqual(t *testing.T) {
	tests := []struct {
		got, want string
		ok        bool
	}{
		{"secret", "secret", true},
		{"secret", "nope", false},
		{"", "secret", false},
		{"anything", "", true},
	}
	for _, tc := range tests {
		if got := TokenEqual(tc.got, tc.want); got != tc.ok {
			t.Fatalf("TokenEqual(%q, %q) = %v, want %v", tc.got, tc.want, got, tc.ok)
		}
	}
}
