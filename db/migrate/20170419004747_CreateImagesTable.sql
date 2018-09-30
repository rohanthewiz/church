
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
DROP TABLE IF EXISTS images;
CREATE TABLE IF NOT EXISTS images (
  id BIGSERIAL PRIMARY KEY NOT NULL,
  created_at timestamptz,
  updated_at timestamptz,
  updated_by text NOT NULL, -- (username)
  published BOOLEAN, -- 0 --disabled, 1 - enabled
  title text NOT NULL,
  slug text,  -- derive from caption - add digits if necessary to make unique
  summary text,
  categories text[],
  large_path text,
  small_b64 text,
  thumb_b64 text,
  image_type text
);
CREATE UNIQUE INDEX idx_images_slug on images (slug);
CREATE INDEX idx_images_published on images (published);
ALTER TABLE images OWNER TO "devuser";

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE images;
