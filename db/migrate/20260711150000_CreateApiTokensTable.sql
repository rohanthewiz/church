-- +goose Up
-- Bearer tokens for the mobile JSON API (/api/v1/auth/*, Phase 2 of the
-- mobile plan). DB-backed on purpose: web sessions live in the in-process
-- kvstore, which evaporates on every deploy — acceptable for a browser
-- session, unacceptable for a phone that should stay signed in for weeks.
--
-- Only a SHA-256 hash of the token is stored, so a leaked DB dump (or a
-- careless SELECT in a support session) cannot be replayed as credentials.
-- The plaintext token exists exactly once: in the login response body.
--
-- Hand-written data access (resource/apitoken), no SQLBoiler model — same
-- approach as sermon_cache_access and event_recurrences: regenerating with
-- the legacy SQLBoiler v2 toolchain is riskier than a few explicit queries.
CREATE TABLE IF NOT EXISTS api_tokens (
    id           BIGSERIAL PRIMARY KEY,
    token_hash   text NOT NULL UNIQUE,  -- SHA-256 hex of the bearer token
    user_id      BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    device       text NOT NULL DEFAULT '', -- optional client label, e.g. "Pixel 9"
    created_at   timestamptz NOT NULL,
    last_used_at timestamptz,           -- touched on each authenticated request
    expires_at   timestamptz NOT NULL
);
-- Revocation sweeps ("log out everywhere", user disabled/deleted) go by user.
CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens (user_id);
ALTER TABLE api_tokens OWNER TO "devuser";

-- +goose Down
DROP TABLE api_tokens;
