package church

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// Graceful shutdown.
//
// rweb's Server.Run traps SIGINT/SIGTERM itself: it closes the listener and
// returns nil. Connection goroutines it already started keep running, so a
// request that is mid-write when a pod is told to stop would, without this,
// race the database closing underneath it. ServeRWeb therefore finishes with:
//
//	SIGTERM ─► rweb closes listener, Run returns
//	        ─► wait for in-flight requests (at most shutdownDrainTimeout)
//	        ─► db.CloseDB(): final replication ship, wire server drain,
//	           engine close (flushes the bytdb WAL)
//
// Closing here, rather than relying on each site's `defer db.CloseDB()` in
// main, makes the order (drain, then close) part of the framework. The sites'
// defers still run afterwards and are harmless: CloseDB is idempotent.
//
// The drain is bounded because some requests never finish on their own: a
// /chat/stream SSE subscriber holds its request open for as long as the
// browser tab does. 10s leaves most of Kubernetes' default 30s grace period
// for the database close, which is the part that protects data.
const shutdownDrainTimeout = 10 * time.Second

// inflight counts requests between entering and leaving the handler chain.
// The WaitGroup is what shutdown waits on; the counter is only for the log
// line, since a WaitGroup can't report its count.
type inflight struct {
	wg    sync.WaitGroup
	count atomic.Int64
}

// middleware tracks each request for the life of the rest of the chain.
// Installed with Server.Use, so it wraps every route, groups included.
func (f *inflight) middleware(ctx rweb.Context) error {
	f.wg.Add(1)
	f.count.Add(1)
	defer func() {
		f.count.Add(-1)
		f.wg.Done()
	}()
	return ctx.Next()
}

// drain waits for in-flight requests to finish, up to timeout, and reports
// whether they all did.
func (f *inflight) drain(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		f.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// shutdown runs after Server.Run returns: drain, then close the database.
func (f *inflight) shutdown(timeout time.Duration) {
	logger.Info("Server stopped accepting connections; draining in-flight requests",
		"in_flight", f.count.Load(), "timeout", timeout.String())
	if !f.drain(timeout) {
		// Usually open SSE streams. Closing the DB under them is fine: they
		// only read, and the process is exiting.
		logger.Warn("Drain timed out; closing the database with requests still open",
			"in_flight", f.count.Load())
	}
	db.CloseDB()
	logger.Info("Database closed; shutdown complete")
}
