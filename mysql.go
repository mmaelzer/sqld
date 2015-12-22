package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func initMySQL() (*sqlx.DB, error) {
	return sqlx.Connect(*DBType, buildDSN())
}
