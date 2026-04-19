package kvstore

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// The tests share package state (the singleton map). Each test uses a unique
// key prefix so they don't collide when go test runs them in sequence, and
// the race-test uses its own prefix too.

func TestSetGetRoundtrip(t *testing.T) {
	if err := Set("rt:a", "hello", time.Minute); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	got, err := Get("rt:a")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("Get returned %q, want %q", got, "hello")
	}
}

func TestGetMissingKeyReturnsKeyNotExists(t *testing.T) {
	_, err := Get("missing:never-set")
	if err == nil {
		t.Fatal("Get on missing key returned nil error")
	}
	if !strings.Contains(err.Error(), KeyNotExists) {
		t.Fatalf("error %q does not contain sentinel %q", err.Error(), KeyNotExists)
	}
}

func TestSetEmptyKeyErrors(t *testing.T) {
	if err := Set("", "v", time.Minute); err == nil {
		t.Fatal("Set with empty key should error")
	}
}

func TestZeroTTLPersists(t *testing.T) {
	// ttl<=0 means no expiry (matches roredis semantics).
	if err := Set("persist:a", "v", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}
	// Force a sweep far into the future — entry must survive.
	sweepExpired(time.Now().Add(24 * time.Hour))
	got, err := Get("persist:a")
	if err != nil {
		t.Fatalf("Get after far-future sweep: %v", err)
	}
	if got != "v" {
		t.Fatalf("got %q, want %q", got, "v")
	}
}

func TestTTLLazyExpiryOnGet(t *testing.T) {
	// Short TTL, wait past it, Get must report missing without relying on the
	// janitor having run yet.
	if err := Set("ttl:lazy", "x", 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	_, err := Get("ttl:lazy")
	if err == nil || !strings.Contains(err.Error(), KeyNotExists) {
		t.Fatalf("expected KeyNotExists after TTL, got err=%v", err)
	}
}

func TestJanitorSweepsExpired(t *testing.T) {
	if err := Set("sweep:a", "x", 5*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	// Drive the sweep deterministically rather than waiting on the ticker.
	time.Sleep(10 * time.Millisecond)
	sweepExpired(time.Now())

	mu.RLock()
	_, present := items["sweep:a"]
	mu.RUnlock()
	if present {
		t.Fatal("janitor did not remove expired entry")
	}
}

func TestDelRemovesKey(t *testing.T) {
	if err := Set("del:a", "v", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := Del("del:a"); err != nil {
		t.Fatalf("Del: %v", err)
	}
	_, err := Get("del:a")
	if err == nil || !strings.Contains(err.Error(), KeyNotExists) {
		t.Fatalf("expected KeyNotExists after Del, got err=%v", err)
	}
}

func TestDelMissingIsNoOp(t *testing.T) {
	if err := Del("del:never-existed"); err != nil {
		t.Fatalf("Del on missing key should not error, got %v", err)
	}
}

// TestConcurrentAccess exercises the RWMutex under load. Run with -race to
// catch any mutex/map misuse. The goroutines write, read, and delete against
// a shared keyspace so the coverage hits all three paths concurrently.
func TestConcurrentAccess(t *testing.T) {
	const workers = 16
	const iterations = 500
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(id int) {
			defer wg.Done()
			for i := range iterations {
				k := "conc:" + string(rune('a'+id%26))
				_ = Set(k, "v", time.Second)
				_, _ = Get(k)
				if i%5 == 0 {
					_ = Del(k)
				}
			}
		}(w)
	}
	wg.Wait()
}