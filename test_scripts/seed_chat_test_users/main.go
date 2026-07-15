// Seeds two throwaway users for the chat/prayer-wall end-to-end smoke test:
//
//	chat-tester   (RegisteredUser, 9) — posts messages
//	chat-editor   (Author/Editor, 7)  — exercises keep/moderation
//
// Password for both: smoke-pass-1. Uses the app's own scrypt hashing
// (resource/auth) so the real login path verifies the credentials.
// Idempotent: re-running updates the existing rows.
//
// Run from the church project root (resource/auth loads cfg/random_seeds.txt
// relative to the working directory):
//
//	go run test_scripts/seed_chat_test_users/main.go
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/rohanthewiz/church/resource/auth"
)

const password = "smoke-pass-1"

func main() {
	dbH, err := sql.Open("postgres",
		"user=devuser password=secret dbname=church_development sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer dbH.Close()

	users := []struct {
		username string
		role     int
	}{
		{"chat-tester", 9}, // RegisteredUser
		{"chat-editor", 7}, // Author ("Editor")
	}
	for _, u := range users {
		salt := auth.GenSalt(u.username)
		hash := auth.PasswordHash(password, salt)
		_, err = dbH.Exec(`
			INSERT INTO users (updated_by, enabled, role, username, email_address,
				first_name, last_name, encrypted_password, encrypted_salt, created_at, updated_at)
			VALUES ('seed_chat_test_users', true, $1, $2, $2 || '@example.com',
				'Smoke', 'Tester', $3, $4, now(), now())
			ON CONFLICT (username) DO UPDATE
				SET role = $1, encrypted_password = $3, encrypted_salt = $4, enabled = true`,
			u.role, u.username, hash, salt)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("seeded", u.username, "role", u.role)
	}
}
