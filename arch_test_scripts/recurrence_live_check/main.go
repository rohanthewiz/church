// Live smoke test for event recurrence expansion against a local dev DB.
// Seeds one weekly, one monthly-last, and one one-time event, then prints
// what WindowedEvents expands them to for Jul-Oct 2026.
//
// Run: go run ./test_scripts/recurrence_live_check
// Requires: local Postgres with church_development migrated (goose up).
package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/resource/event"
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

	// Clean any prior run, then seed
	defer cleanup(dbH)
	cleanup(dbH)

	seed := func(title, slug, date string) (id int64) {
		err := dbH.QueryRow(`INSERT INTO events (updated_by, published, title, slug, event_date, event_time, categories)
			VALUES ('smoke', true, $1, $2, $3, '10:00 AM', '{}') RETURNING id`, title, slug, date).Scan(&id)
		if err != nil {
			log.Fatal(err)
		}
		return id
	}

	weeklyID := seed("Sunday Service", "smoke-weekly", "2026-07-12")
	monthlyID := seed("Men's Breakfast (last Sunday)", "smoke-monthly", "2026-07-26")
	seed("Church Picnic", "smoke-onetime", "2026-08-15")

	must(event.UpsertRecurrence(dbH, event.Recurrence{
		EventID: weeklyID, Freq: event.RecurWeekly, Weekday: time.Sunday,
		Until: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC), // series ends Sep 30
	}))
	must(event.UpsertRecurrence(dbH, event.Recurrence{
		EventID: monthlyID, Freq: event.RecurMonthly, Weekday: time.Sunday, Week: event.RecurWeekLast,
	}))

	from := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	events, err := event.WindowedEvents(from, to)
	must(err)

	fmt.Printf("\n%-12s  %-30s  recurring  desc\n", "date", "title")
	for _, e := range events {
		fmt.Printf("%-12s  %-30s  %-9v  %s\n", e.EventDate, e.Title, e.Recurring, e.RecurrenceDesc)
	}
	fmt.Printf("\n%d total entries\n", len(events))
}

// cleanup removes seeded rows; event_recurrences rows cascade with the events
func cleanup(dbH *sql.DB) {
	if _, err := dbH.Exec(`DELETE FROM events WHERE slug LIKE 'smoke-%'`); err != nil {
		log.Println("cleanup:", err)
	}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
