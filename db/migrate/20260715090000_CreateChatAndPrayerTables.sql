-- +goose Up
-- Live chat messages. A single table serves every placement of the chat
-- module — top-level chat page, prayer-wall discussion, per-article comments —
-- distinguished by the channel key (e.g. "community", "prayer-wall",
-- "article-42"). One table instead of one-per-context because retention,
-- moderation, and the SSE fan-out are identical everywhere; only the key
-- differs.
--
-- Messages are ephemeral by design: a background sweep (resource/chat)
-- deletes rows older than 24h unless an editor-or-above set keep = true.
-- username/display_name are denormalized at post time so rendering a busy
-- channel never joins users, and so a message survives (correctly attributed)
-- a later username change.
--
-- Hand-written data access (resource/chat), no SQLBoiler model — same
-- approach as api_tokens: regenerating with the legacy SQLBoiler v2 toolchain
-- is riskier than a few explicit queries.
CREATE TABLE IF NOT EXISTS chat_messages (
    id           BIGSERIAL PRIMARY KEY,
    channel      text NOT NULL,
    user_id      BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    username     text NOT NULL,
    display_name text NOT NULL DEFAULT '',
    body         text NOT NULL,
    keep         boolean NOT NULL DEFAULT false, -- editor-pinned: survives the daily sweep
    created_at   timestamptz NOT NULL
);
-- (channel, id) serves both the initial window load (newest N in a channel)
-- and incremental polls (id > after_id) with one index — id order and
-- created_at order are identical for a BIGSERIAL.
CREATE INDEX IF NOT EXISTS idx_chat_messages_channel_id ON chat_messages (channel, id);
-- The retention sweep scans by age alone (all channels at once).
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages (created_at);
ALTER TABLE chat_messages OWNER TO "devuser";

-- Prayer wall requests. Unlike chat these are durable content (no retention
-- sweep): a request stays up until an editor removes it or the requester
-- withdraws it. answered/answered_note let editors publicly close the loop
-- ("Praise report!") without deleting the request's history.
CREATE TABLE IF NOT EXISTS prayer_requests (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    username      text NOT NULL,
    display_name  text NOT NULL DEFAULT '',
    title         text NOT NULL,
    body          text NOT NULL,
    answered      boolean NOT NULL DEFAULT false,
    answered_note text NOT NULL DEFAULT '',
    published     boolean NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_prayer_requests_created_at ON prayer_requests (created_at);
ALTER TABLE prayer_requests OWNER TO "devuser";

-- +goose Down
DROP TABLE prayer_requests;
DROP TABLE chat_messages;
