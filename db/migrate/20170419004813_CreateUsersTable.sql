
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL primary key NOT NULL,
  created_at timestamptz,
  updated_at timestamptz,
  updated_by text NOT NULL, -- username - for accountability
  enabled BOOLEAN NOT NULL,
  role int NOT NULL, -- 99 - Super admin, 1 - admin, 5 - publisher, 7 - editor, 9 - registered_user
  username text NOT NULL,  -- (like a slug) could assist by auto creating based on first_name, last_name, checking for uniqueness
  email_address text NOT NULL,
  first_name text NOT NULL,
  last_name text,
  summary text,
  encrypted_password text,
  encrypted_salt text,
  -- moving this to Redis -- reset_password_token text,
  password_reset_at timestamptz,
  -- moving this to Reids (if gonna use it) -- confirmation_token text,
  -- moving this to Redis -- confirmation_token_at timestamptz,
  confirmed_at timestamptz,
  prefs jsonb
);
CREATE UNIQUE INDEX idx_users_username on users (username);
CREATE UNIQUE INDEX idx_users_email on users (email_address);
ALTER TABLE users OWNER TO "devuser";

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE users;
