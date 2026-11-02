package database

import (
	_ "modernc.org/sqlite"

	"github.com/jmoiron/sqlx"
)

func NewDatabase(databaseURL string) (*sqlx.DB, error) {
	return sqlx.Connect("sqlite", databaseURL)
}
