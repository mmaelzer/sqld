package drivers

import (
	"github.com/jmoiron/sqlx"
)

type SQLConnector func(driverName, dataSourceName string) (*sqlx.DB, error)
