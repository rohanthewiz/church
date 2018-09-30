
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS event_categories (
  id BIGSERIAL PRIMARY KEY NOT NULL,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  name text,
  published BOOLEAN  --  0 - disabled,  1 - enabled
);
CREATE INDEX idx_event_categories_name on event_categories (name);
ALTER TABLE users OWNER TO "devuser";

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE event_categories;
