package drivers

import (
	"github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"

	"github.com/jmoiron/sqlx"
)

func InitSQLite(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}
