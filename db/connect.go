package db

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/bytdb"
	"github.com/rohanthewiz/bytdb/pgwire"
	bsql "github.com/rohanthewiz/bytdb/sql"
	"github.com/rohanthewiz/serr"
)

// Cache DB handle and options
var dbHandle *sql.DB
var dbOpts *DBOpts

// Embedded bytdb runtime — populated only when DBType is bytdb.
// The app still talks to it through dbHandle (lib/pq over a loopback
// pgwire connection) so SQLBoiler models and raw $n queries run
// unchanged against either backend; only the endpoint differs.
//
//	SQLBoiler / raw SQL ──lib/pq──▶ loopback TCP ──pgwire──▶ bsql ──▶ bytdb engine ──▶ WAL file
//
// Going through the wire instead of calling bsql directly costs a
// loopback round trip but buys total query-layer compatibility — the
// entire legacy data-access layer is untouched. If profiling ever
// shows the hop matters, individual hot paths can move to bsql
// directly without disturbing the rest.
var bytdbEngine *bytdb.Engine
var bytdbServer *pgwire.Server
var bytdbAddr string // actual listen address (resolves the :0 ephemeral port)

// BytDB is the default backend: embedded and WAL-durable, it lets each
// site deploy as one self-contained binary (single pod + block-storage
// volume). Postgres remains a supported fallback for existing installs.
var DBTypes = dbTypes{"postgres", "mysql", "bytdb"}

type dbTypes struct {
	Postgres, MySQL, BytDB string
}

type DBOpts struct {
	DBType   string
	Host     string
	Port     string
	User     string
	Word     string
	Database string
	// bytdb-only options. File is the single WAL-backed data file — it must
	// sit on a real filesystem (block storage in k8s, never object storage:
	// object stores cannot honor fsync-before-ack). Listen is the loopback
	// address pgwire binds; ":0" picks an ephemeral port so several sites
	// can share a host without coordinating port assignments.
	File   string // default "data/church.db"
	Listen string // default "127.0.0.1:0"
}

func InitDB(opts DBOpts) error {
	dbOpts = &opts
	err := openDB()
	if err != nil {
		return serr.Wrap(err, "Error initializing database")
	}
	return nil
}

func CloseDB() {
	if dbHandle != nil {
		dbHandle.Close()
	}
	// Order matters on the embedded path: the wire server drains before the
	// engine closes so no in-flight statement lands on a closed engine.
	if bytdbServer != nil {
		bytdbServer.Close()
		bytdbServer = nil
	}
	if bytdbEngine != nil {
		bytdbEngine.Close()
		bytdbEngine = nil
	}
}

// Get a valid DB handle
func Db() (*sql.DB, error) {
	if dbHandle != nil {
		if dbHandle.Ping() == nil { // pings without error
			return dbHandle, nil
		}
	}
	err := openDB()
	return dbHandle, err
}

func openDB() error {
	if dbOpts == nil {
		return serr.Wrap(errors.New("Please call InitDB before using database"))
	}
	// Empty DBType intentionally falls through to bytdb — the default backend.
	// Selecting Postgres (or MySQL) must be explicit in config.
	if dbOpts.DBType == "" || dbOpts.DBType == DBTypes.BytDB {
		return openBytDB()
	}
	opts := fmt.Sprintf("host=%s dbname=%s user=%s password=%s port=%s sslmode=disable",
		dbOpts.Host, dbOpts.Database, dbOpts.User, dbOpts.Word, dbOpts.Port,
	)
	db_, err := sql.Open(dbOpts.DBType, opts)
	if err != nil {
		return serr.Wrap(err, "Failed to open database")
	}
	dbHandle = db_
	return nil
}

// openBytDB brings up the embedded engine (once) and (re)dials it over
// the loopback wire. Split into "start" and "dial" halves because Db()
// retries openDB on a failed ping — the engine and listener must not be
// re-created on such a retry, only the client connection.
func openBytDB() error {
	if bytdbEngine == nil {
		if err := startBytDB(); err != nil {
			return err
		}
	}

	host, port, err := net.SplitHostPort(bytdbAddr)
	if err != nil {
		return serr.Wrap(err, "Invalid bytdb listen address", "addr", bytdbAddr)
	}
	dbName := dbOpts.Database
	if dbName == "" {
		dbName = "church"
	}
	// pgwire accepts any user/dbname; keeping the configured names makes
	// psql sessions and pg_stat-style introspection read naturally.
	dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s sslmode=disable", host, port, dbName, "church")
	db_, err := sql.Open("postgres", dsn)
	if err != nil {
		return serr.Wrap(err, "Failed to open loopback connection to bytdb")
	}
	// Fail fast here rather than on the first query: a broken embedded
	// setup should abort startup, not surface as scattered query errors.
	if err = db_.Ping(); err != nil {
		return serr.Wrap(err, "bytdb loopback connection did not answer ping", "addr", bytdbAddr)
	}
	dbHandle = db_
	return nil
}

// startBytDB opens the data file, ensures the schema, and starts the
// loopback pgwire listener. Listen-then-serve (rather than
// ListenAndServe) so that by the time this returns the port is
// accepting — no readiness race with the lib/pq dial that follows.
func startBytDB() error {
	file := dbOpts.File
	if file == "" {
		file = filepath.Join("data", "church.db")
	}
	if dir := filepath.Dir(file); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return serr.Wrap(err, "Could not create bytdb data directory", "dir", dir)
		}
	}

	eng, err := bytdb.Open(file)
	if err != nil {
		return serr.Wrap(err, "Failed to open bytdb data file", "file", file)
	}

	bdb := bsql.New(eng)
	// Schema bootstrap runs against the embedded handle before the wire is
	// up: no client can observe a half-created schema.
	if err = ensureBytDBSchema(bdb); err != nil {
		eng.Close()
		return serr.Wrap(err, "Failed ensuring bytdb schema")
	}

	addr := dbOpts.Listen
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		eng.Close()
		return serr.Wrap(err, "Failed to listen for bytdb wire connections", "addr", addr)
	}

	srv := pgwire.NewServer(bdb)
	go func() {
		// Serve returns nil after Close; anything else is a real accept-loop
		// failure. fmt (not logger) keeps db free of app-logging imports —
		// it is imported by packages that predate the logger setup.
		if serveErr := srv.Serve(ln); serveErr != nil {
			fmt.Println("bytdb pgwire server exited with error:", serveErr.Error())
		}
	}()

	bytdbEngine = eng
	bytdbServer = srv
	bytdbAddr = ln.Addr().String()
	fmt.Println("bytdb serving embedded database", "file:", file, "addr:", bytdbAddr)
	return nil
}

// BytDBWireAddr reports the loopback address pgwire is serving on
// ("" when the backend is not bytdb). Useful for attaching psql to a
// live site when Listen was left on an ephemeral port.
func BytDBWireAddr() string {
	return bytdbAddr
}
