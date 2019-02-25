package payment

import (
	"errors"
	"fmt"
	//"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/models"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
	"github.com/vattle/sqlboiler/queries/qm"
	"gopkg.in/nullbio/null.v6"
	"strconv"
	"strings"
)

// Amounts are cents
type ChargePresenter struct {
	Id            string
	CustomerId    string
	CustomerName  string
	Description   string
	ReceiptNumber string
	ReceiptURL    string
	PaymentToken  string
	Captured      bool
	Paid          bool
	AmtPaid       int64
	Refunded      bool
	AmtRefunded   int64
	Meta string
	CreatedAt     string
	UpdatedAt     string
}

func (p ChargePresenter) Upsert() (updateOp bool, err error) {
	dbH, err := db.Db()
	if err != nil {
		return  updateOp, err
	}
	chg, create, err := modelFromPresenter(p)
	if err != nil {
		logger.LogErr(err, "Error in charge from presenter")
		return updateOp, err
	}
	fmt.Printf("In Upsert: charge model (from presenter) %#v\n", chg)
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
	chgMod.CustomerID = null.NewInt64(cp.CustomerId, true)

	if custName := strings.TrimSpace(cp.CustomerName); custName != "" {
		chgMod.CustomerName = custName
	} else {
		msg := "Customer name is a required field when creating charges"
		return chgMod, create_op, serr.Wrap(errors.New(msg))
	}
	chgMod.Description = null.NewString(strings.TrimSpace(cp.Description), true)
	chgMod.ReceiptNumber = null.NewString(strings.TrimSpace(cp.ReceiptNumber), true)
	chgMod.ReceiptURL = null.NewString(strings.TrimSpace(cp.ReceiptURL), true)
	chgMod.PaymentToken = strings.TrimSpace(cp.PaymentToken)
	chgMod.Captured = null.NewBool(cp.Captured, true)
	chgMod.Paid = null.NewBool(cp.Paid, true)
	chgMod.AmountPaid = null.NewInt64(cp.AmtPaid, true)
	chgMod.Refunded = null.NewBool(cp.Refunded, true)
	chgMod.AmountRefunded = null.NewInt64(cp.AmtRefunded, true)
	chgMod.Meta = null.NewString("", true) // no meta for now

	return
}
