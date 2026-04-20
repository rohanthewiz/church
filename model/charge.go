package model

import "database/sql"

// Charge mirrors the `charges` table. Every non-required column is nullable
// in the schema, hence the heavy use of sql.Null* wrappers — presenter layer
// maps these to plain string/int for the view. AmountPaid / AmountRefunded
// are cents (int64) per the migration comment.
type Charge struct {
	ID             int64
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	CustomerID     sql.NullString
	CustomerName   string
	CustomerEmail  sql.NullString
	Description    sql.NullString
	Comment        sql.NullString
	ReceiptNumber  sql.NullString
	ReceiptURL     sql.NullString
	PaymentToken   string
	Captured       sql.NullBool
	Paid           sql.NullBool
	AmountPaid     sql.NullInt64
	Refunded       sql.NullBool
	AmountRefunded sql.NullInt64
	Meta           sql.NullString
}

const chargeColumns = `id, created_at, updated_at, customer_id, customer_name, customer_email,
	description, comment, receipt_number, receipt_url, payment_token, captured, paid,
	amount_paid, refunded, amount_refunded, meta`

func scanCharge(s scannable) (*Charge, error) {
	c := &Charge{}
	err := s.Scan(
		&c.ID,
		&c.CreatedAt,
		&c.UpdatedAt,
		&c.CustomerID,
		&c.CustomerName,
		&c.CustomerEmail,
		&c.Description,
		&c.Comment,
		&c.ReceiptNumber,
		&c.ReceiptURL,
		&c.PaymentToken,
		&c.Captured,
		&c.Paid,
		&c.AmountPaid,
		&c.Refunded,
		&c.AmountRefunded,
		&c.Meta,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}
