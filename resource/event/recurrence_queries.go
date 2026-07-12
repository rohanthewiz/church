package event

import (
	"database/sql"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

// Hand-written SQL (no SQLBoiler model) — event_recurrences postdates the
// generated models, and regenerating with the legacy SQLBoiler v2 toolchain
// is riskier than a few explicit queries. Same approach as sermon_cache_access.

// GetRecurrence loads an event's rule. found=false (no error) when the event
// simply doesn't recur — the common case.
func GetRecurrence(exec db.Executor, eventID int64) (rec Recurrence, found bool, err error) {
	var weekday int
	var until sql.NullTime
	row := exec.QueryRow(
		`SELECT event_id, freq, weekday, week, until FROM event_recurrences WHERE event_id = $1`, eventID)
	err = row.Scan(&rec.EventID, &rec.Freq, &weekday, &rec.Week, &until)
	if err == sql.ErrNoRows {
		return rec, false, nil
	}
	if err != nil {
		return rec, false, serr.Wrap(err, "error loading event recurrence")
	}
	rec.Weekday = time.Weekday(weekday)
	if until.Valid {
		rec.Until = until.Time
	}
	return rec, true, nil
}

// UpsertRecurrence writes an event's rule (insert or replace — one rule per event).
func UpsertRecurrence(exec db.Executor, rec Recurrence) error {
	if err := rec.Validate(); err != nil {
		return err
	}

	var until any // nil -> SQL NULL for open-ended series
	if !rec.Until.IsZero() {
		until = rec.Until
	}
	now := time.Now()
	_, err := exec.Exec(`
		INSERT INTO event_recurrences (event_id, freq, weekday, week, until, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (event_id)
		DO UPDATE SET freq = EXCLUDED.freq, weekday = EXCLUDED.weekday,
		              week = EXCLUDED.week, until = EXCLUDED.until,
		              updated_at = EXCLUDED.updated_at`,
		rec.EventID, rec.Freq, int(rec.Weekday), rec.Week, until, now)
	if err != nil {
		return serr.Wrap(err, "error upserting event recurrence")
	}
	return nil
}

// DeleteRecurrence removes an event's rule (making it one-time again).
// Deleting the event itself cascades via the FK, so this is only needed when
// an admin switches recurrence back to "None".
func DeleteRecurrence(exec db.Executor, eventID int64) error {
	if _, err := exec.Exec(`DELETE FROM event_recurrences WHERE event_id = $1`, eventID); err != nil {
		return serr.Wrap(err, "error deleting event recurrence")
	}
	return nil
}

// allRecurrences returns every rule. The table is tiny at church scale (one
// row per repeating event), so window expansion just loads them all rather
// than pushing date logic into SQL.
func allRecurrences(exec db.Executor) (recs []Recurrence, err error) {
	rows, err := exec.Query(`SELECT event_id, freq, weekday, week, until FROM event_recurrences`)
	if err != nil {
		return nil, serr.Wrap(err, "error loading event recurrences")
	}
	defer rows.Close()

	for rows.Next() {
		var rec Recurrence
		var weekday int
		var until sql.NullTime
		if err = rows.Scan(&rec.EventID, &rec.Freq, &weekday, &rec.Week, &until); err != nil {
			return nil, serr.Wrap(err, "error scanning event recurrence")
		}
		rec.Weekday = time.Weekday(weekday)
		if until.Valid {
			rec.Until = until.Time
		}
		recs = append(recs, rec)
	}
	if err = rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error iterating event recurrences")
	}
	return recs, nil
}
