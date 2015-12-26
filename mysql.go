package main

import (
	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func initMySQL() (*sqlx.DB, error) {
	sq = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	return sqlx.Connect(*dbtype, buildDSN())
}
