package drivers

import (
	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func InitMySQL(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}
