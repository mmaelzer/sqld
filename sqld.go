package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	sq "github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

const usageMessage = "" +
	`Usage of 'sqld':
	sqld -user=root -name=table_name -port=8000
`

var (
	DSN    = flag.String("dsn", "", "database source name (default: 'localhost:3306')")
	user   = flag.String("user", "", "database username (default: root)")
	pass   = flag.String("pass", "", "database password (default: '')")
	DBType = flag.String("type", "", "database type (default: mysql)")
	DBName = flag.String("name", "", "database name")
	port   = flag.String("port", "", "http port (default: 8080")

	mysqlDSNTemplate    = "%s:%s@(%s)/%s?parseTime=true"
	postgresDSNTemplate = "user='%s' password='%s' dbname='%s'"

	db *sqlx.DB
)

func usage() {
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(2)
}

func buildDsn() string {
	if DSN == nil || *DSN == "" {
		*DSN = "localhost:3306"
	}

	if user == nil || *user == "" {
		*user = "root"
	}

	if pass == nil || *pass == "" {
		*pass = ""
	}

	switch *DBType {
	case "mysql":
		return fmt.Sprintf(mysqlDSNTemplate, *user, *pass, *DSN, *DBName)
	case "postgres":
		return fmt.Sprintf(postgresDSNTemplate, *user, *pass, *DBName)
	default:
		return ""
	}
}

func buildQuery(r *http.Request) (string, []interface{}, error) {
	table := strings.TrimPrefix(r.URL.Path, "/")
	query := sq.Select("*").From(table)
	values := r.URL.Query()

	limitStr := values.Get("__limit__")
	values.Del("__limit__")

	offsetStr := values.Get("__offset__")
	values.Del("__offset__")

	limit := 0
	offset := 0

	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	if offsetStr != "" {
		offset, _ = strconv.Atoi(offsetStr)
	}

	for key, val := range values {
		query = query.Where(sq.Eq{key: val})
	}

	if offset > 0 {
		query = query.Offset(uint64(offset))
	}

	if limit > 0 {
		query = query.Limit(uint64(limit))
	}

	return query.ToSql()
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	sql, args, err := buildQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := db.Query(sql, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	count := len(columns)
	tableData := make([]map[string]interface{}, 0)
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)

	for rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		err = rows.Scan(valuePtrs...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rowData := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)

			var v interface{}
			if ok {
				v = string(b)
			} else {
				v = val
			}
			rowData[col] = v
		}
		tableData = append(tableData, rowData)
	}

	err = rows.Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tableData)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if DBType == nil || *DBType == "" {
		*DBType = "mysql"
	}

	if port == nil || *port == "" {
		*port = "8080"
	}

	var err error
	db, err = sqlx.Connect(*DBType, buildDsn())
	if err != nil {
		fmt.Printf("Unable to connect to database: %s\n", err)
		os.Exit(1)
	}

	http.HandleFunc("/", handleQuery)
	fmt.Println(http.ListenAndServe(":"+*port, nil))
}
