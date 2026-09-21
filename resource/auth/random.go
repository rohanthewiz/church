package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

// Randomness in this package comes straight from crypto/rand, the OS CSPRNG
// (getrandom/arc4random under the hood).
//
// Design note: this replaced a "seed pool" scheme in which init() loaded
// cfg/random_seeds.txt and RandomKey scrypt-hashed three strings picked from
// that pool together with a nanosecond timestamp. The pool was dropped because
//   - it added no entropy the CSPRNG didn't already provide: the picks
//     themselves were made with crypto/rand, so the pool only diluted 256 bits
//     of OS randomness down to three ~6-bit choices plus a guessable clock;
//   - it made a secret file a hard boot dependency (init() log.Fatal'd without
//     it, before main()), which meant a fixture copy in every package that
//     imports auth, a Secret key in every k8s site, and a production guard
//     against deploying the committed sample;
//   - each key cost an scrypt derivation (tens of ms, 16 MiB) on the login path.
//
// A direct CSPRNG draw is both stronger and has no configuration to get wrong.

// randomKeyBytes is the size of a RandomKey before hex encoding. 32 bytes =
// 256 bits, which is unguessable by brute force, and hex-encodes to the same
// 64-character lowercase string the scrypt-based version produced — so session
// keys, form tokens and the SuperAdmin bootstrap token keep their shape for
// anything that stores or compares them (Redis keys, cookies, hidden fields).
const randomKeyBytes = 32

// randomStringBytes sizes RandomString: 16 bytes = 128 bits → 32 hex chars.
// That is ample for its use as a collision-free (not secret) identifier.
const randomStringBytes = 16

// randomHex returns n CSPRNG bytes, hex encoded (2n characters, [0-9a-f]).
//
// rand.Read's error is deliberately ignored: since Go 1.24 crypto/rand.Read
// never returns an error — if the OS entropy source fails it crashes the
// program irrecoverably rather than hand back predictable bytes. That is the
// behavior wanted here too; a security token must never silently degrade.
func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// RandomString returns a fresh 32-character random hex string. It is meant
// for unique identifiers (e.g. DOM ids for a module instance), not secrets —
// use RandomKey for anything that guards access.
func RandomString() string {
	return randomHex(randomStringBytes)
}

// RandomInt returns a uniform random integer in [0, max).
func RandomInt(max int64) int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		fmt.Println("Rand Int generator failed")
	}
	return int64(n.Int64())
}

// RandomKey returns a 64-character hex string carrying 256 bits of CSPRNG
// output. It backs session keys, form (CSRF) tokens and the SuperAdmin
// bootstrap token.
func RandomKey() string {
	return randomHex(randomKeyBytes)
}
