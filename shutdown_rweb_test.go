package church

import (
	"testing"
	"time"

	"github.com/rohanthewiz/rweb"
)

// TestInflightDrain checks that drain waits for a request still inside the
// handler chain, and gives up at the timeout when one never finishes.
func TestInflightDrain(t *testing.T) {
	f := &inflight{}
	release := make(chan struct{})
	entered := make(chan struct{})

	s := rweb.NewServer(rweb.ServerOptions{})
	s.Use(f.middleware)
	s.Get("/slow", func(ctx rweb.Context) error {
		close(entered)
		<-release
		return ctx.WriteText("done")
	})

	go s.Request("GET", "/slow", nil, nil)
	<-entered

	if got := f.count.Load(); got != 1 {
		t.Fatalf("in-flight count = %d, want 1", got)
	}
	if f.drain(50 * time.Millisecond) {
		t.Fatal("drain reported done while a request was still running")
	}

	close(release)
	if !f.drain(2 * time.Second) {
		t.Fatal("drain timed out after the request finished")
	}
	if got := f.count.Load(); got != 0 {
		t.Errorf("in-flight count = %d after drain, want 0", got)
	}
}
