package drivers

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	// Import sqlite driver
	_ "github.com/mattn/go-sqlite3"
)

// InitSQLite sets up squirrel and creates a SQLite connection
func InitSQLite(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}
