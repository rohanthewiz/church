package payment

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	// "github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/nullbio/null.v6"
)

// Amounts are cents
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

func (p ChargePresenter) Upsert() (updateOp bool, err error) {
	dbH, err := db.Db()
	if err != nil {
		return updateOp, err
	}
	chg, create, err := modelFromPresenter(p)
	if err != nil {
		logger.LogErr(err, "Error in charge from presenter")
		return updateOp, err
	}
	logger.Debug("In Upsert: charge model (from presenter)", "charge", fmt.Sprintf("%#v", chg))
	if create {
		err = chg.Insert(dbH)
		if err != nil {
			logger.LogErr(err, "Error inserting charge into DB")
			return updateOp, err
		} else {
			logger.Log("Info", "Successfully created charge")
		}
	} else {
		updateOp = true
		err = chg.Update(dbH)
		if err != nil {
			logger.LogErr(err, "Error updating charge in DB")
		} else {
			logger.Log("Info", "Successfully updated charge")
		}
	}
	return
}

// FindChargeIdByPaymentToken returns the local DB id of a charge previously recorded
// under the given payment token (for PaymentIntents flows the token column holds the
// PaymentIntent id). This gives us idempotency: the receipt page can be reloaded, or a
// webhook can arrive after the redirect, without inserting duplicate charge rows --
// the caller feeds the found id back into ChargePresenter.Id so Upsert takes the update path.
func FindChargeIdByPaymentToken(token string) (id int64, found bool, err error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, false, nil
	}
	dbH, err := db.Db()
	if err != nil {
		return 0, false, err
	}
	chg, err := models.Charges(dbH, qm.Where("payment_token = ?", token)).One()
	if err != nil {
		// sql.ErrNoRows is the expected miss case - not an error for our purposes
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, serr.Wrap(err, "Error querying charge by payment token", "token", token)
	}
	return chg.ID, true, nil
}

// Returns a charge model for id `id` or error
func findChargeById(id int64) (*models.Charge, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, err
	}
	ser, err := models.Charges(dbH, qm.Where("id = ?", id)).One()
	if err != nil {
		return nil, serr.Wrap(err, "Error retrieving charge by id", "id", fmt.Sprintf("%d", id))
	}
	return ser, err
}

// Returns a charge model for id `id` or a new charge model
func findByIdOrCreate(id string) (model *models.Charge) {
	if id != "" {
		intId, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			logger.LogErr(err, "Unable to convert Charge id to integer", "Id", id)
			return new(models.Charge)
		}
		model, err = findChargeById(intId)
		if err != nil {
			return new(models.Charge)
		}
	}
	if model == nil {
		model = new(models.Charge)
	}
	return
}

func modelFromPresenter(cp ChargePresenter) (chgMod *models.Charge, create_op bool, err error) {
	chgMod = findByIdOrCreate(cp.Id)
	if chgMod.ID < 1 {
		create_op = true
	}
	chgMod.CustomerID = null.NewString(cp.CustomerId, true)

	if custName := strings.TrimSpace(cp.CustomerName); custName != "" {
		chgMod.CustomerName = custName
	} else {
		msg := "Customer name is a required field when creating charges"
		return chgMod, create_op, serr.Wrap(errors.New(msg))
	}
	chgMod.CustomerEmail = null.NewString(strings.TrimSpace(cp.CustomerEmail), true)
	chgMod.Comment = null.NewString(strings.TrimSpace(cp.Comment), true)
	chgMod.Description = null.NewString(strings.TrimSpace(cp.Description), true)
	chgMod.ReceiptNumber = null.NewString(strings.TrimSpace(cp.ReceiptNumber), true)
	chgMod.ReceiptURL = null.NewString(strings.TrimSpace(cp.ReceiptURL), true)
	chgMod.PaymentToken = strings.TrimSpace(cp.PaymentToken)
	chgMod.Captured = null.NewBool(cp.Captured, true)
	chgMod.Paid = null.NewBool(cp.Paid, true)
	chgMod.AmountPaid = null.NewInt64(cp.AmtPaid, true)
	chgMod.Refunded = null.NewBool(cp.Refunded, true)
	chgMod.AmountRefunded = null.NewInt64(cp.AmtRefunded, true)
	// Meta carries auxiliary identifiers as JSON (e.g. the Stripe charge id under the
	// PaymentIntents flow, where PaymentToken holds the intent id). Passed through
	// rather than blanked so callers control what lands here.
	chgMod.Meta = null.NewString(strings.TrimSpace(cp.Meta), true)

	return
}
