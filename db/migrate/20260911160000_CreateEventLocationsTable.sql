-- +goose Up
-- The point an event is held at, at most one per event (1:1). Its own table
-- rather than columns on events for the reason event_recurrences gives: the
-- generated SQLBoiler events model stays untouched, and hand-written data
-- access is cheaper than regenerating with the legacy toolchain.
--
-- The 1:1 shape also states the rule that two nullable columns could only
-- check: a coordinate is a PAIR. One number without the other is a half-typed
-- form and not a place, and here that is unrepresentable rather than merely
-- rejected — a row exists or it does not.
--
-- Both columns are double precision and NOT NULL for the same reason. There is
-- no "unset" coordinate to spell: 0,0 is the Gulf of Guinea, a real place a map
-- will happily draw, so absence has to be carried by something other than the
-- numbers. Row presence is that something.
--
-- The range CHECKs are typo checks, not projections. A latitude of 300 is a
-- mistyped 30, and there is nothing useful to do with it: clamping moves the
-- event to the pole and passing it through draws somebody else's map. Longitude
-- is checked rather than wrapped even though wrapping is the correct operation
-- on a circle — wrapping is right for a number that arrived from a computation,
-- refusing is right for a number somebody typed.
CREATE TABLE IF NOT EXISTS event_locations (
    event_id   BIGINT PRIMARY KEY REFERENCES events (id) ON DELETE CASCADE,
    latitude   double precision NOT NULL,
    longitude  double precision NOT NULL,
    created_at timestamptz,
    updated_at timestamptz,
    CONSTRAINT chk_event_lat CHECK (latitude BETWEEN -90 AND 90),
    CONSTRAINT chk_event_lng CHECK (longitude BETWEEN -180 AND 180)
);
ALTER TABLE event_locations OWNER TO "devuser";

-- +goose Down
DROP TABLE event_locations;
