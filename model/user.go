package model

import (
	"database/sql"
)

// User mirrors the `users` table. Password material is nullable in schema
// (older rows predate encryption) so encrypted_password / encrypted_salt
// use sql.NullString rather than plain string.
//
// Prefs column is jsonb — we carry it as json.RawMessage-equivalent []byte
// so the struct is driver-neutral for the eventual DuckDB swap.
type User struct {
	ID                int64
	CreatedAt         sql.NullTime
	UpdatedAt         sql.NullTime
	UpdatedBy         string
	Enabled           bool
	Role              int
	Username          string
	EmailAddress      string
	FirstName         string
	LastName          sql.NullString
	Summary           sql.NullString
	EncryptedPassword sql.NullString
	EncryptedSalt     sql.NullString
	PasswordResetAt   sql.NullTime
	ConfirmedAt       sql.NullTime
	Prefs             []byte // jsonb — nil when NULL
}

// prefs is cast to VARCHAR on read — see menuDefColumns for rationale.
const userColumns = `id, created_at, updated_at, updated_by, enabled, role, ` +
	`username, email_address, first_name, last_name, summary, ` +
	`encrypted_password, encrypted_salt, password_reset_at, confirmed_at, ` +
	`CAST(prefs AS VARCHAR) AS prefs`

func scanUser(s scannable) (*User, error) {
	u := &User{}
	err := s.Scan(
		&u.ID,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.UpdatedBy,
		&u.Enabled,
		&u.Role,
		&u.Username,
		&u.EmailAddress,
		&u.FirstName,
		&u.LastName,
		&u.Summary,
		&u.EncryptedPassword,
		&u.EncryptedSalt,
		&u.PasswordResetAt,
		&u.ConfirmedAt,
		&u.Prefs,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}
