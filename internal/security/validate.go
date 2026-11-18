package security

import (
	"strings"
	"unicode"
)

// ValidNick accepts 1-32 letters, digits, underscores, or hyphens.
func ValidNick(s string) bool {
	return validName(s)
}

// ValidRoom accepts 1-32 letters, digits, underscores, or hyphens.
func ValidRoom(s string) bool {
	return validName(s)
}

func validName(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// ValidMessage rejects oversized payloads and NUL bytes.
func ValidMessage(s string, max int) bool {
	if max < 1 {
		max = 1
	}
	return len(s) <= max && !strings.ContainsRune(s, '\x00')
}
