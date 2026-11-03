package database

import (
	_ "modernc.org/sqlite"

	"github.com/jmoiron/sqlx"
)

func NewDatabase(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlite", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	return db, nil
}
