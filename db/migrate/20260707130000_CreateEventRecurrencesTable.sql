-- +goose Up
-- Recurrence rule for an event, at most one per event (1:1). Kept in its own
-- table rather than as columns on events so the generated SQLBoiler events
-- model stays untouched (same approach as sermon_cache_access: hand-written
-- data access for post-SQLBoiler tables).
--
-- Supported rules:
--   freq='weekly'  : every <weekday>                 (week unused, 0)
--   freq='monthly' : the <week>th <weekday> of each month
--                    week 1..4 = first..fourth, -1 = last
-- The event's own event_date anchors the series: no occurrence is generated
-- before it, and the base row itself represents the first/only literal date.
CREATE TABLE IF NOT EXISTS event_recurrences (
    event_id   BIGINT PRIMARY KEY REFERENCES events (id) ON DELETE CASCADE,
    freq       text NOT NULL,
    weekday    smallint NOT NULL DEFAULT 0, -- 0=Sunday .. 6=Saturday (matches Go time.Weekday)
    week       smallint NOT NULL DEFAULT 0, -- monthly ordinal; 0 for weekly
    until      date,                        -- last date an occurrence may fall on; NULL = open-ended
    created_at timestamptz,
    updated_at timestamptz,
    CONSTRAINT chk_recur_freq    CHECK (freq IN ('weekly', 'monthly')),
    CONSTRAINT chk_recur_weekday CHECK (weekday BETWEEN 0 AND 6),
    CONSTRAINT chk_recur_week    CHECK (
        (freq = 'weekly'  AND week = 0) OR
        (freq = 'monthly' AND (week BETWEEN 1 AND 4 OR week = -1))
    )
);
ALTER TABLE event_recurrences OWNER TO "devuser";

-- +goose Down
DROP TABLE event_recurrences;
