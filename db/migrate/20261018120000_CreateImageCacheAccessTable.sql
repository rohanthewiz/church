-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Tracks locally-cached article images (dist/img/) fetched from, or uploaded
-- to, IDrive e2, so the background media cache cleanup can evict copies that
-- have not been requested within the idle window (see core/idrive
-- image_cache.go). Same shape as sermon_cache_access; one table per media kind
-- keeps the admin Sermon Cleanup listing sermon-only.
create table if not exists image_cache_access
(
	id bigserial not null
		constraint image_cache_access_pkey
			primary key,
	created_at timestamp with time zone,
	-- last_accessed_at is bumped on upload, on fetch from e2, and on local cache hits.
	last_accessed_at timestamp with time zone not null,
	-- rel_file_spec is the IDrive object key (e.g. "images/photo.png.1a2b.png");
	-- eviction HEADs it before deleting the local copy.
	rel_file_spec text not null,
	-- local_file_spec is the path of the cached copy on disk (e.g. "dist/img/photo.png.1a2b.png").
	local_file_spec text not null
);

alter table image_cache_access
  owner to devuser; -- be sure to change to the owner of the production DB

-- One row per cached object; the upsert on access relies on this uniqueness.
create unique index if not exists idx_image_cache_access_rel_file_spec
  on image_cache_access (rel_file_spec);

-- The cleanup scan filters/sorts on last_accessed_at.
create index if not exists idx_image_cache_access_last_accessed_at
  on image_cache_access (last_accessed_at);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE image_cache_access;
