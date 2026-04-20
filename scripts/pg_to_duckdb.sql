-- pg_to_duckdb.sql
--
-- One-shot data migration from the legacy Postgres database into a fresh
-- DuckDB file. Runs inside a DuckDB CLI session (not psql).
--
-- Prerequisites
--   1. The DuckDB file has already been initialised with
--      db/schema_duckdb.sql — the app does this automatically on first
--      open, or you can run it manually:
--          duckdb church.duckdb < db/schema_duckdb.sql
--   2. The Postgres database is reachable from wherever you run duckdb.
--      The `postgres` extension pulls data over the wire.
--
-- How to run
--   duckdb church.duckdb < scripts/pg_to_duckdb.sql
--
-- Editing
--   Update the connection string in the ATTACH statement below to match
--   your environment (user / password / host / db). This script is
--   intentionally concrete — a single cut-over, not a general tool.
--
-- Column-order contract
--   Each INSERT uses an explicit column list that matches
--   schema_duckdb.sql. The `id` column is included so sequence values
--   are preserved verbatim — no re-numbering. sequences are then bumped
--   past MAX(id) so the next INSERT from the app keeps going without
--   colliding with legacy rows.

INSTALL postgres;
LOAD postgres;

-- Edit this line for your Postgres DSN.
ATTACH 'postgresql://devuser:secret@localhost/church_development' AS pg (TYPE postgres, READ_ONLY);

-- ----------------------------------------------------------------------
-- Table copies. Column lists are explicit so a future schema addition in
-- one side (new column, reordering) fails loudly instead of silently
-- shifting data into the wrong column.
-- ----------------------------------------------------------------------

INSERT INTO articles
  (id, created_at, updated_at, updated_by, title, slug, summary, body, published, categories)
SELECT
   id, created_at, updated_at, updated_by, title, slug, summary, body, published, categories
FROM pg.public.articles;

INSERT INTO events
  (id, created_at, updated_at, updated_by, published, title, slug,
   summary, body, event_date, event_time, event_location,
   contact_person, contact_phone, contact_email, contact_url, categories)
SELECT
   id, created_at, updated_at, updated_by, published, title, slug,
   summary, body, event_date, event_time, event_location,
   contact_person, contact_phone, contact_email, contact_url, categories
FROM pg.public.events;

INSERT INTO users
  (id, created_at, updated_at, updated_by, enabled, role, username, email_address,
   first_name, last_name, summary, encrypted_password, encrypted_salt,
   password_reset_at, confirmed_at, prefs)
SELECT
   id, created_at, updated_at, updated_by, enabled, role, username, email_address,
   first_name, last_name, summary, encrypted_password, encrypted_salt,
   password_reset_at, confirmed_at, prefs
FROM pg.public.users;

INSERT INTO pages
  (id, created_at, updated_at, updated_by, title, slug,
   published, is_home, is_admin, available_positions, data)
SELECT
   id, created_at, updated_at, updated_by, title, slug,
   published, is_home, is_admin, available_positions, data
FROM pg.public.pages;

INSERT INTO sermons
  (id, created_at, updated_at, updated_by, title, slug, published,
   summary, body, audio_link, date_taught, place_taught, teacher,
   scripture_refs, categories)
SELECT
   id, created_at, updated_at, updated_by, title, slug, published,
   summary, body, audio_link, date_taught, place_taught, teacher,
   scripture_refs, categories
FROM pg.public.sermons;

INSERT INTO menu_defs
  (id, created_at, updated_at, updated_by, title, slug, published, is_admin, items)
SELECT
   id, created_at, updated_at, updated_by, title, slug, published, is_admin, items
FROM pg.public.menu_defs;

INSERT INTO charges
  (id, created_at, updated_at, customer_id, customer_name, customer_email,
   description, comment, receipt_number, receipt_url, payment_token,
   captured, paid, amount_paid, refunded, amount_refunded, meta)
SELECT
   id, created_at, updated_at, customer_id, customer_name, customer_email,
   description, comment, receipt_number, receipt_url, payment_token,
   captured, paid, amount_paid, refunded, amount_refunded, meta
FROM pg.public.charges;

-- ----------------------------------------------------------------------
-- Advance each sequence past the max legacy id so the next app INSERT
-- starts one step ahead. COALESCE handles the empty-table case.
-- ----------------------------------------------------------------------

SELECT setval('articles_id_seq',  (SELECT COALESCE(MAX(id), 0) FROM articles));
SELECT setval('events_id_seq',    (SELECT COALESCE(MAX(id), 0) FROM events));
SELECT setval('users_id_seq',     (SELECT COALESCE(MAX(id), 0) FROM users));
SELECT setval('pages_id_seq',     (SELECT COALESCE(MAX(id), 0) FROM pages));
SELECT setval('sermons_id_seq',   (SELECT COALESCE(MAX(id), 0) FROM sermons));
SELECT setval('menu_defs_id_seq', (SELECT COALESCE(MAX(id), 0) FROM menu_defs));
SELECT setval('charges_id_seq',   (SELECT COALESCE(MAX(id), 0) FROM charges));

DETACH pg;
