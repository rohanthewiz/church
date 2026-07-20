// selfheal_probe verifies, on an actual migrated data file, that bytdb's
// identity counters self-healed during the id-preserving copy: a DEFAULT-id
// insert must land ABOVE max(id), not collide with a migrated row. This is
// the on-file confirmation of the upstream finding that made setval
// unnecessary in pg_to_bytdb.
//
//	go run ./test_scripts/selfheal_probe -file <migrated.db>
//
// The probe row is deleted again immediately; only an id gap remains.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rohanthewiz/bytdb"
	bsql "github.com/rohanthewiz/bytdb/sql"
)

func main() {
	file := flag.String("file", "", "migrated bytdb data file to probe")
	flag.Parse()
	if *file == "" {
		fmt.Println("usage: selfheal_probe -file <migrated.db>")
		os.Exit(2)
	}

	eng, err := bytdb.Open(*file)
	if err != nil {
		fmt.Println("could not open data file:", err)
		os.Exit(1)
	}
	defer eng.Close()
	d := bsql.New(eng)

	res, err := d.Exec(`SELECT COALESCE(max(id), 0) FROM users`)
	if err != nil || len(res.Rows) == 0 {
		fmt.Println("could not read max(id) from users:", err)
		os.Exit(1)
	}
	maxID, _ := res.Rows[0][0].(int64)

	// Insert-then-delete (plain DB.Exec has no transaction session): the
	// probe row is removed immediately, leaving only a gap at the probed id —
	// harmless, and identical to what any deleted row leaves behind.
	res, err = d.Exec(`INSERT INTO users
		(updated_by, enabled, role, username, email_address, first_name)
		VALUES ('selfheal_probe', false, 9, '__probe__', '__probe__@example.invalid', 'Probe')
		RETURNING id`)
	if err != nil {
		fmt.Println("probe insert failed:", err)
		os.Exit(1)
	}
	newID, _ := res.Rows[0][0].(int64)
	if _, err = d.Exec(fmt.Sprintf(`DELETE FROM users WHERE id = %d`, newID)); err != nil {
		fmt.Println("warning: could not remove probe row:", err)
	}

	if newID <= maxID {
		fmt.Printf("FAIL: DEFAULT insert got id %d, which collides with migrated max(id) %d\n", newID, maxID)
		os.Exit(1)
	}
	fmt.Printf("PASS: max(id)=%d, DEFAULT insert got id %d — identity counter self-healed; no setval needed\n",
		maxID, newID)
}
