package db

import (
	"fmt"
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/rohanthewiz/serr"
	"errors"
)

// Cache DB handle and options
var dbHandle *sql.DB
var dbOpts *DBOpts

var DBTypes = dbTypes{"postgres", "mysql"}
type dbTypes struct {
	Postgres, MySQL string
}

type DBOpts struct {
	DBType   string
	Host     string
	Port     string
	User     string
	Word     string
	Database string
}

func InitDB(opts DBOpts) error {
	dbOpts = &opts
	err := openDB()
	if err != nil {
		serr.Wrap(err, "Error initializing database")
	}
	return nil
}

func CloseDB() {
	if dbHandle != nil {
		dbHandle.Close()
	}
}

// Get a valid DB handle
func Db() (*sql.DB, error) {
	if dbHandle != nil {
		if dbHandle.Ping() == nil {  // pings without error
			return dbHandle, nil
		}
	}
	err := openDB()
	return dbHandle, err
}

func openDB() error {
	if dbOpts == nil {
		return serr.Wrap(errors.New("Please call InitDB before using database"))
	}
	opts := fmt.Sprintf("host=%s dbname=%s user=%s password=%s port=%s sslmode=disable",
		dbOpts.Host, dbOpts.Database, dbOpts.User, dbOpts.Word, dbOpts.Port,
	)
	db_, err := sql.Open(dbOpts.DBType, opts)
	if err != nil {
		return serr.Wrap(err, "Failed to open database")
	}
	dbHandle = db_
	return nil
}
