
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS articles (
  id BIGSERIAL PRIMARY KEY NOT NULL,
  created_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ,
  updated_by text NOT NULL, -- (username)
  title text NOT NULL,
  slug text NOT NULL,
  summary text NOT NULL,
  body text,
  published BOOLEAN NOT NULL,
  categories text[] NOT NULL
);
CREATE UNIQUE INDEX idx_articles_slug on articles(slug);
CREATE INDEX idx_articles_published on articles(published);
ALTER TABLE articles OWNER TO "devuser";

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE articles;
