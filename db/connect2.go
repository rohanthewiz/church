package db

import (
	"fmt"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/rohanthewiz/serr"
	"errors"
)

// Cache DB handle and options
var dbHandle2 *sql.DB
var dbOpts2 *DBOpts

func InitDB2(opts DBOpts) error {
	dbOpts2 = &opts
	err := openDB2()
	if err != nil {
		serr.Wrap(err, "Error initializing database")
	}
	return nil
}

func CloseDB2() {
	if dbHandle2 != nil {
		dbHandle2.Close()
	}
}

// Get a valid DB handle
func Db2() (*sql.DB, error) {
	if dbHandle2 != nil {
		if dbHandle2.Ping() == nil {  // pings without error
			return dbHandle2, nil
		}
	}
	err := openDB2()
	return dbHandle2, err
}

func openDB2() error {
	if dbOpts2 == nil {
		return serr.Wrap(errors.New("Please call InitDB2 before using database"))
	}
	opts := fmt.Sprintf("host=%s dbname=%s user=%s password=%s port=%s sslmode=disable",
		dbOpts2.Host, dbOpts2.Database, dbOpts2.User, dbOpts2.Word, dbOpts2.Port,
	)
	db_, err := sql.Open(dbOpts2.DBType, opts)
	if err != nil {
		return serr.Wrap(err, "Failed to open database")
	}
	dbHandle2 = db_
	return nil
}
