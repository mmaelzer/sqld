package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/mmaelzer/sqld/drivers"
)

const usageMessage = "" +
	`Usage of 'sqld':
	sqld -u root -db database_name -h localhost:3306 -type mysql
`

var (
	allowRaw = flag.Bool("raw", false, "allow raw sql queries")
	dsn      = flag.String("dsn", "", "database source name")
	user     = flag.String("u", "root", "database username")
	pass     = flag.String("p", "", "database password")
	host     = flag.String("h", "", "database host")
	dbtype   = flag.String("type", "mysql", "database type")
	dbname   = flag.String("db", "", "database name")
	port     = flag.Int("port", 8080, "http port")
	url      = flag.String("url", "/", "url prefix")

	mysqlDSNTemplate    = "%s:%s@(%s)/%s?parseTime=true"
	postgresDSNTemplate = "postgres://%s:%s@%s/%s?sslmode=disable"

	db *sqlx.DB
	sq squirrel.StatementBuilderType
)

// RawQuery wraps the request body of a raw sqld request
type RawQuery struct {
	ReadQuery  string `json:"read"`
	WriteQuery string `json:"write"`
}

// SqldError provides additional information on errors encountered
type SqldError struct {
	Code int
	Err  error
}

// Error is implemented to ensure SqldError conforms to the error
// interface
func (s *SqldError) Error() string {
	return s.Err.Error()
}

// NewError is a SqldError factory
func NewError(err error, code int) *SqldError {
	if err == nil {
		err = errors.New("")
	}
	return &SqldError{
		Code: code,
		Err:  err,
	}
}

// BadRequest builds a SqldError that represents a bad request
func BadRequest(err error) *SqldError {
	return NewError(err, http.StatusBadRequest)
}

// NotFound builds a SqldError that represents a not found error
func NotFound(err error) *SqldError {
	return NewError(err, http.StatusNotFound)
}

// InternalError builds a SqldError that represents an internal error
func InternalError(err error) *SqldError {
	return NewError(err, http.StatusInternalServerError)
}

func usage() {
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(2)
}

func handleFlags() {
	flag.Usage = usage
	flag.Parse()
	if !strings.HasSuffix(*url, "/") {
		*url += "/"
	}

	if !strings.HasPrefix(*url, "/") {
		*url = "/" + *url
	}
}

func buildDSN() string {
	if dsn != nil && *dsn != "" {
		return *dsn
	}

	if host == nil || *host == "" {
		if *dbtype == "postgres" {
			*host = "localhost:5432"
		} else {
			*host = "localhost:3306"
		}
	}

	if user == nil || *user == "" {
		*user = "root"
	}

	if pass == nil || *pass == "" {
		*pass = ""
	}

	switch *dbtype {
	case "mysql":
		return fmt.Sprintf(mysqlDSNTemplate, *user, *pass, *host, *dbname)
	case "postgres":
		return fmt.Sprintf(postgresDSNTemplate, *user, *pass, *host, *dbname)
	default:
		return ""
	}
}

func initDB(connect drivers.SQLConnector) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	switch *dbtype {
	case "mysql":
		return drivers.InitMySQL(connect, *dbtype, buildDSN())
	case "postgres":
		return drivers.InitPostgres(connect, *dbtype, buildDSN())
	case "sqlite3":
		return drivers.InitSQLite(connect, *dbtype, buildDSN())
	}
	return nil, sq, errors.New("Unsupported database type " + *dbtype)
}

func closeDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func parseRequest(r *http.Request) (string, map[string][]string, string) {
	paths := strings.Split(strings.TrimPrefix(r.URL.Path, *url), "/")
	table := paths[0]
	id := ""
	if len(paths) > 1 {
		id = paths[1]
	}
	return table, r.URL.Query(), id
}

func buildSelectQuery(r *http.Request) (string, []interface{}, error) {
	table, args, id := parseRequest(r)
	query := sq.Select("*").From(table)

	if id != "" {
		query = query.Where(squirrel.Eq{"id": id})
	}

	for key, val := range args {
		switch key {
		case "__limit__":
			limit, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Limit(uint64(limit))
			}
		case "__offset__":
			offset, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Offset(uint64(offset))
			}
		case "__order_by__":
			query = query.OrderBy(val...)
		default:
			query = query.Where(squirrel.Eq{key: val})
		}
	}

	return query.ToSql()
}

func buildUpdateQuery(r *http.Request, values map[string]interface{}) (string, []interface{}, error) {
	table, args, id := parseRequest(r)
	query := sq.Update("").Table(table)

	for key, val := range values {
		query = query.SetMap(squirrel.Eq{key: val})
	}

	if id != "" {
		query = query.Where(squirrel.Eq{"id": id})
	}

	for key, val := range args {
		switch key {
		case "__limit__":
			limit, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Limit(uint64(limit))
			}
		default:
			query = query.Where(squirrel.Eq{key: val})
		}
	}

	return query.ToSql()
}

func buildDeleteQuery(r *http.Request) (string, []interface{}, error) {
	table, args, id := parseRequest(r)
	query := sq.Delete("").From(table)

	if id != "" {
		query = query.Where(squirrel.Eq{"id": id})
	}

	for key, val := range args {
		switch key {
		case "__limit__":
			limit, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Limit(uint64(limit))
			}
		default:
			query = query.Where(squirrel.Eq{key: val})
		}
	}

	return query.ToSql()
}

func readQuery(sql string, args []interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	count := len(columns)
	var tableData []map[string]interface{}
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)

	for rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		err = rows.Scan(valuePtrs...)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	return tableData, nil
}

// read handles the GET request.
func read(r *http.Request) (interface{}, *SqldError) {
	sql, args, err := buildSelectQuery(r)
	if err != nil {
		return nil, BadRequest(err)
	}

	tableData, err := readQuery(sql, args)
	if err != nil {
		return nil, InternalError(err)
	}
	return tableData, nil
}

// createSingle handles the POST method when only a single model
// is provided in the request body.
func createSingle(table string, item map[string]interface{}) (map[string]interface{}, error) {
	columns := make([]string, len(item))
	values := make([]interface{}, len(item))

	i := 0
	for c, val := range item {
		columns[i] = c
		values[i] = val
		i++
	}

	query := sq.Insert(table).
		Columns(columns...).
		Values(values...)

	sql, args, err := query.ToSql()

	res, err := db.Exec(sql, args...)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	item["id"] = id
	return item, nil
}

// create handles the POST method.
func create(r *http.Request) (interface{}, *SqldError) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, BadRequest(err)
	}

	table, _, _ := parseRequest(r)

	item, ok := data.(map[string]interface{})
	if ok {
		saved, err := createSingle(table, item)
		if err != nil {
			return nil, InternalError(err)
		}
		return saved, nil
	}

	return nil, BadRequest(nil)
}

// update handles the PUT method.
func update(r *http.Request) (interface{}, *SqldError) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, BadRequest(err)
	}

	sql, args, err := buildUpdateQuery(r, data)

	if err != nil {
		return nil, BadRequest(err)
	}

	return execQuery(sql, args)
}

// del handles the DELETE method.
func del(r *http.Request) (interface{}, *SqldError) {
	sql, args, err := buildDeleteQuery(r)

	if err != nil {
		return nil, BadRequest(err)
	}

	return execQuery(sql, args)
}

// execQuery will perform a sql query, return the appropriate error code
// given error states or return an http 204 NO CONTENT on success.
func execQuery(sql string, args []interface{}) (interface{}, *SqldError) {
	res, err := db.Exec(sql, args...)
	if err != nil {
		return nil, BadRequest(err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, BadRequest(err)
	}

	if res != nil && rows == 0 {
		return nil, NotFound(err)
	}

	return nil, nil
}

func raw(r *http.Request) (interface{}, *SqldError) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	var query RawQuery
	err = json.Unmarshal(body, &query)
	if err != nil {
		return nil, BadRequest(err)
	}

	var noArgs []interface{}
	if query.ReadQuery != "" {
		tableData, err := readQuery(query.ReadQuery, noArgs)
		if err != nil {
			return nil, BadRequest(err)
		}
		return tableData, nil
	} else if query.WriteQuery != "" {
		res, err := db.Exec(query.WriteQuery, noArgs...)
		if err != nil {
			return nil, BadRequest(err)
		}
		lastID, _ := res.LastInsertId()
		rAffect, _ := res.RowsAffected()
		return map[string]interface{}{
			"last_insert_id": lastID,
			"rows_affected":  rAffect,
		}, nil
	}
	return nil, BadRequest(nil)
}

// handleQuery routes the given request to the proper handler
// given the request method. If the request method matches
// no available handlers, it responds with a method not found
// status.
func handleQuery(w http.ResponseWriter, r *http.Request) {
	var err *SqldError
	var data interface{}
	status := http.StatusOK

	start := time.Now()
	logRequest := func(status int) {
		log.Printf(
			"%d %s %s %s",
			status,
			r.Method,
			r.URL.String(),
			time.Since(start),
		)
	}

	if r.URL.Path == "/" {
		if *allowRaw == true && r.Method == "POST" {
			data, err = raw(r)
		} else {
			err = BadRequest(nil)
		}
	} else {
		switch r.Method {
		case "GET":
			data, err = read(r)
		case "POST":
			data, err = create(r)
			status = http.StatusCreated
		case "PUT":
			data, err = update(r)
		case "DELETE":
			data, err = del(r)
		default:
			err = &SqldError{http.StatusMethodNotAllowed, errors.New("")}
		}
	}

	if err == nil && data == nil {
		status := http.StatusNoContent
		w.WriteHeader(status)
		logRequest(status)
	} else if err != nil {
		http.Error(w, err.Error(), err.Code)
		logRequest(err.Code)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(data)
		logRequest(http.StatusOK)
	}
}

// main handles some flag defaults, connects to the database,
// and starts the http server.
func main() {
	log.SetOutput(os.Stdout)
	handleFlags()

	var err error
	db, sq, err = initDB(sqlx.Connect)
	if err != nil {
		log.Fatalf("Unable to connect to database: %s\n", err)
	}

	http.HandleFunc(*url, handleQuery)
	log.Printf("sqld listening on port %d", *port)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
