// Throwaway seeder for the N-013 browser click-through. Targets church_test
// ONLY (a copy of church_development), never the dev database.
//
// It creates the cast the click-through needs, each with a distinct authority
// so the permission-driven UI has something to differ about:
//
//	n013-super    legacy 99 (SuperAdmin), no role rows  — sees /debug, everything
//	n013-admin    legacy  1, Administrator role         — full catalog, not SuperAdmin
//	n013-editor   legacy  7, Editor role                — no publish, no delete
//	n013-reader   legacy  9, "Read Only" role (new)     — *.read only: no +, Edit or Delete
//	n013-member   legacy  9, no roles                   — no admin access at all
//
// Passwords are all n013-pass-1. Hashing goes through resource/auth so the real
// login path verifies them. Run from the church project root:
//
//	go run ./test_scripts/n013_seed
//
// Idempotent: re-running updates the rows and re-applies the role grants.
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/church/resource/auth"
)

const password = "n013-pass-1"

// readOnlyPerms is every *.read in the catalog plus nothing else. The point is
// a user who may open each admin list but must not see "+", "Edit" or
// "Delete" on it (session 2026-09-17, item 16).
// userMgrPerms is deliberately narrow: enough to open and save the user form,
// and nothing else. An actor holding only these sees every richer role
// (Administrator, Publisher, ...) LOCKED on the user form, because a role that
// grants permissions the actor lacks must not be grantable by them.
var userMgrPerms = []string{
	"users.read", "users.update", "users.enable", "roles.read", "articles.read",
}

var readOnlyPerms = []string{
	"articles.read", "events.read", "sermons.read",
	"pages.read", "menus.read", "users.read", "roles.read", "charges.read",
}

type person struct {
	username string
	role     int    // legacy numeric role
	roleName string // row in `roles` to grant, "" for none
}

func main() {
	dbH, err := sql.Open("postgres",
		"user=devuser password=secret dbname=church_test sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer dbH.Close()

	// Guard: refuse to run anywhere but church_test. A copy-paste of this file
	// pointed at church_development would rewrite the real admin's password.
	var dbName string
	if err = dbH.QueryRow("select current_database()").Scan(&dbName); err != nil {
		log.Fatal(err)
	}
	if dbName != "church_test" {
		log.Fatalf("refusing to seed %q; this script is for church_test only", dbName)
	}

	readOnlyID := ensureRole(dbH, "Read Only",
		"N-013: may open admin lists, may not change anything", readOnlyPerms)
	userMgrID := ensureRole(dbH, "User Manager",
		"N-013: may manage users, holds no content permissions", userMgrPerms)

	for _, p := range []person{
		{"n013-super", 99, ""},
		{"n013-admin", 1, "Administrator"},
		{"n013-editor", 7, "Editor"},
		{"n013-reader", 9, "Read Only"},
		{"n013-member", 9, ""},
		{"n013-usermgr", 9, "User Manager"},
	} {
		id := upsertUser(dbH, p)
		if p.roleName != "" {
			grant(dbH, id, roleID(dbH, p.roleName))
		}
		fmt.Printf("%-12s legacy=%-2d role=%-14s id=%d\n", p.username, p.role, p.roleName, id)
	}
	fmt.Printf("Read Only role id=%d (%d perms), User Manager role id=%d (%d perms)\n",
		readOnlyID, len(readOnlyPerms), userMgrID, len(userMgrPerms))
	fmt.Println("password for all:", password)
}

// ensureRole creates (or refreshes) a role and sets exactly the given
// permissions. Permissions are deleted and re-inserted rather than upserted: it
// is the same delete-then-insert the role handler uses, and it keeps a re-run
// from accumulating duplicates.
func ensureRole(dbH *sql.DB, name, desc string, perms []string) int64 {
	var id int64
	err := dbH.QueryRow(`insert into roles (name, description, updated_by, created_at, updated_at)
		values ($1, $2, 'n013_seed', now(), now())
		on conflict (name) do update set description = excluded.description, updated_at = now()
		returning id`, name, desc).Scan(&id)
	if err != nil {
		log.Fatal("role: ", err)
	}
	if _, err = dbH.Exec(`delete from role_permissions where role_id = $1`, id); err != nil {
		log.Fatal("clear perms: ", err)
	}
	for _, perm := range perms {
		if _, err = dbH.Exec(`insert into role_permissions (role_id, permission)
			values ($1, $2)`, id, perm); err != nil {
			log.Fatal("perm ", perm, ": ", err)
		}
	}
	return id
}

func upsertUser(dbH *sql.DB, p person) int64 {
	salt := auth.GenSalt(p.username)
	hash := auth.PasswordHash(password, salt)
	var id int64
	err := dbH.QueryRow(`insert into users
		(username, role, enabled, email_address, first_name, last_name, updated_by,
		 encrypted_password, encrypted_salt, created_at, updated_at, confirmed_at)
		values ($1, $2, true, $3, $4, 'Tester', 'n013_seed', $5, $6, now(), now(), now())
		on conflict (username) do update set
			role = excluded.role, enabled = true,
			encrypted_password = excluded.encrypted_password,
			encrypted_salt = excluded.encrypted_salt,
			updated_at = now(), updated_by = 'n013_seed'
		returning id`,
		p.username, p.role, p.username+"@example.com", p.username, hash, salt).Scan(&id)
	if err != nil {
		log.Fatal("user ", p.username, ": ", err)
	}
	return id
}

func roleID(dbH *sql.DB, name string) int64 {
	var id int64
	if err := dbH.QueryRow(`select id from roles where name = $1`, name).Scan(&id); err != nil {
		log.Fatal("role lookup ", name, ": ", err)
	}
	return id
}

func grant(dbH *sql.DB, userID, roleIDv int64) {
	if _, err := dbH.Exec(`insert into user_roles (user_id, role_id, created_at)
		values ($1, $2, now()) on conflict (user_id, role_id) do nothing`,
		userID, roleIDv); err != nil {
		log.Fatal("grant: ", err)
	}
}
