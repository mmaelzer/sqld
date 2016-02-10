package drivers

import (
	"github.com/jmoiron/sqlx"
)

// SQLConnector provides a type alias for a db initialize function
type SQLConnector func(driverName, dataSourceName string) (*sqlx.DB, error)
