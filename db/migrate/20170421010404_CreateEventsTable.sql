
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS events (
    id BIGSERIAL PRIMARY KEY NOT NULL,
    created_at timestamptz,
    updated_at timestamptz,
    updated_by text NOT NULL,
    published BOOLEAN NOT NULL,  -- 0 - not published, 1 - published Todo - NOT NULL
    title text NOT NULL,
    slug text NOT NULL,
    summary text,
    body text,
    event_date timestamptz NOT NULL, -- todo NOT NULL,
    event_time text NOT NULL,
    event_location text,
    contact_person text,
    contact_phone text,
    contact_email text,
    contact_url text,
    -- author_id BIGINT NOT NULL,
    categories text[] NOT NULL
    -- foreign key(author_id) references users (id)
);
CREATE UNIQUE INDEX idx_events_slug on events (slug);
CREATE INDEX idx_events_event_date on events (event_date);
CREATE INDEX idx_events_published on events (published);
ALTER TABLE events OWNER TO "devuser";


-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE events;
