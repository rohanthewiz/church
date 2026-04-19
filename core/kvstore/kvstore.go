// Package kvstore is an in-process key-value store with per-entry TTL.
//
// It replaces the previous roredis-backed session/token store. The store is a
// map guarded by a sync.RWMutex; each entry carries its own expiresAt time.
// Expiry is enforced two ways:
//
//  1. Lazily, inside Get: if a key's expiresAt has passed, Get deletes it and
//     returns KeyNotExists. This makes reads correct even if the janitor is
//     behind.
//  2. Proactively, by a background janitor goroutine that periodically sweeps
//     and deletes expired entries, reclaiming memory for keys that are never
//     read again (e.g. abandoned form tokens).
//
// Design choices:
//
//   - Single package-level store. The callers (sessions, form tokens) are the
//     only consumers in this codebase and they share process memory anyway, so
//     a singleton avoids threading a handle through the app. If a second use
//     case ever wants isolated namespaces, swap in a Store struct.
//   - RWMutex over sync.Map: entries have structured values (value + expiry),
//     and Get needs to both read and conditionally delete under a consistent
//     view. A plain map + RWMutex makes that straightforward.
//   - Error shape: Get returns an error whose Error() contains the sentinel
//     "Key does not exist" so the existing auth middleware, which uses
//     strings.Contains against session.KeyNotExists, continues to work
//     unchanged.
//
// Durability: the store is process-local and non-persistent. A restart
// invalidates all sessions and form tokens. This is acceptable for a single-
// binary per-site deployment; if horizontal scaling is ever on the table,
// swap this out for a shared store.
package kvstore

import (
	"sync"
	"time"

	"github.com/rohanthewiz/serr"
)

// KeyNotExists is embedded in the error message returned by Get when a key is
// absent or expired. Callers detect the condition with strings.Contains so
// that wrapping layers (serr) don't hide it.
const KeyNotExists = "Key does not exist"

// defaultSweepInterval is the cadence of the background janitor. 60s is a
// trade-off between wasted wakeups on a quiet process and delaying reclamation
// of expired but unread keys (e.g. form tokens users never submit). Lazy
// eviction in Get keeps correctness independent of this value.
const defaultSweepInterval = 60 * time.Second

type entry struct {
	value     string
	expiresAt time.Time // zero => no expiry
}

var (
	mu        sync.RWMutex
	items     = make(map[string]entry)
	startOnce sync.Once
)

// init starts the janitor automatically so callers don't have to wire it up.
// sync.Once ensures tests that import this package multiple times (or future
// callers of a public Init) never double-start the goroutine.
func init() {
	startJanitor(defaultSweepInterval)
}

func startJanitor(interval time.Duration) {
	startOnce.Do(func() {
		go sweepLoop(interval)
	})
}

func sweepLoop(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		sweepExpired(time.Now())
	}
}

// sweepExpired walks the map once under a write lock and drops expired keys.
// Split out from sweepLoop so tests can drive deterministic sweeps.
func sweepExpired(now time.Time) {
	mu.Lock()
	defer mu.Unlock()
	for k, e := range items {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(items, k)
		}
	}
}

// Set writes value under key. A positive ttl sets an expiry; a zero or
// negative ttl stores the entry with no expiry (matches roredis semantics
// where a zero expiration means persist).
func Set(key, value string, ttl time.Duration) error {
	if key == "" {
		return serr.New("kvstore: key is empty on Set")
	}
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	mu.Lock()
	items[key] = entry{value: value, expiresAt: exp}
	mu.Unlock()
	return nil
}

// Get returns the value under key, or an error containing KeyNotExists if the
// key is missing or expired. It performs lazy eviction when it finds an
// expired entry so a slow janitor can't serve stale data.
func Get(key string) (string, error) {
	mu.RLock()
	e, ok := items[key]
	mu.RUnlock()
	if !ok {
		return "", serr.New(KeyNotExists, "key", key)
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		// Lazy eviction. Re-check under the write lock in case a concurrent
		// Set replaced the entry with a fresh one between our read and the
		// re-acquire; only delete if the current entry is still expired.
		mu.Lock()
		if cur, still := items[key]; still && !cur.expiresAt.IsZero() && time.Now().After(cur.expiresAt) {
			delete(items, key)
		}
		mu.Unlock()
		return "", serr.New(KeyNotExists, "key", key)
	}
	return e.value, nil
}

// Del removes key. It is a no-op if the key is absent, matching roredis.Del.
func Del(key string) error {
	mu.Lock()
	delete(items, key)
	mu.Unlock()
	return nil
}