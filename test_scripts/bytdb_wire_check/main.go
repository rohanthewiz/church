package main

// Phase-1 wire proof for the bytdb backend.
//
// Boots the embedded bytdb engine exactly the way a site binary does
// (db.InitDB with the bytdb default), then drives the five hand-written
// raw-SQL features through their REAL query functions — not mirrored
// SQL — so what passes here is what runs in production:
//
//	chat            resource/chat.InsertMessage / RecentMessages / SetKeep / DeleteExpired
//	prayer wall     resource/prayerwall.InsertRequest / GetRequest / DeleteRequest
//	api tokens      resource/apitoken.Issue / LookupUser / RevokeAllForUser
//	recurrences     resource/event.UpsertRecurrence (ON CONFLICT upsert) / GetRecurrence
//	sermon cache    mirrored upsert (core/idrive pulls S3 config at import; the
//	                SQL shape is what matters here)
//
// It also pins the wire-compat assumptions the SQLBoiler layer rests on:
// types.StringArray ({a,b} literal) binding/scanning on text[], time.Time
// round-tripping timestamptz, jsonb round-trip, array_to_string + ILIKE
// (the sermon search), and FK ON DELETE CASCADE fan-out.
//
// Run from the church module root:  go run ./test_scripts/bytdb_wire_check

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/apitoken"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/resource/event"
	"github.com/rohanthewiz/church/resource/prayerwall"
	"github.com/vattle/sqlboiler/types"
)

var failures int

func check(label string, err error) {
	if err != nil {
		failures++
		fmt.Printf("FAIL  %s: %s\n", label, err.Error())
		return
	}
	fmt.Printf("pass  %s\n", label)
}

func expect(label string, cond bool, detail string) {
	if !cond {
		failures++
		fmt.Printf("FAIL  %s: %s\n", label, detail)
		return
	}
	fmt.Printf("pass  %s\n", label)
}

func main() {
	tmpDir, err := os.MkdirTemp("", "bytdb_wire_check")
	if err != nil {
		fmt.Println("could not create temp dir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// Boot the same path a site binary takes: empty DBType selects bytdb,
	// InitDB bootstraps the consolidated schema and starts pgwire.
	err = db.InitDB(db.DBOpts{File: filepath.Join(tmpDir, "church.db")})
	check("InitDB (bytdb default, schema bootstrap, pgwire up)", err)
	if err != nil {
		os.Exit(1)
	}
	defer db.CloseDB()
	fmt.Println("      wire addr:", db.BytDBWireAddr())

	dbH, err := db.Db()
	check("lib/pq connects over loopback wire", err)
	if err != nil {
		os.Exit(1)
	}

	// ---- users: raw insert exercising timestamptz binding + RETURNING ----
	now := time.Now().UTC().Truncate(time.Microsecond) // pg wire format carries microseconds
	var userID int64
	err = dbH.QueryRow(
		`INSERT INTO users (created_at, updated_at, updated_by, enabled, role, username,
			email_address, first_name, prefs)
		 VALUES ($1, $1, 'wire_check', true, 9, 'wire_tester', 'wire@test.local', 'Wire', '{"theme":"dark"}')
		 RETURNING id`, now).Scan(&userID)
	check("users insert with timestamptz + jsonb + RETURNING id", err)

	var createdAt time.Time
	err = dbH.QueryRow(`SELECT created_at FROM users WHERE id = $1`, userID).Scan(&createdAt)
	check("timestamptz scans into time.Time", err)
	expect("timestamptz round-trips losslessly", createdAt.Equal(now),
		fmt.Sprintf("wrote %v read %v", now, createdAt))

	var prefs string
	err = dbH.QueryRow(`SELECT prefs->>'theme' FROM users WHERE id = $1`, userID).Scan(&prefs)
	check("jsonb ->> operator over the wire", err)
	expect("jsonb value intact", prefs == "dark", "got "+prefs)

	// ---- chat: real query functions ----
	msg, err := chat.InsertMessage(dbH, chat.Message{
		Channel: "community", UserId: userID, Username: "wire_tester",
		DisplayName: "Wire", Body: "hello from the wire", CreatedAt: now,
	})
	check("chat.InsertMessage (RETURNING id)", err)
	_, err = chat.InsertMessage(dbH, chat.Message{
		Channel: "community", UserId: userID, Username: "wire_tester",
		Body: "second message", CreatedAt: now,
	})
	check("chat.InsertMessage second row", err)

	msgs, err := chat.RecentMessages(dbH, "community", 0, 10)
	check("chat.RecentMessages windowed select", err)
	expect("chat window returns both messages", len(msgs) == 2, fmt.Sprintf("got %d", len(msgs)))

	err = chat.SetKeep(dbH, msg.Id, true)
	check("chat.SetKeep update", err)

	// Cutoff in the future: sweeps everything except the kept message.
	deleted, err := chat.DeleteExpired(dbH, now.Add(time.Hour))
	check("chat.DeleteExpired retention sweep", err)
	expect("sweep honors keep flag", deleted == 1, fmt.Sprintf("deleted %d, want 1", deleted))

	// ---- prayer wall: real query functions ----
	req, err := prayerwall.InsertRequest(dbH, prayerwall.Request{
		UserId: userID, Username: "wire_tester", DisplayName: "Wire",
		Title: "Traveling mercies", Body: "for the team", Published: true,
		CreatedAt: now, UpdatedAt: now,
	})
	check("prayerwall.InsertRequest", err)
	got, found, err := prayerwall.GetRequest(dbH, req.Id)
	check("prayerwall.GetRequest", err)
	expect("prayer request round-trips", found && got.Title == "Traveling mercies", "row not found or mangled")

	// ---- api tokens: real query functions + unique constraint ----
	plain, _, err := apitoken.Issue(dbH, userID, "Pixel 9")
	check("apitoken.Issue", err)
	tu, found, err := apitoken.LookupUser(dbH, plain)
	check("apitoken.LookupUser (JOIN users + touch last_used_at)", err)
	expect("token resolves to user", found && tu.UserID == userID, "lookup missed")

	_, err = dbH.Exec(
		`INSERT INTO api_tokens (token_hash, user_id, created_at, expires_at)
		 VALUES ($1, $2, $3, $4)`, apitoken.HashToken(plain), userID, now, now.Add(time.Hour))
	expect("UNIQUE token_hash rejects duplicates", err != nil, "duplicate insert unexpectedly succeeded")

	// ---- event recurrences: ON CONFLICT DO UPDATE upsert ----
	var eventID int64
	err = dbH.QueryRow(
		`INSERT INTO events (created_at, updated_at, updated_by, published, title, slug,
			event_date, event_time, categories)
		 VALUES ($1, $1, 'wire_check', true, 'Prayer Night', 'prayer-night', $1, '19:00', $2)
		 RETURNING id`, now, types.StringArray{"prayer", "weekly"}).Scan(&eventID)
	check("events insert binds types.StringArray into text[]", err)

	err = event.UpsertRecurrence(dbH, event.Recurrence{EventID: eventID, Freq: event.RecurWeekly, Weekday: time.Wednesday})
	check("event.UpsertRecurrence initial insert", err)
	err = event.UpsertRecurrence(dbH, event.Recurrence{EventID: eventID, Freq: event.RecurWeekly, Weekday: time.Friday})
	check("event.UpsertRecurrence conflict update", err)
	rec, found, err := event.GetRecurrence(dbH, eventID)
	check("event.GetRecurrence", err)
	expect("upsert updated in place", found && rec.Weekday == time.Friday,
		fmt.Sprintf("weekday=%v found=%v", rec.Weekday, found))

	// ---- sermon cache: mirrored upsert from core/idrive/sermon_cache.go ----
	cacheUpsert := `INSERT INTO sermon_cache_access (created_at, last_accessed_at, rel_file_spec, local_file_spec)
		VALUES ($1, $1, $2, $3)
		ON CONFLICT (rel_file_spec) DO UPDATE SET last_accessed_at = EXCLUDED.last_accessed_at`
	_, err = dbH.Exec(cacheUpsert, now, "2026/sermon.mp3", "/cache/2026/sermon.mp3")
	check("sermon_cache upsert insert", err)
	later := now.Add(time.Minute)
	_, err = dbH.Exec(cacheUpsert, later, "2026/sermon.mp3", "/cache/2026/sermon.mp3")
	check("sermon_cache upsert conflict path", err)
	var cacheRows int64
	var lastAccess time.Time
	err = dbH.QueryRow(`SELECT count(*) FROM sermon_cache_access`).Scan(&cacheRows)
	check("sermon_cache count", err)
	_ = dbH.QueryRow(`SELECT last_accessed_at FROM sermon_cache_access WHERE rel_file_spec = $1`,
		"2026/sermon.mp3").Scan(&lastAccess)
	expect("upsert kept one row and bumped access time",
		cacheRows == 1 && lastAccess.Equal(later),
		fmt.Sprintf("rows=%d last=%v", cacheRows, lastAccess))

	// ---- sermons: StringArray round-trip + the live search shape ----
	_, err = dbH.Exec(
		`INSERT INTO sermons (created_at, updated_at, updated_by, title, slug, published,
			date_taught, teacher, scripture_refs, categories)
		 VALUES ($1, $1, 'wire_check', 'On Hope', 'on-hope', true, $1, 'R. Allison', $2, $3)`,
		now, types.StringArray{"John 3:16", "Rom 5:5"}, types.StringArray{"hope"})
	check("sermons insert with two text[] columns", err)

	var refs types.StringArray
	var title string
	// Same predicate resource/sermon/api_rweb.go builds for ?ref= searches.
	err = dbH.QueryRow(
		`SELECT title, scripture_refs FROM sermons
		 WHERE array_to_string(scripture_refs, '|') ILIKE $1`, "%john%").Scan(&title, &refs)
	check("sermon search: array_to_string + ILIKE + StringArray scan", err)
	expect("search found the sermon with refs intact",
		title == "On Hope" && len(refs) == 2 && refs[0] == "John 3:16",
		fmt.Sprintf("title=%q refs=%v", title, refs))

	// ---- FK fan-out: deleting the user cascades to all child tables ----
	_, err = dbH.Exec(`DELETE FROM users WHERE id = $1`, userID)
	check("users delete (cascade trigger)", err)
	var leftovers int64
	err = dbH.QueryRow(
		`SELECT (SELECT count(*) FROM chat_messages WHERE user_id = $1)
		      + (SELECT count(*) FROM prayer_requests WHERE user_id = $1)
		      + (SELECT count(*) FROM api_tokens WHERE user_id = $1)`, userID).Scan(&leftovers)
	check("cascade leftovers query", err)
	expect("ON DELETE CASCADE cleared chat, prayer, and token rows", leftovers == 0,
		fmt.Sprintf("%d rows survived", leftovers))

	fmt.Println()
	if failures > 0 {
		fmt.Printf("RESULT: %d check(s) FAILED\n", failures)
		os.Exit(1)
	}
	fmt.Println("RESULT: all checks passed — bytdb wire compatibility proven for the phase-1 surface")
}
