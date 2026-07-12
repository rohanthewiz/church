package payment

// Charge persistence tests. These use the executor-injection seam directly —
// each test hands its own sqlmock to the function under test, no global
// db.SetHandleForTesting swap needed. This is the payoff of query functions
// taking a db.Executor first parameter (see db/executor.go).

import (
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestChargeUpsertInsertsNewCharge(t *testing.T) {
	dbH, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbH.Close() })

	// No Id on the presenter → create path: no lookup, straight INSERT.
	// SQLBoiler inserts with RETURNING for defaulted columns, hence ExpectQuery.
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "charges"`)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42)))

	cp := ChargePresenter{
		CustomerName: "Kim Lee",
		PaymentToken: "pi_test_123",
		Description:  "Tithe",
		Paid:         true,
		AmtPaid:      5000,
	}
	updateOp, err := cp.Upsert(dbH)
	if err != nil {
		t.Fatalf("Upsert (insert path) failed: %v", err)
	}
	if updateOp {
		t.Error("a presenter without an Id must take the insert path (updateOp=false)")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestChargeUpsertUpdatesExistingCharge(t *testing.T) {
	dbH, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbH.Close() })

	// Id set → the model is loaded first, then UPDATEd — the path the
	// idempotent payment recorder relies on to avoid duplicate charge rows.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "charges" WHERE (id = $1)`)).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "customer_name", "payment_token"}).
			AddRow(int64(42), "Kim Lee", "pi_test_123"))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "charges"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	cp := ChargePresenter{
		Id:           "42",
		CustomerName: "Kim Lee",
		PaymentToken: "pi_test_123",
		Paid:         true,
		AmtPaid:      5000,
	}
	updateOp, err := cp.Upsert(dbH)
	if err != nil {
		t.Fatalf("Upsert (update path) failed: %v", err)
	}
	if !updateOp {
		t.Error("a presenter with an existing Id must take the update path (updateOp=true)")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

// A missing customer name must fail before any write reaches the DB.
func TestChargeUpsertRequiresCustomerName(t *testing.T) {
	dbH, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbH.Close() })

	cp := ChargePresenter{PaymentToken: "pi_x", AmtPaid: 100} // no CustomerName
	if _, err := cp.Upsert(dbH); err == nil {
		t.Error("Upsert should reject a charge without a customer name")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("no SQL should run for an invalid charge: %v", err)
	}
}

func TestFindChargeIdByPaymentToken(t *testing.T) {
	dbH, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbH.Close() })

	t.Run("found", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "charges" WHERE (payment_token = $1)`)).
			WithArgs("pi_test_123").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
		id, found, err := FindChargeIdByPaymentToken(dbH, "pi_test_123")
		if err != nil || !found || id != 9 {
			t.Errorf("got (%d, %v, %v), want (9, true, nil)", id, found, err)
		}
	})

	t.Run("miss is not an error", func(t *testing.T) {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "charges" WHERE (payment_token = $1)`)).
			WithArgs("pi_unknown").
			WillReturnRows(sqlmock.NewRows([]string{"id"})) // zero rows
		id, found, err := FindChargeIdByPaymentToken(dbH, "pi_unknown")
		if err != nil || found || id != 0 {
			t.Errorf("got (%d, %v, %v), want (0, false, nil)", id, found, err)
		}
	})

	t.Run("blank token short-circuits without a query", func(t *testing.T) {
		id, found, err := FindChargeIdByPaymentToken(dbH, "   ")
		if err != nil || found || id != 0 {
			t.Errorf("got (%d, %v, %v), want (0, false, nil)", id, found, err)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}
