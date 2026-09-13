// Postgres check for role-based admin access.
//
// Applies the Up section of db/migrate/20260913160000_CreateRolesTables.sql and
// drives the real authz query functions, all inside ONE transaction that is
// always rolled back. Postgres DDL is transactional, so the dev database ends
// exactly as it started: no roles tables, no role rows, no assignments.
//
// That makes this safe to run against a shared church_development that hasn't
// run the migration yet, and it proves the two things the bytdb tests can't:
// the goose SQL is valid Postgres, and the hand-written queries behave the
// same on lib/pq against real Postgres.
//
//	go run ./test_scripts/roles_pg_check   (from the church module root)
//
// Requires: local Postgres with church_development migrated up to (but not
// necessarily including) the roles migration.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/authz"
	"github.com/rohanthewiz/church/util/inputerr"
)

const migrationFile = "db/migrate/20260913160000_CreateRolesTables.sql"

var failures int

func expect(label string, cond bool, detail string) {
	if !cond {
		failures++
		fmt.Printf("FAIL  %s: %s\n", label, detail)
		return
	}
	fmt.Printf("pass  %s\n", label)
}

// upStatements extracts the goose Up section and splits it into statements.
// A plain split on ";" is enough for this file: it has no functions, DO
// blocks or semicolons inside literals.
func upStatements(sqlText string) []string {
	up := sqlText
	if i := strings.Index(up, "-- +goose Up"); i >= 0 {
		up = up[i+len("-- +goose Up"):]
	}
	if i := strings.Index(up, "-- +goose Down"); i >= 0 {
		up = up[:i]
	}
	var out []string
	for _, stmt := range strings.Split(up, ";") {
		var kept []string
		for _, line := range strings.Split(stmt, "\n") {
			if t := strings.TrimSpace(line); t != "" && !strings.HasPrefix(t, "--") {
				kept = append(kept, line)
			}
		}
		if len(kept) > 0 {
			out = append(out, strings.Join(kept, "\n"))
		}
	}
	return out
}

func main() {
	os.Exit(run())
}

// run returns the exit code; the deferred rollback must run before exit.
func run() int {
	err := db.InitDB(db.DBOpts{
		DBType: db.DBTypes.Postgres,
		Host:   "localhost", Port: "5432",
		User: "devuser", Word: "secret",
		Database: "church_development",
	})
	if err != nil {
		fmt.Println("FATAL InitDB:", err)
		return 1
	}
	defer db.CloseDB()
	dbH, err := db.Db()
	if err != nil {
		fmt.Println("FATAL Db:", err)
		return 1
	}

	tx, err := dbH.Begin()
	if err != nil {
		fmt.Println("FATAL Begin:", err)
		return 1
	}
	// Always roll back: this check must leave the database untouched.
	defer func() {
		if err := tx.Rollback(); err != nil {
			fmt.Println("WARN  rollback:", err)
		} else {
			fmt.Println("      rolled back: database unchanged")
		}
	}()

	raw, err := os.ReadFile(migrationFile)
	if err != nil {
		fmt.Println("FATAL reading migration:", err)
		return 1
	}
	for _, stmt := range upStatements(string(raw)) {
		if _, err := tx.Exec(stmt); err != nil {
			first := strings.SplitN(strings.TrimSpace(stmt), "\n", 2)[0]
			fmt.Printf("FAIL  migration statement %q: %v\n", first, err)
			return 1
		}
	}
	fmt.Println("pass  migration Up section applies on Postgres")

	// Seed users inside the transaction. Existing dev users get backfilled
	// too, which is fine: it all rolls back.
	ids := map[string]int64{}
	for _, u := range []struct {
		name string
		role int
	}{{"rolespg_admin", 1}, {"rolespg_editor", 7}, {"rolespg_member", 9}, {"rolespg_root", 99}} {
		var id int64
		err := tx.QueryRow(`INSERT INTO users (updated_by, enabled, role, username, email_address, first_name)
			VALUES ('rolespg', true, $1, $2, $3, $2) RETURNING id`, u.role, u.name, u.name+"@rolespg.test").Scan(&id)
		if err != nil {
			fmt.Printf("FAIL  seeding user %s: %v\n", u.name, err)
			return 1
		}
		ids[u.name] = id
	}

	if err := authz.EnsureDefaultRoles(tx); err != nil {
		fmt.Println("FAIL  EnsureDefaultRoles:", err)
		return 1
	}
	roles, err := authz.ListRoles(tx)
	expect("default roles created", err == nil && len(roles) == 3, fmt.Sprintf("%d roles, err %v", len(roles), err))

	load := func(name string) *authz.Actor {
		a, found, err := authz.LoadActor(tx, name)
		if err != nil || !found {
			fmt.Printf("FAIL  LoadActor %s: found=%v err=%v\n", name, found, err)
			return nil
		}
		return a
	}
	if a := load("rolespg_admin"); a != nil {
		expect("legacy admin backfilled to Administrator", a.Can(authz.RolesUpdate) && a.Can(authz.ChargesRead), "")
	}
	if a := load("rolespg_editor"); a != nil {
		expect("legacy editor backfilled to Editor", a.Can(authz.ArticlesUpdate) && !a.Can(authz.ArticlesPublish), "")
	}
	if a := load("rolespg_member"); a != nil {
		expect("legacy member gets no admin access", !a.HasAdminAccess(), "")
	}

	_, err = authz.SaveRole(tx, authz.Role{Name: "EDITOR"}, "rolespg")
	_, isInput := inputerr.UserMessage(err)
	expect("duplicate role name refused (case-insensitive)", isInput, fmt.Sprintf("err %v", err))

	greeter, err := authz.SaveRole(tx, authz.Role{Name: "PG Greeter", Perms: authz.NewSet(authz.ChargesRead)}, "rolespg")
	expect("create role", err == nil && greeter > 0, fmt.Sprintf("err %v", err))

	edRoles, _ := authz.RoleIDsForUser(tx, ids["rolespg_editor"])
	err = authz.SetUserRoles(tx, ids["rolespg_editor"], append(edRoles, greeter))
	a := load("rolespg_editor")
	expect("two roles combine", err == nil && a != nil && a.Can(authz.ChargesRead) && a.Can(authz.ArticlesUpdate),
		fmt.Sprintf("err %v", err))

	names, err := authz.RoleNamesByUser(tx)
	expect("RoleNamesByUser", err == nil && len(names[ids["rolespg_editor"]]) == 2, fmt.Sprintf("%v err %v", names[ids["rolespg_editor"]], err))
	perms, err := authz.PermsByUser(tx)
	expect("PermsByUser", err == nil && perms[ids["rolespg_editor"]].Has(authz.ChargesRead), fmt.Sprintf("err %v", err))

	editor := load("rolespg_editor")
	ok, err := authz.CanManageUser(tx, editor, ids["rolespg_admin"])
	expect("editor can't manage an Administrator", err == nil && !ok, fmt.Sprintf("ok %v err %v", ok, err))
	admin := load("rolespg_admin")
	ok, err = authz.CanManageUser(tx, admin, ids["rolespg_root"])
	expect("Administrator can't manage a SuperAdmin", err == nil && !ok, fmt.Sprintf("ok %v err %v", ok, err))

	var artID int64
	err = tx.QueryRow(`INSERT INTO articles (updated_by, title, slug, summary, published, categories)
		VALUES ('rolespg', 'Live', 'rolespg-live', 's', true, '{}') RETURNING id`).Scan(&artID)
	if err != nil {
		fmt.Println("FAIL  seeding article:", err)
		return 1
	}
	v, err := authz.ResolveFlag(tx, editor, authz.ArticlesPublish, authz.FlagArticlePublished, fmt.Sprint(artID), false)
	expect("editor can't unpublish a live article", err == nil && v, fmt.Sprintf("v %v err %v", v, err))

	err = authz.DeleteRole(tx, greeter)
	a = load("rolespg_editor")
	expect("deleting a role revokes it", err == nil && a != nil && !a.Can(authz.ChargesRead), fmt.Sprintf("err %v", err))

	if failures > 0 {
		fmt.Printf("\n%d check(s) FAILED\n", failures)
		return 1
	}
	fmt.Println("\nall Postgres role checks passed")
	return 0
}
