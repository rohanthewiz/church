
-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
-- Tracks locally-cached sermon files downloaded from IDrive e2 so a background
-- process can evict (delete) stale local copies in LRU fashion once they have
-- not been accessed for a configured window (see core/idrive cleanup).
create table sermon_cache_access
(
	id bigserial not null
		constraint sermon_cache_access_pkey
			primary key,
	created_at timestamp with time zone,
	-- last_accessed_at is bumped on every serve (fresh download OR local cache hit)
	-- so frequently played sermons stay resident and only truly idle files are evicted.
	last_accessed_at timestamp with time zone not null,
	-- rel_file_spec is the IDrive object key (e.g. "2024/sermon.mp3"); it is the
	-- natural key for a cached sermon and is what we HeadObject against before eviction.
	rel_file_spec text not null,
	-- local_file_spec is the absolute/relative path of the cached copy on disk.
	local_file_spec text not null
);

alter table sermon_cache_access
  owner to devuser; -- be sure to change to the owner of the production DB

-- One row per cached object; the upsert on access relies on this uniqueness.
create unique index idx_sermon_cache_access_rel_file_spec
  on sermon_cache_access (rel_file_spec);

-- The cleanup scan filters/sorts on last_accessed_at.
create index idx_sermon_cache_access_last_accessed_at
  on sermon_cache_access (last_accessed_at);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE sermon_cache_access;
