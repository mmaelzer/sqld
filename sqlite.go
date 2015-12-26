package main

import (
	"github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"
)

func initSQLite() (*sqlx.DB, error) {
	sq = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	return sqlx.Connect(*dbtype, buildDSN())
}
