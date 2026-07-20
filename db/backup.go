package db

import (
	"io"

	"github.com/rohanthewiz/serr"
)

// BytDBBackupTo streams a consistent snapshot of the embedded database to w
// and reports the bytes written. This is the only sanctioned way to copy the
// database of a running site: the engine coordinates the snapshot with the
// WAL, whereas copying the data file externally can capture a torn state.
// The engine handle stays unexported — callers get exactly this one
// capability, not arbitrary engine access.
//
// Errors when the active backend is not bytdb (Postgres installs back up
// with pg_dump); callers wanting a friendlier gate can pre-check
// BytDBWireAddr() != "".
func BytDBBackupTo(w io.Writer) (int64, error) {
	if bytdbEngine == nil {
		return 0, serr.New("database backup requires the bytdb backend")
	}
	n, err := bytdbEngine.BackupTo(w)
	if err != nil {
		return n, serr.Wrap(err, "error streaming bytdb backup")
	}
	return n, nil
}
