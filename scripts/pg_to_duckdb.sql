-- pg_to_duckdb.sql
--
-- One-shot data migration from the legacy Postgres database into a fresh
-- DuckDB file. Designed to run via the /super/pg-to-duckdb endpoint
-- (admin_controller.MigratePgToDuckDBRWeb), which substitutes the
-- {{PG_DSN}} placeholder below with the Postgres DSN from the request
-- and then feeds the script to db.ExecScript.
--
-- It can also be run by hand from the duckdb CLI in a pinch — replace
-- the {{PG_DSN}} marker with a literal connection string first:
--
--   sed 's|{{PG_DSN}}|postgresql://devuser:secret@localhost/church_development|' \
--     scripts/pg_to_duckdb.sql | duckdb church.duckdb
--
-- Idempotency / one-shot guarantee
--   The endpoint refuses to run if `migration_state` already contains a
--   row with name = 'pg_to_duckdb'. The final statement here records
--   that marker, so the migration is durably "done" once it succeeds.
--   On partial failure (any statement above the marker errors out) the
--   marker is *not* written, so a retry is allowed — TRUNCATEs at the
--   top of every table block ensure the retry starts from a clean slate
--   instead of double-inserting rows that the previous attempt already
--   loaded.
--
-- Column-order contract
--   Each INSERT uses an explicit column list that matches
--   schema_duckdb.sql. The `id` column is included so sequence values
--   are preserved verbatim — no re-numbering. Sequences are then bumped
--   past MAX(id) so the next INSERT from the app keeps going without
--   colliding with legacy rows.

INSTALL postgres;
LOAD postgres;

-- {{PG_DSN}} is substituted at runtime by the migration endpoint.
ATTACH '{{PG_DSN}}' AS pg (TYPE postgres, READ_ONLY);

-- ----------------------------------------------------------------------
-- Wipe each target table before reloading. We do this rather than
-- assume an empty DB so the migration can be re-attempted after a
-- partial failure without manual cleanup. The marker-row check at the
-- handler layer prevents this from ever clobbering live data after the
-- cut-over has been completed once.
-- ----------------------------------------------------------------------

TRUNCATE articles;
TRUNCATE events;
TRUNCATE users;
TRUNCATE pages;
TRUNCATE sermons;
TRUNCATE menu_defs;
TRUNCATE charges;

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

-- ----------------------------------------------------------------------
-- Final marker. Reaching this statement means every step above
-- succeeded — the row is the durable "do not run again" signal that the
-- handler checks before allowing another invocation.
-- ----------------------------------------------------------------------

INSERT INTO migration_state (name, completed_at) VALUES ('pg_to_duckdb', now());
