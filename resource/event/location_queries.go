package event

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

// Hand-written SQL (no SQLBoiler model) — event_locations postdates the
// generated models, and regenerating with the legacy SQLBoiler v2 toolchain is
// riskier than a few explicit queries. Same approach as event_recurrences and
// sermon_cache_access.

// Point is where an event is held: a pair, always, or nothing.
//
// There is no Valid field and no pointer pair. The absence of a point is the
// absence of a row, which is why every function here returns a found bool
// beside the value — see the migration for why "both or neither" is a shape
// rather than a validation.
type Point struct {
	EventID   int64
	Latitude  float64
	Longitude float64
}

// Validate refuses a pair outside the addressable range. See the migration's
// CHECK constraints, which state the same rule to the database; this is the
// half that can say something useful to the admin who typed it.
func (p Point) Validate() error {
	if p.Latitude < -90 || p.Latitude > 90 {
		return serr.New("latitude must be between -90 and 90",
			"latitude", strconv.FormatFloat(p.Latitude, 'f', -1, 64))
	}
	if p.Longitude < -180 || p.Longitude > 180 {
		return serr.New("longitude must be between -180 and 180",
			"longitude", strconv.FormatFloat(p.Longitude, 'f', -1, 64))
	}
	return nil
}

// GetEventPoint loads one event's point. found=false (no error) when the event
// has none, which is the common case — most events are at the church, and the
// church's own point is site configuration rather than per-event data.
func GetEventPoint(exec db.Executor, eventID int64) (pt Point, found bool, err error) {
	row := exec.QueryRow(
		`SELECT event_id, latitude, longitude FROM event_locations WHERE event_id = $1`, eventID)
	err = row.Scan(&pt.EventID, &pt.Latitude, &pt.Longitude)
	if err == sql.ErrNoRows {
		return pt, false, nil
	}
	if err != nil {
		return pt, false, serr.Wrap(err, "error loading event location")
	}
	return pt, true, nil
}

// EventPoints loads the points for a set of events in one query, keyed by
// event id. Events with no point are simply absent from the map.
//
// One query rather than one per event because the caller is a list response:
// WindowedEvents expands a recurring series into an occurrence per date, so a
// per-row lookup would be N+1 against N that is not even the number of events.
// An empty or nil id slice returns an empty map without touching the database.
//
// The id list is interpolated rather than parameterised, which is safe here in
// the one way that matters: every element is an int64 the caller already holds
// as a number, formatted by strconv, so nothing that is not a decimal integer
// can reach the string. A variadic IN would need a driver-specific array type
// or a generated placeholder list, and this path is shared by two backends.
func EventPoints(exec db.Executor, eventIDs []int64) (map[int64]Point, error) {
	points := make(map[int64]Point, len(eventIDs))
	if len(eventIDs) == 0 {
		return points, nil
	}

	// Deduplicated, because an expanded recurring series carries the base
	// event's id once per occurrence.
	seen := make(map[int64]bool, len(eventIDs))
	ids := make([]string, 0, len(eventIDs))
	for _, id := range eventIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, strconv.FormatInt(id, 10))
	}

	rows, err := exec.Query(
		`SELECT event_id, latitude, longitude FROM event_locations WHERE event_id IN (` +
			strings.Join(ids, ",") + `)`)
	if err != nil {
		return points, serr.Wrap(err, "error loading event locations")
	}
	defer rows.Close()

	for rows.Next() {
		var pt Point
		if err := rows.Scan(&pt.EventID, &pt.Latitude, &pt.Longitude); err != nil {
			return points, serr.Wrap(err, "error scanning event location")
		}
		points[pt.EventID] = pt
	}
	if err := rows.Err(); err != nil {
		return points, serr.Wrap(err, "error reading event locations")
	}
	return points, nil
}

// UpsertEventPoint writes an event's point (insert or replace — one point per
// event).
func UpsertEventPoint(exec db.Executor, pt Point) error {
	if err := pt.Validate(); err != nil {
		return err
	}
	now := time.Now()
	_, err := exec.Exec(`
		INSERT INTO event_locations (event_id, latitude, longitude, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (event_id)
		DO UPDATE SET latitude = $2, longitude = $3, updated_at = $4`,
		pt.EventID, pt.Latitude, pt.Longitude, now)
	if err != nil {
		return serr.Wrap(err, "error saving event location", "event_id",
			strconv.FormatInt(pt.EventID, 10))
	}
	return nil
}

// DeleteEventPoint removes an event's point. Deleting one that is not there is
// not an error: this is what an admin clearing both form fields means, and the
// common case is that there was nothing to clear.
func DeleteEventPoint(exec db.Executor, eventID int64) error {
	_, err := exec.Exec(`DELETE FROM event_locations WHERE event_id = $1`, eventID)
	if err != nil {
		return serr.Wrap(err, "error deleting event location", "event_id",
			strconv.FormatInt(eventID, 10))
	}
	return nil
}
