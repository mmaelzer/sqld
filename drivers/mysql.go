package drivers

import (
	"github.com/Masterminds/squirrel"
	// Bring in the mysql driver
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// InitMySQL sets up squirrel and creates a MySQL connection
func InitMySQL(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}
