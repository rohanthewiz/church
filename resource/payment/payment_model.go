package payment

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/rohanthewiz/church/model"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// ChargePresenter is the view-layer representation of a charges row.
// Amounts are cents (int64).
type ChargePresenter struct {
	Id            string
	CustomerId    string
	CustomerName  string
	CustomerEmail string
	Comment       string
	Description   string
	ReceiptNumber string
	ReceiptURL    string
	PaymentToken  string
	Captured      bool
	Paid          bool
	AmtPaid       int64
	Refunded      bool
	AmtRefunded   int64
	Meta          string
	CreatedAt     string
	UpdatedAt     string
}

// Upsert inserts or updates a charges row from the presenter, returning
// updateOp=true when an existing row was modified (false on create).
func (p ChargePresenter) Upsert() (updateOp bool, err error) {
	chg, create, err := modelFromPresenter(p)
	if err != nil {
		logger.LogErr(err, "Error in charge from presenter")
		return updateOp, err
	}
	if create {
		if err := model.InsertCharge(chg); err != nil {
			logger.LogErr(err, "Error inserting charge into DB")
			return updateOp, err
		}
		logger.Log("Info", "Successfully created charge")
	} else {
		updateOp = true
		if err := model.UpdateCharge(chg); err != nil {
			logger.LogErr(err, "Error updating charge in DB")
			return updateOp, err
		}
		logger.Log("Info", "Successfully updated charge")
	}
	return updateOp, nil
}

func findChargeById(id int64) (*model.Charge, error) {
	return model.ChargeByID(id)
}

// findByIdOrCreate returns a zero-valued Charge when the id is missing or
// invalid — callers detect create vs update via m.ID < 1. Matches the
// silent-fallback pattern used by the other resource packages.
func findByIdOrCreate(id string) *model.Charge {
	if id == "" {
		return &model.Charge{}
	}
	intId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		logger.LogErr(err, "Unable to convert Charge id to integer", "Id", id)
		return &model.Charge{}
	}
	c, err := findChargeById(intId)
	if err != nil || c == nil {
		return &model.Charge{}
	}
	return c
}

// modelFromPresenter maps the flat presenter into the DB-level struct with
// sql.Null* wrappers. Every nullable text column is written with Valid=true
// and a trimmed value — empty-string is a legitimate stored value here.
// Boolean and int nullables are always populated for simplicity: the business
// logic always knows a concrete value for captured/paid/refunded.
func modelFromPresenter(cp ChargePresenter) (chgMod *model.Charge, create_op bool, err error) {
	chgMod = findByIdOrCreate(cp.Id)
	if chgMod.ID < 1 {
		create_op = true
	}

	chgMod.CustomerID = sql.NullString{String: cp.CustomerId, Valid: true}

	custName := strings.TrimSpace(cp.CustomerName)
	if custName == "" {
		return chgMod, create_op, serr.Wrap(errors.New("Customer name is a required field when creating charges"))
	}
	chgMod.CustomerName = custName

	chgMod.CustomerEmail = sql.NullString{String: strings.TrimSpace(cp.CustomerEmail), Valid: true}
	chgMod.Comment = sql.NullString{String: strings.TrimSpace(cp.Comment), Valid: true}
	chgMod.Description = sql.NullString{String: strings.TrimSpace(cp.Description), Valid: true}
	chgMod.ReceiptNumber = sql.NullString{String: strings.TrimSpace(cp.ReceiptNumber), Valid: true}
	chgMod.ReceiptURL = sql.NullString{String: strings.TrimSpace(cp.ReceiptURL), Valid: true}
	chgMod.PaymentToken = strings.TrimSpace(cp.PaymentToken)
	chgMod.Captured = sql.NullBool{Bool: cp.Captured, Valid: true}
	chgMod.Paid = sql.NullBool{Bool: cp.Paid, Valid: true}
	chgMod.AmountPaid = sql.NullInt64{Int64: cp.AmtPaid, Valid: true}
	chgMod.Refunded = sql.NullBool{Bool: cp.Refunded, Valid: true}
	chgMod.AmountRefunded = sql.NullInt64{Int64: cp.AmtRefunded, Valid: true}
	// No meta captured from the form yet — stored as empty string, not NULL,
	// so downstream SELECTs don't need a nil guard.
	chgMod.Meta = sql.NullString{String: "", Valid: true}

	return
}
