-- schema_duckdb.sql
--
-- Single-file schema for a fresh DuckDB install of the church platform.
-- Consolidates the legacy goose migrations under db/migrate/*.sql.
--
-- Translation notes vs. the Postgres schema this replaces:
--   * BIGSERIAL           -> BIGINT PRIMARY KEY DEFAULT nextval('<table>_id_seq')
--                            (DuckDB has no SERIAL; we emit one sequence per table.)
--   * text[]              -> VARCHAR[]   (DuckDB's first-class list type)
--   * jsonb               -> JSON        (single JSON type in DuckDB)
--   * timestamptz         -> TIMESTAMPTZ (alias for TIMESTAMP WITH TIME ZONE)
--   * ALTER TABLE ... OWNER TO ... and GRANT statements are dropped
--     (DuckDB is file-backed and has no ownership/role concept).
--
-- Idempotency: every object uses IF NOT EXISTS so re-running the file
-- against an existing DuckDB database is a no-op. The connect path
-- runs this on every open; new objects pick up automatically while
-- existing ones are left untouched.
--
-- The legacy 'images' table from the Postgres migrations is intentionally
-- omitted: the current codebase has no DAO or reader for it. Image
-- handling lives in resource/chimage and operates on article HTML.

-- articles --------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS articles_id_seq START 1;
CREATE TABLE IF NOT EXISTS articles (
  id          BIGINT PRIMARY KEY DEFAULT nextval('articles_id_seq'),
  created_at  TIMESTAMPTZ,
  updated_at  TIMESTAMPTZ,
  updated_by  VARCHAR NOT NULL,
  title       VARCHAR NOT NULL,
  slug        VARCHAR NOT NULL,
  summary     VARCHAR NOT NULL,
  body        VARCHAR,
  published   BOOLEAN NOT NULL,
  categories  VARCHAR[] NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_slug      ON articles(slug);
CREATE INDEX        IF NOT EXISTS idx_articles_published ON articles(published);

-- events ----------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS events_id_seq START 1;
CREATE TABLE IF NOT EXISTS events (
  id              BIGINT PRIMARY KEY DEFAULT nextval('events_id_seq'),
  created_at      TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ,
  updated_by      VARCHAR NOT NULL,
  published       BOOLEAN NOT NULL,
  title           VARCHAR NOT NULL,
  slug            VARCHAR NOT NULL,
  summary         VARCHAR,
  body            VARCHAR,
  event_date      TIMESTAMPTZ NOT NULL,
  event_time      VARCHAR NOT NULL,
  event_location  VARCHAR,
  contact_person  VARCHAR,
  contact_phone   VARCHAR,
  contact_email   VARCHAR,
  contact_url     VARCHAR,
  categories      VARCHAR[] NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_events_slug       ON events(slug);
CREATE INDEX        IF NOT EXISTS idx_events_event_date ON events(event_date);
CREATE INDEX        IF NOT EXISTS idx_events_published  ON events(published);

-- users -----------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS users_id_seq START 1;
CREATE TABLE IF NOT EXISTS users (
  id                  BIGINT PRIMARY KEY DEFAULT nextval('users_id_seq'),
  created_at          TIMESTAMPTZ,
  updated_at          TIMESTAMPTZ,
  updated_by          VARCHAR NOT NULL,
  enabled             BOOLEAN NOT NULL,
  role                INTEGER NOT NULL,
  username            VARCHAR NOT NULL,
  email_address       VARCHAR NOT NULL,
  first_name          VARCHAR NOT NULL,
  last_name           VARCHAR,
  summary             VARCHAR,
  encrypted_password  VARCHAR,
  encrypted_salt      VARCHAR,
  password_reset_at   TIMESTAMPTZ,
  confirmed_at        TIMESTAMPTZ,
  prefs               JSON
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email    ON users(email_address);

-- pages -----------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS pages_id_seq START 1;
CREATE TABLE IF NOT EXISTS pages (
  id                   BIGINT PRIMARY KEY DEFAULT nextval('pages_id_seq'),
  created_at           TIMESTAMPTZ,
  updated_at           TIMESTAMPTZ,
  updated_by           VARCHAR NOT NULL,
  title                VARCHAR NOT NULL,
  slug                 VARCHAR NOT NULL,
  published            BOOLEAN NOT NULL,
  is_home              BOOLEAN NOT NULL,
  is_admin             BOOLEAN NOT NULL,
  available_positions  VARCHAR[] NOT NULL,
  data                 JSON
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pages_slug      ON pages(slug);
CREATE INDEX        IF NOT EXISTS idx_pages_published ON pages(published);
CREATE INDEX        IF NOT EXISTS idx_pages_is_admin  ON pages(is_admin);

-- sermons ---------------------------------------------------------------
-- date_taught was `timestamp` (no TZ) in the Postgres schema; preserved
-- as TIMESTAMP here so teaching-date display logic doesn't shift by TZ.
CREATE SEQUENCE IF NOT EXISTS sermons_id_seq START 1;
CREATE TABLE IF NOT EXISTS sermons (
  id              BIGINT PRIMARY KEY DEFAULT nextval('sermons_id_seq'),
  created_at      TIMESTAMPTZ,
  updated_at      TIMESTAMPTZ,
  updated_by      VARCHAR NOT NULL,
  title           VARCHAR NOT NULL,
  slug            VARCHAR,
  published       BOOLEAN NOT NULL,
  summary         VARCHAR,
  body            VARCHAR,
  audio_link      VARCHAR,
  date_taught     TIMESTAMP NOT NULL,
  place_taught    VARCHAR,
  teacher         VARCHAR NOT NULL,
  scripture_refs  VARCHAR[] NOT NULL,
  categories      VARCHAR[] NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sermons_slug ON sermons(slug);

-- menu_defs -------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS menu_defs_id_seq START 1;
CREATE TABLE IF NOT EXISTS menu_defs (
  id          BIGINT PRIMARY KEY DEFAULT nextval('menu_defs_id_seq'),
  created_at  TIMESTAMPTZ,
  updated_at  TIMESTAMPTZ,
  updated_by  VARCHAR NOT NULL,
  title       VARCHAR NOT NULL,
  slug        VARCHAR NOT NULL,
  published   BOOLEAN NOT NULL,
  is_admin    BOOLEAN NOT NULL,
  items       JSON
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_menu_defs_slug      ON menu_defs(slug);
CREATE INDEX        IF NOT EXISTS idx_menu_defs_published ON menu_defs(published);

-- charges ---------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS charges_id_seq START 1;
CREATE TABLE IF NOT EXISTS charges (
  id                BIGINT PRIMARY KEY DEFAULT nextval('charges_id_seq'),
  created_at        TIMESTAMPTZ,
  updated_at        TIMESTAMPTZ,
  customer_id       VARCHAR,
  customer_name     VARCHAR NOT NULL,
  customer_email    VARCHAR,
  description       VARCHAR,
  comment           VARCHAR,
  receipt_number    VARCHAR,
  receipt_url       VARCHAR,
  payment_token     VARCHAR NOT NULL,
  captured          BOOLEAN DEFAULT FALSE,
  paid              BOOLEAN DEFAULT FALSE,
  amount_paid       BIGINT,
  refunded          BOOLEAN DEFAULT FALSE,
  amount_refunded   BIGINT,
  meta              VARCHAR
);
CREATE INDEX IF NOT EXISTS idx_charges_created_at ON charges(created_at);
