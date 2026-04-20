package model

import (
	"fmt"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

func ChargeByID(id int64) (*Charge, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err)
	}
	const q = `SELECT ` + chargeColumns + ` FROM charges WHERE id = ? LIMIT 1`
	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q), id)
	c, err := scanCharge(row)
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving charge by id", "id", fmt.Sprintf("%d", id))
	}
	return c, nil
}

func InsertCharge(c *Charge) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		INSERT INTO charges
			(created_at, updated_at, customer_id, customer_name, customer_email,
			 description, comment, receipt_number, receipt_url, payment_token,
			 captured, paid, amount_paid, refunded, amount_refunded, meta)
		VALUES
			(NOW(), NOW(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		c.CustomerID, c.CustomerName, c.CustomerEmail,
		c.Description, c.Comment, c.ReceiptNumber, c.ReceiptURL, c.PaymentToken,
		c.Captured, c.Paid, c.AmountPaid, c.Refunded, c.AmountRefunded, c.Meta,
	)
	if err := row.Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error inserting charge")
	}
	return nil
}

func UpdateCharge(c *Charge) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `
		UPDATE charges SET
			updated_at      = NOW(),
			customer_id     = ?,
			customer_name   = ?,
			customer_email  = ?,
			description     = ?,
			comment         = ?,
			receipt_number  = ?,
			receipt_url     = ?,
			payment_token   = ?,
			captured        = ?,
			paid            = ?,
			amount_paid     = ?,
			refunded        = ?,
			amount_refunded = ?,
			meta            = ?
		WHERE id = ?
		RETURNING updated_at`

	row := dbH.QueryRow(db.Rebind(db.CurrentDialect(), q),
		c.CustomerID, c.CustomerName, c.CustomerEmail,
		c.Description, c.Comment, c.ReceiptNumber, c.ReceiptURL, c.PaymentToken,
		c.Captured, c.Paid, c.AmountPaid, c.Refunded, c.AmountRefunded, c.Meta, c.ID,
	)
	if err := row.Scan(&c.UpdatedAt); err != nil {
		return serr.Wrap(err, "Error updating charge", "id", fmt.Sprintf("%d", c.ID))
	}
	return nil
}

func DeleteCharge(id int64) error {
	dbH, err := db.Db()
	if err != nil {
		return serr.Wrap(err)
	}
	const q = `DELETE FROM charges WHERE id = ?`
	if _, err := dbH.Exec(db.Rebind(db.CurrentDialect(), q), id); err != nil {
		return serr.Wrap(err, "Error deleting charge", "id", fmt.Sprintf("%d", id))
	}
	return nil
}
