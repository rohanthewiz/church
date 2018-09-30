
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- A Page is mostly a collection of modules
CREATE TABLE IF NOT EXISTS pages (
  id BIGSERIAL PRIMARY KEY NOT NULL,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  updated_by text NOT NULL, -- (username)
  title text NOT NULL,
  slug text NOT NULL,
  published BOOLEAN NOT NULL, -- 0 | 1
  is_home BOOLEAN NOT NULL, -- bool
  is_admin BOOLEAN NOT NULL, -- bool
  available_positions text[] NOT NULL,
  data jsonb -- modules
);
CREATE UNIQUE INDEX idx_pages_slug on pages(slug);
CREATE INDEX idx_pages_published on pages(published);
CREATE INDEX idx_pages_is_admin on pages(is_admin);
ALTER TABLE pages OWNER TO "devuser";

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE pages;
