// Live smoke test for the Phase 2 mobile auth flow against a local dev DB.
// Seeds a user with known credentials, then drives the real rweb routes
// in-process (Server.Request — no listener): login → me → logout → me.
// This exercises the hand-written api_tokens SQL against real Postgres,
// which the sqlmock contract tests by design cannot.
//
// Run (from repo root, so cfg/random_seeds.txt resolves):
//
//	go run ./test_scripts/auth_live_check
//
// Requires: local Postgres with church_development migrated (goose up).
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/rweb"
)

const (
	smokeUser = "authsmoke"
	smokePass = "correct-horse-battery"
)

func main() {
	err := db.InitDB(db.DBOpts{
		DBType: db.DBTypes.Postgres,
		Host:   "localhost", Port: "5432",
		User: "devuser", Word: "secret",
		Database: "church_development",
	})
	if err != nil {
		log.Fatal(err)
	}
	dbH, _ := db.Db()

	defer cleanup(dbH)
	cleanup(dbH)

	// Seed a user the way the admin user form would (scrypt hash + salt)
	salt := auth.GenSalt("auth-live-check")
	hash := auth.PasswordHash(smokePass, salt)
	var userID int64
	err = dbH.QueryRow(`INSERT INTO users
		(updated_by, enabled, role, username, email_address, first_name, last_name,
		 encrypted_password, encrypted_salt)
		VALUES ('smoke', true, 9, $1, 'authsmoke@example.com', 'Auth', 'Smoke', $2, $3)
		RETURNING id`, smokeUser, hash, salt).Scan(&userID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Seeded user %s (id %d)\n\n", smokeUser, userID)

	// Same route shapes as router_rweb.go
	s := rweb.NewServer(rweb.ServerOptions{})
	api := s.Group("/api/v1")
	api.Post("/auth/login", apitoken.APILoginRWeb)
	api.Get("/auth/me", apitoken.APIGuard(apitoken.APIMeRWeb))
	api.Post("/auth/logout", apitoken.APIGuard(apitoken.APILogoutRWeb))

	jsonHdr := []rweb.Header{{Key: "Content-Type", Value: "application/json"}}

	// 1. Login — wrong password first (expect 401), then right (expect 200)
	resp := s.Request("POST", "/api/v1/auth/login", jsonHdr,
		strings.NewReader(`{"username":"`+smokeUser+`","password":"wrong"}`))
	fmt.Printf("login (bad pass):  %d %s\n", resp.Status(), resp.Body())

	resp = s.Request("POST", "/api/v1/auth/login", jsonHdr, strings.NewReader(
		`{"username":"`+smokeUser+`","password":"`+smokePass+`","device":"live-check"}`))
	fmt.Printf("login (good):      %d %s\n", resp.Status(), truncate(resp.Body()))
	if resp.Status() != 200 {
		log.Fatal("login failed — aborting")
	}
	var loginDoc struct {
		Token string `json:"token"`
	}
	if err = json.Unmarshal(resp.Body(), &loginDoc); err != nil || loginDoc.Token == "" {
		log.Fatal("no token in login response")
	}

	// Confirm the DB holds a hash, not the plaintext token
	var stored string
	err = dbH.QueryRow(`SELECT token_hash FROM api_tokens WHERE user_id = $1`, userID).Scan(&stored)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("db row:            hash stored=%v, plaintext stored=%v\n",
		stored == apitoken.HashToken(loginDoc.Token), stored == loginDoc.Token)

	bearer := []rweb.Header{{Key: "Authorization", Value: "Bearer " + loginDoc.Token}}

	// 2. Me — with token, then with garbage
	resp = s.Request("GET", "/api/v1/auth/me", bearer, nil)
	fmt.Printf("me (valid):        %d %s\n", resp.Status(), resp.Body())
	resp = s.Request("GET", "/api/v1/auth/me",
		[]rweb.Header{{Key: "Authorization", Value: "Bearer deadbeef"}}, nil)
	fmt.Printf("me (bad token):    %d %s\n", resp.Status(), resp.Body())

	// 3. Logout, then the token must stop working
	resp = s.Request("POST", "/api/v1/auth/logout", bearer, nil)
	fmt.Printf("logout:            %d %s\n", resp.Status(), resp.Body())
	resp = s.Request("GET", "/api/v1/auth/me", bearer, nil)
	fmt.Printf("me (after logout): %d %s\n", resp.Status(), resp.Body())

	var remaining int
	_ = dbH.QueryRow(`SELECT count(*) FROM api_tokens WHERE user_id = $1`, userID).Scan(&remaining)
	fmt.Printf("\ntokens remaining for user: %d (want 0)\n", remaining)
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 220 {
		return s[:220] + "..."
	}
	return s
}

func cleanup(dbH *sql.DB) {
	// api_tokens rows cascade with the user
	if _, err := dbH.Exec(`DELETE FROM users WHERE username = $1`, smokeUser); err != nil {
		log.Println("cleanup:", err)
	}
}
