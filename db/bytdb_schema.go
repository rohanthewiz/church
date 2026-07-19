package db

import (
	bsql "github.com/rohanthewiz/bytdb/sql"
	"github.com/rohanthewiz/serr"
)

// Consolidated schema for the bytdb backend.
//
// The goose migration chain stays authoritative for the Postgres
// fallback, but bytdb boots from this single in-code definition
// instead: an embedded database that creates its own schema on first
// open needs no external migration tool, and bytdb's parser takes this
// dialect directly (no OWNER TO, no IF NOT EXISTS). The definitions
// below are the goose chain flattened to its net result, with two
// deliberate deviations:
//
//   - goose_db_version is omitted — it only tracks goose itself.
//   - user_id FK columns on chat_messages / prayer_requests gain
//     indexes the Postgres schema lacks: bytdb's ON DELETE CASCADE
//     probes child tables per deleted parent key, which is a full scan
//     without an index on the child column.
//   - the event_recurrences CHECKs use explicit >= / <= comparisons:
//     bytdb (as of v0.6.1) does not parse BETWEEN inside a CHECK
//     expression. Same semantics as the goose original.
//
// Future schema changes on the bytdb path: append a new tableDef (new
// table) or a guarded ALTER here. Note bytdb executes DDL outside
// transaction blocks, and ALTER TABLE ADD COLUMN with DEFAULT/NOT NULL
// requires an empty table — on live data, add the column nullable and
// backfill.
type tableDef struct {
	name string
	ddl  []string // CREATE TABLE first, then its indexes
}

// Creation order respects FK dependencies: users and events must exist
// before the tables that reference them.
var bytdbTables = []tableDef{
	{name: "users", ddl: []string{
		`CREATE TABLE users (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			updated_by text NOT NULL,
			enabled boolean NOT NULL,
			role int NOT NULL,
			username text NOT NULL,
			email_address text NOT NULL,
			first_name text NOT NULL,
			last_name text,
			summary text,
			encrypted_password text,
			encrypted_salt text,
			password_reset_at timestamptz,
			confirmed_at timestamptz,
			prefs jsonb
		)`,
		`CREATE UNIQUE INDEX idx_users_username ON users (username)`,
		`CREATE UNIQUE INDEX idx_users_email ON users (email_address)`,
	}},
	{name: "images", ddl: []string{
		`CREATE TABLE images (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			updated_by text NOT NULL,
			published boolean,
			title text NOT NULL,
			slug text,
			summary text,
			categories text[],
			large_path text,
			small_b64 text,
			thumb_b64 text,
			image_type text
		)`,
		`CREATE UNIQUE INDEX idx_images_slug ON images (slug)`,
		`CREATE INDEX idx_images_published ON images (published)`,
	}},
	{name: "events", ddl: []string{
		`CREATE TABLE events (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			updated_by text NOT NULL,
			published boolean NOT NULL,
			title text NOT NULL,
			slug text NOT NULL,
			summary text,
			body text,
			event_date timestamptz NOT NULL,
			event_time text NOT NULL,
			event_location text,
			contact_person text,
			contact_phone text,
			contact_email text,
			contact_url text,
			categories text[] NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_events_slug ON events (slug)`,
		`CREATE INDEX idx_events_event_date ON events (event_date)`,
		`CREATE INDEX idx_events_published ON events (published)`,
	}},
	{name: "pages", ddl: []string{
		`CREATE TABLE pages (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			updated_by text NOT NULL,
			title text NOT NULL,
			slug text NOT NULL,
			published boolean NOT NULL,
			is_home boolean NOT NULL,
			is_admin boolean NOT NULL,
			available_positions text[] NOT NULL,
			data jsonb
		)`,
		`CREATE UNIQUE INDEX idx_pages_slug ON pages (slug)`,
		`CREATE INDEX idx_pages_published ON pages (published)`,
		`CREATE INDEX idx_pages_is_admin ON pages (is_admin)`,
	}},
	{name: "articles", ddl: []string{
		`CREATE TABLE articles (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			updated_by text NOT NULL,
			title text NOT NULL,
			slug text NOT NULL,
			summary text NOT NULL,
			body text,
			published boolean NOT NULL,
			categories text[] NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_articles_slug ON articles (slug)`,
		`CREATE INDEX idx_articles_published ON articles (published)`,
	}},
	{name: "sermons", ddl: []string{
		`CREATE TABLE sermons (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			updated_by text NOT NULL,
			title text NOT NULL,
			slug text,
			published boolean NOT NULL,
			summary text,
			body text,
			audio_link text,
			date_taught timestamp NOT NULL,
			place_taught text,
			teacher text NOT NULL,
			scripture_refs text[] NOT NULL,
			categories text[] NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_sermons_slug ON sermons (slug)`,
	}},
	{name: "menu_defs", ddl: []string{
		`CREATE TABLE menu_defs (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			updated_by text NOT NULL,
			title text NOT NULL,
			slug text NOT NULL,
			published boolean NOT NULL,
			is_admin boolean NOT NULL,
			items jsonb
		)`,
		`CREATE UNIQUE INDEX idx_menu_defs_slug ON menu_defs (slug)`,
		`CREATE INDEX idx_menu_defs_published ON menu_defs (published)`,
	}},
	{name: "charges", ddl: []string{
		`CREATE TABLE charges (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			updated_at timestamptz,
			customer_id text,
			customer_name text NOT NULL,
			customer_email text,
			description text,
			comment text,
			receipt_number text,
			receipt_url text,
			payment_token text NOT NULL,
			captured boolean DEFAULT false,
			paid boolean DEFAULT false,
			amount_paid bigint,
			refunded boolean DEFAULT false,
			amount_refunded bigint,
			meta text
		)`,
		`CREATE INDEX idx_created_at ON charges (created_at)`,
	}},
	{name: "sermon_cache_access", ddl: []string{
		`CREATE TABLE sermon_cache_access (
			id bigserial PRIMARY KEY,
			created_at timestamptz,
			last_accessed_at timestamptz NOT NULL,
			rel_file_spec text NOT NULL,
			local_file_spec text NOT NULL
		)`,
		`CREATE UNIQUE INDEX idx_sermon_cache_access_rel_file_spec ON sermon_cache_access (rel_file_spec)`,
		`CREATE INDEX idx_sermon_cache_access_last_accessed_at ON sermon_cache_access (last_accessed_at)`,
	}},
	{name: "event_recurrences", ddl: []string{
		`CREATE TABLE event_recurrences (
			event_id bigint PRIMARY KEY REFERENCES events (id) ON DELETE CASCADE,
			freq text NOT NULL,
			weekday smallint NOT NULL DEFAULT 0,
			week smallint NOT NULL DEFAULT 0,
			until date,
			created_at timestamptz,
			updated_at timestamptz,
			CONSTRAINT chk_recur_freq CHECK (freq IN ('weekly', 'monthly')),
			CONSTRAINT chk_recur_weekday CHECK (weekday >= 0 AND weekday <= 6),
			CONSTRAINT chk_recur_week CHECK (
				(freq = 'weekly' AND week = 0) OR
				(freq = 'monthly' AND ((week >= 1 AND week <= 4) OR week = -1))
			)
		)`,
	}},
	{name: "api_tokens", ddl: []string{
		`CREATE TABLE api_tokens (
			id bigserial PRIMARY KEY,
			token_hash text NOT NULL UNIQUE,
			user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			device text NOT NULL DEFAULT '',
			created_at timestamptz NOT NULL,
			last_used_at timestamptz,
			expires_at timestamptz NOT NULL
		)`,
		`CREATE INDEX idx_api_tokens_user_id ON api_tokens (user_id)`,
	}},
	{name: "chat_messages", ddl: []string{
		`CREATE TABLE chat_messages (
			id bigserial PRIMARY KEY,
			channel text NOT NULL,
			user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			username text NOT NULL,
			display_name text NOT NULL DEFAULT '',
			body text NOT NULL,
			keep boolean NOT NULL DEFAULT false,
			created_at timestamptz NOT NULL
		)`,
		`CREATE INDEX idx_chat_messages_channel_id ON chat_messages (channel, id)`,
		`CREATE INDEX idx_chat_messages_created_at ON chat_messages (created_at)`,
		`CREATE INDEX idx_chat_messages_user_id ON chat_messages (user_id)`,
	}},
	{name: "prayer_requests", ddl: []string{
		`CREATE TABLE prayer_requests (
			id bigserial PRIMARY KEY,
			user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
			username text NOT NULL,
			display_name text NOT NULL DEFAULT '',
			title text NOT NULL,
			body text NOT NULL,
			answered boolean NOT NULL DEFAULT false,
			answered_note text NOT NULL DEFAULT '',
			published boolean NOT NULL DEFAULT true,
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL
		)`,
		`CREATE INDEX idx_prayer_requests_created_at ON prayer_requests (created_at)`,
		`CREATE INDEX idx_prayer_requests_user_id ON prayer_requests (user_id)`,
	}},
}

// ensureBytDBSchema creates any missing tables (with their indexes) on
// the embedded handle, before the wire listener starts. Existence is
// checked per table rather than via one sentinel so a schema that grows
// a new tableDef in a later release gets just that table created on
// upgrade. A table found present is assumed complete — table + indexes
// are created back-to-back on a WAL that fsyncs each DDL, so a torn
// bootstrap would take a crash inside a first boot's few milliseconds;
// recovery is deleting the data file of that failed first boot.
func ensureBytDBSchema(bdb *bsql.DB) error {
	for _, t := range bytdbTables {
		exists, err := bytdbTableExists(bdb, t.name)
		if err != nil {
			return serr.Wrap(err, "checking table existence", "table", t.name)
		}
		if exists {
			continue
		}
		for _, stmt := range t.ddl {
			if _, err := bdb.Exec(stmt); err != nil {
				return serr.Wrap(err, "creating schema object", "table", t.name)
			}
		}
	}
	return nil
}

func bytdbTableExists(bdb *bsql.DB, name string) (bool, error) {
	res, err := bdb.Exec(
		`SELECT count(*) FROM information_schema.tables WHERE table_name = $1`, name)
	if err != nil {
		return false, serr.Wrap(err)
	}
	if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
		return false, serr.New("empty result from information_schema.tables count")
	}
	count, ok := res.Rows[0][0].(int64)
	if !ok {
		return false, serr.New("unexpected count type from information_schema query")
	}
	return count > 0, nil
}
