package main

import (
	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"
)

func initSQLite() (*sqlx.DB, error) {
	return sqlx.Connect(*DBType, buildDSN())
}
