package drivers

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func InitPostgres(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}
