// Note: this package's init() (random.go) fatals without cfg/random_seeds.txt
// relative to the working directory. Tests run with cwd = this package dir, so
// a dummy fixture lives at resource/auth/cfg/random_seeds.txt purely to let
// `go test` load the package; production reads the real file at the app root.
package auth

import "testing"

// Login verification (auth_controller.AuthHandlerRWeb) recomputes the hash from
// the submitted password and the stored salt, so PasswordHash must be a pure
// function of (password, salt) — same inputs, same output, across restarts.
func TestPasswordHashDeterministic(t *testing.T) {
	salt := GenSalt("$&@randomness!!$$$")
	if salt == "" {
		t.Fatal("GenSalt returned an empty salt")
	}

	h1 := PasswordHash("abcde", salt)
	h2 := PasswordHash("abcde", salt)
	if h1 != h2 {
		t.Errorf("same password+salt produced different hashes: %s vs %s", h1, h2)
	}

	// cryptit derives a 32-byte key for password hashes, hex-encoded => 64 chars
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars: %s", len(h1), h1)
	}
}

func TestPasswordHashDiffersByInput(t *testing.T) {
	salt := GenSalt("$&@randomness!!$$$")

	if PasswordHash("abcde", salt) == PasswordHash("abcdf", salt) {
		t.Error("different passwords with the same salt should not collide")
	}

	// A per-user salt is what keeps identical passwords from sharing a hash
	// (defeats rainbow tables); verify a different salt changes the hash.
	otherSalt := salt + "x"
	if PasswordHash("abcde", salt) == PasswordHash("abcde", otherSalt) {
		t.Error("same password with different salts should not collide")
	}
}
