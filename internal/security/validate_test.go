package security

import "testing"

func TestValidNick(t *testing.T) {
	tests := []struct {
		in string
		ok bool
	}{
		{"alice", true},
		{"alice_99", true},
		{"a-b", true},
		{"", false},
		{"has space", false},
		{"emoji😀", false},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
	}
	for _, tc := range tests {
		if got := ValidNick(tc.in); got != tc.ok {
			t.Fatalf("ValidNick(%q) = %v, want %v", tc.in, got, tc.ok)
		}
	}
}

func TestValidMessage(t *testing.T) {
	if !ValidMessage("hello", 10) {
		t.Fatal("hello should be valid")
	}
	if ValidMessage("hello world", 5) {
		t.Fatal("too long should be invalid")
	}
	if ValidMessage("bad\x00byte", 100) {
		t.Fatal("NUL byte should be invalid")
	}
}
