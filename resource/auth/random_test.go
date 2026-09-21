package auth

import (
	"regexp"
	"testing"
)

// hexOnly pins the alphabet: consumers put these values in cookies, Redis
// keys and hidden form fields, all of which are safe with [0-9a-f] and none
// of which should have to escape anything.
var hexOnly = regexp.MustCompile(`^[0-9a-f]+$`)

// TestRandomKeyShape pins the contract the rest of the app relies on: a
// 64-char lowercase hex string, the same shape the earlier scrypt-based
// RandomKey produced, so stored/compared tokens don't change format.
func TestRandomKeyShape(t *testing.T) {
	key := RandomKey()
	if len(key) != 64 {
		t.Errorf("RandomKey length = %d, want 64", len(key))
	}
	if !hexOnly.MatchString(key) {
		t.Errorf("RandomKey %q is not lowercase hex", key)
	}
}

func TestRandomStringShape(t *testing.T) {
	s := RandomString()
	if len(s) != 32 {
		t.Errorf("RandomString length = %d, want 32", len(s))
	}
	if !hexOnly.MatchString(s) {
		t.Errorf("RandomString %q is not lowercase hex", s)
	}
}

// TestRandomKeyUnique is a smoke test against a broken source (e.g. a zeroed
// buffer that never got filled), not a statistical test: with 256 bits per
// key, any repeat in a few thousand draws means the generator is not random.
func TestRandomKeyUnique(t *testing.T) {
	const draws = 5000
	seen := make(map[string]struct{}, draws)
	for i := 0; i < draws; i++ {
		key := RandomKey()
		if _, dup := seen[key]; dup {
			t.Fatalf("RandomKey repeated after %d draws: %q", i, key)
		}
		seen[key] = struct{}{}
	}
}

func TestRandomIntRange(t *testing.T) {
	const max = 7
	for i := 0; i < 1000; i++ {
		if n := RandomInt(max); n < 0 || n >= max {
			t.Fatalf("RandomInt(%d) = %d, want [0, %d)", max, n, max)
		}
	}
}
