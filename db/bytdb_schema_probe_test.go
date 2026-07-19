package db

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/rohanthewiz/bytdb"
	bsql "github.com/rohanthewiz/bytdb/sql"
)

// Executes every bootstrap DDL statement against a scratch engine so a
// dialect regression names the exact failing statement instead of
// surfacing as a bare parse error at site startup.
func TestBytdbSchemaStatements(t *testing.T) {
	eng, err := bytdb.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	bdb := bsql.New(eng)

	for _, tbl := range bytdbTables {
		for _, stmt := range tbl.ddl {
			if _, err := bdb.Exec(stmt); err != nil {
				t.Errorf("table %s\nstatement:\n%s\nerror: %s", tbl.name, stmt, fmt.Sprintf("%+v", err))
			}
		}
	}
}
