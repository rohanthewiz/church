package model

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

// ArticleByID returns a single article or sql.ErrNoRows wrapped in serr.
// Kept terse — the generated SQLBoiler equivalent was a dozen lines of
// reflection; we need exactly one parametrized SELECT.
func ArticleByID(id int64) (*Article, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + articleColumns + ` FROM articles WHERE id = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), id)
	a, err := scanArticle(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving article by id", "id", fmt.Sprintf("%d", id))
	}
	return a, nil
}

func ArticleBySlug(slug string) (*Article, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + articleColumns + ` FROM articles WHERE slug = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), slug)
	a, err := scanArticle(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving article by slug", "slug", slug)
	}
	return a, nil
}

// AnyArticleExists is used by bootstrap routines to skip seeding a welcome
// article when the articles table already has rows. Keeps the bootstrap code
// table-agnostic — it doesn't need to know the seeder inserts a particular
// slug.
func AnyArticleExists() (bool, error) {
	dbH, err := db.Db()
	if err != nil {
		return false, serr.Wrap(err)
	}
	const q = `SELECT 1 FROM articles LIMIT 1`
	var one int
	err = dbH.QueryRow(db.Rebind(db.CurrentDialect(), q)).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, serr.Wrap(err, "Error checking article existence")
	}
	return true, nil
}

// QueryArticles preserves the legacy signature: `condition` and `order` are
// already-formed SQL fragments (no placeholders; callers build them locally
// from trusted module config). We string-concat them into the final query.
//
// Security note: this is the same trust model as the prior SQLBoiler call —
// conditions come from internal module config like "published = true", not
// user input. If that ever changes, the caller is responsible for sanitising.
func QueryArticles(condition, order string, limit, offset int64) ([]*Article, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}

	q := `SELECT ` + articleColumns + ` FROM articles`
	if condition != "" {
		q += ` WHERE ` + condition
	}
	if order != "" {
		q += ` ORDER BY ` + order
	}
	q += ` LIMIT ? OFFSET ?`

	rows, err := dbH.Query(db.Rebind(db.CurrentDialect(), q), limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "Error querying articles")
	}
	defer rows.Close()

	var out []*Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, serr.Wrap(err, "Error scanning article row")
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "Error iterating article rows")
	}
	return out, nil
}

// InsertArticle creates a row and back-fills the generated id/timestamps onto
// the caller's struct — mirroring SQLBoiler's Insert hook behavior so callers
// can continue to read art.ID after a successful create.
//
// Uses RETURNING so we round-trip created_at/updated_at the DB actually stored.
// Both Postgres and DuckDB (>=0.9) support RETURNING on INSERT.
func InsertArticle(a *Article) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		INSERT INTO articles
			(created_at, updated_at, updated_by, title, slug, summary, body, published, categories)
		VALUES
			(NOW(), NOW(), ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		a.UpdatedBy, a.Title, a.Slug, a.Summary, a.Body, a.Published, a.Categories,
	)
	if err := row.Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error inserting article")
	}
	return nil
}

// UpdateArticle rewrites every non-timestamp column. We don't do partial
// updates here; the presenter layer already builds the full desired row
// before calling us, so column-level dirty tracking would be dead weight.
func UpdateArticle(a *Article) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		UPDATE articles SET
			updated_at = NOW(),
			updated_by = ?,
			title      = ?,
			slug       = ?,
			summary    = ?,
			body       = ?,
			published  = ?,
			categories = ?
		WHERE id = ?
		RETURNING updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		a.UpdatedBy, a.Title, a.Slug, a.Summary, a.Body, a.Published, a.Categories, a.ID,
	)
	if err := row.Scan(&a.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return serr.Wrap(err, "Article not found for update", "id", fmt.Sprintf("%d", a.ID))
		}
		return serr.Wrap(err, "Error updating article", "id", fmt.Sprintf("%d", a.ID))
	}
	return nil
}

func DeleteArticle(id int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `DELETE FROM articles WHERE id = ?`
	if _, err := dbH.Exec(db.Rebind(db.CurrentDialect(), q), id); err != nil {
		return serr.Wrap(err, "Error deleting article", "id", fmt.Sprintf("%d", id))
	}
	return nil
}