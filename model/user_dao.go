package model

import (
	"database/sql"
	"fmt"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func UserByID(id int64) (*User, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + userColumns + ` FROM users WHERE id = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), id)
	u, err := scanUser(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving user by id", "id", fmt.Sprintf("%d", id))
	}
	return u, nil
}

func UserByUsername(username string) (*User, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + userColumns + ` FROM users WHERE username = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), username)
	u, err := scanUser(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving user by username", "username", username)
	}
	return u, nil
}

// UserCredsByUsername fetches only the fields required for authentication.
// Narrowing the SELECT keeps password hashes out of unrelated code paths.
func UserCredsByUsername(username string) (passHash, salt string, err error) {
	dbH, err := db.Db()
	if err != nil {
		return "", "", serr.Wrap(err)
	}
	const q = `SELECT encrypted_password, encrypted_salt FROM users
		WHERE username = ? AND enabled = ? LIMIT 1`
	var ph, s sql.NullString
	err = dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), username, true).Scan(&ph, &s)
	if err != nil {
		return "", "", err
	}
	return ph.String, s.String, nil
}

// ExistsUserWithRole returns true if at least one row matches `role`.
// Single call site (SuperAdminsExist) — limited to a tiny SELECT + EXISTS.
func ExistsUserWithRole(role int) (bool, error) {
	dbH, err := db.Db()
	if err != nil {
		return false, serr.Wrap(err)
	}
	const q = `SELECT EXISTS (SELECT 1 FROM users WHERE role = ?)`
	var exists bool
	if err := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), role).Scan(&exists); err != nil {
		return false, serr.Wrap(err, "Error checking for users with role")
	}
	return exists, nil
}

func QueryUsers(condition, order string, limit, offset int64) ([]*User, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	q := `SELECT ` + userColumns + ` FROM users`
	if condition != "" {
		q += ` WHERE ` + condition
	}
	if order != "" {
		q += ` ORDER BY ` + order
	}
	q += ` LIMIT ? OFFSET ?`

	rows, err := dbH.Query(db.Rebind(db.CurrentDialect(), q), limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying users")
	}
	defer rows.Close()

	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, serr.Wrap(err, "Error scanning user row")
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "Error iterating user rows")
	}
	return out, nil
}

// AllUsers is a convenience wrapper around QueryUsers — retained because
// it was part of the package-level API consumed by admin views.
func AllUsers() ([]*User, error) {
	// Large LIMIT acts as "no bound" — the DB has handfuls of rows, not
	// millions, so an unbounded scan is fine here.
	return QueryUsers("", "id ASC", 1<<30, 0)
}

func InsertUser(u *User) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		INSERT INTO users
			(created_at, updated_at, updated_by, enabled, role,
			 username, email_address, first_name, last_name, summary,
			 encrypted_password, encrypted_salt, password_reset_at, confirmed_at, prefs)
		VALUES
			(NOW(), NOW(), ?, ?, ?,
			 ?, ?, ?, ?, ?,
			 ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		u.UpdatedBy, u.Enabled, u.Role,
		u.Username, u.EmailAddress, u.FirstName, u.LastName, u.Summary,
		u.EncryptedPassword, u.EncryptedSalt, u.PasswordResetAt, u.ConfirmedAt, u.Prefs,
	)
	if err := row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error inserting user")
	}
	return nil
}

// UpdateUser intentionally does NOT touch `username` — per the presenter
// layer it is write-once on create. Keeping that invariant in SQL means
// any caller attempting to change it via the struct is silently no-op'd.
func UpdateUser(u *User) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		UPDATE users SET
			updated_at         = NOW(),
			updated_by         = ?,
			enabled            = ?,
			role               = ?,
			email_address      = ?,
			first_name         = ?,
			last_name          = ?,
			summary            = ?,
			encrypted_password = ?,
			encrypted_salt     = ?,
			password_reset_at  = ?,
			confirmed_at       = ?,
			prefs              = ?
		WHERE id = ?
		RETURNING updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		u.UpdatedBy, u.Enabled, u.Role,
		u.EmailAddress, u.FirstName, u.LastName, u.Summary,
		u.EncryptedPassword, u.EncryptedSalt, u.PasswordResetAt, u.ConfirmedAt, u.Prefs,
		u.ID,
	)
	if err := row.Scan(&u.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error updating user", "id", fmt.Sprintf("%d", u.ID))
	}
	return nil
}

func DeleteUser(id int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `DELETE FROM users WHERE id = ?`
	if _, err := dbH.Exec(db.Rebind(db.CurrentDialect(), q), id); err != nil {
		return serr.Wrap(err, "Error deleting user", "id", fmt.Sprintf("%d", id))
	}
	return nil
}
