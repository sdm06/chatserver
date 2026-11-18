// Package security holds the small, testable pieces that keep the chat server
// honest: constant-time token checks, rate limiting, and input validation.
package security

import "crypto/subtle"

// TokenEqual compares a client-supplied token with the configured one.
//
// It uses crypto/subtle so the comparison time does not depend on where the
// first differing byte is. An empty configured token means "auth disabled".
func TokenEqual(got, want string) bool {
	if want == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
