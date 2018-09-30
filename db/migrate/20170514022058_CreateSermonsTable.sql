
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS sermons (
  id BIGSERIAL PRIMARY KEY NOT NULL,
  created_at timestamptz,
  updated_at timestamptz,
  updated_by text NOT NULL, -- (username)
  title text NOT NULL,
  slug text,
  published BOOLEAN NOT NULL,
  summary text,
  body text,
  audio_link text,
  date_taught timestamp NOT NULL,
  place_taught text,
  teacher text NOT NULL,
  scripture_refs text[] NOT NULL,
  categories text[] NOT NULL
  --FOREIGN KEY (author_id) REFERENCES users (id)
);
CREATE UNIQUE INDEX idx_sermons_slug on sermons(slug);
ALTER TABLE sermons OWNER TO "devuser";

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE sermons;
