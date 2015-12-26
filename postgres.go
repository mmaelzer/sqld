package main

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func initPostgres() (*sqlx.DB, error) {
	sq = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	return sqlx.Connect(*dbtype, buildDSN())
}
