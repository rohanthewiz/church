
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- A Page is mostly a collection of modules
CREATE TABLE IF NOT EXISTS menu_defs (
  id BIGSERIAL PRIMARY KEY NOT NULL,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  updated_by text NOT NULL, -- (username)
  title text NOT NULL,
  slug text NOT NULL,
  published BOOLEAN NOT NULL, -- 0 | 1
  is_admin BOOLEAN NOT NULL, -- bool
  items jsonb
);

CREATE UNIQUE INDEX idx_menu_defs_slug on menu_defs(slug);
CREATE INDEX idx_menu_defs_published on menu_defs(published);
ALTER TABLE menu_defs OWNER TO "devuser";

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE menu_defs;
