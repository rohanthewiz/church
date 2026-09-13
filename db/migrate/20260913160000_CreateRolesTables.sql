-- +goose Up
-- Role-based admin access (see resource/authz).
--
--   users ──< user_roles >── roles ──< role_permissions
--
-- Permissions are rows ("articles.publish") rather than a text[] on roles so
-- the hand-written queries stay plain single-table SELECTs on both backends,
-- with no array scanning.
--
-- Every table has a surrogate bigserial id, with the natural key enforced by
-- a unique index instead of a composite primary key. That is the shape the
-- bytdb schema can express identically (db/bytdb_schema.go).
--
-- The legacy users.role column stays: it still drives chat moderation and is
-- part of the mobile /auth/me contract. Default roles and the backfill from
-- users.role are created at boot (authz.EnsureDefaultRoles), not here, so the
-- two backends seed through one code path.
CREATE TABLE IF NOT EXISTS roles (
    id          BIGSERIAL PRIMARY KEY,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz,
    updated_at  timestamptz,
    updated_by  text NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_roles_name ON roles (name);
ALTER TABLE roles OWNER TO "devuser";

CREATE TABLE IF NOT EXISTS role_permissions (
    id         BIGSERIAL PRIMARY KEY,
    role_id    BIGINT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission text NOT NULL
);
CREATE UNIQUE INDEX idx_role_permissions_role_perm ON role_permissions (role_id, permission);
ALTER TABLE role_permissions OWNER TO "devuser";

CREATE TABLE IF NOT EXISTS user_roles (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    BIGINT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created_at timestamptz
);
CREATE UNIQUE INDEX idx_user_roles_user_role ON user_roles (user_id, role_id);
-- Deleting a role scans assignments by role_id.
CREATE INDEX idx_user_roles_role_id ON user_roles (role_id);
ALTER TABLE user_roles OWNER TO "devuser";

-- +goose Down
DROP TABLE user_roles;
DROP TABLE role_permissions;
DROP TABLE roles;
