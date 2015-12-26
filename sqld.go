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
	"sync"
	"time"

	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
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
	nolog    = flag.Bool("nolog", false, "disable logging")

	mysqlDSNTemplate    = "%s:%s@(%s)/%s?parseTime=true"
	postgresDSNTemplate = "postgres://%s:%s@%s/%s?sslmode=disable"

	db *sqlx.DB
	sq squirrel.StatementBuilderType
)

type RawQuery struct {
	ReadQuery  string `json:"read"`
	WriteQuery string `json:"write"`
}

type SqldError struct {
	Code int
	Err  error
}

func (s *SqldError) Error() string {
	return s.Err.Error()
}

func NewError(err error, code int) *SqldError {
	if err == nil {
		err = errors.New("")
	}
	return &SqldError{
		Code: code,
		Err:  err,
	}
}

func BadRequest(err error) *SqldError {
	return NewError(err, http.StatusBadRequest)
}

func NotFound(err error) *SqldError {
	return NewError(err, http.StatusNotFound)
}

func InternalError(err error) *SqldError {
	return NewError(err, http.StatusInternalServerError)
}

func usage() {
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(2)
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
		return *dsn
	}
}

func initDB() (*sqlx.DB, error) {
	switch *dbtype {
	case "mysql":
		return initMySQL()
	case "postgres":
		return initPostgres()
	case "sqlite3":
		return initSQLite()
	}
	return nil, errors.New("Unsupported database type " + *dbtype)
}

func buildSelectQuery(r *http.Request) (string, []interface{}, error) {
	paths := strings.Split(r.URL.Path, "/")
	table := paths[1]
	query := sq.Select("*").From(table)

	if len(paths) > 2 && paths[2] != "" {
		query = query.Where(squirrel.Eq{"id": paths[2]})
	}

	for key, val := range r.URL.Query() {
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
		default:
			query = query.Where(squirrel.Eq{key: val})
		}
	}

	return query.ToSql()
}

func buildUpdateQuery(r *http.Request, values map[string]interface{}) (string, []interface{}, error) {
	paths := strings.Split(r.URL.Path, "/")
	table := paths[1]
	query := sq.Update("").Table(table)

	for key, val := range values {
		query = query.SetMap(squirrel.Eq{key: val})
	}

	if len(paths) > 2 && paths[2] != "" {
		query = query.Where(squirrel.Eq{"id": paths[2]})
	}

	for key, val := range r.URL.Query() {
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
	paths := strings.Split(r.URL.Path, "/")
	table := paths[1]
	query := sq.Delete("").From(table)

	if len(paths) > 2 && paths[2] != "" {
		query = query.Where(squirrel.Eq{"id": paths[2]})
	}

	for key, val := range r.URL.Query() {
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
	tableData := make([]map[string]interface{}, 0)
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

// createMany handles the POST method when only multiple models
// are provided in the request body.
func createMany(table string, list []interface{}) ([]map[string]interface{}, []error) {

	var wg sync.WaitGroup
	var errors []error
	var errMutex sync.Mutex
	var itemMutex sync.Mutex

	items := make([]map[string]interface{}, 0)

	for i := range list {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			item, err := createSingle(table, list[i].(map[string]interface{}))
			if err != nil {
				errMutex.Lock()
				errors = append(errors, err)
				errMutex.Unlock()
			} else {
				itemMutex.Lock()
				items = append(items, item)
				itemMutex.Unlock()
			}
		}(i)
	}
	wg.Wait()

	return items, errors
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

	paths := strings.Split(r.URL.Path, "/")
	table := paths[1]

	list, ok := data.([]interface{})
	if ok {
		manySaved, err := createMany(table, list)
		if len(err) == 0 {
			return manySaved, nil
		} else {
			return map[string]interface{}{
				"errors":  err,
				"objects": manySaved,
			}, nil
		}
	}

	item, ok := data.(map[string]interface{})
	if ok {
		saved, err := createSingle(table, item)
		if err != nil {
			return nil, InternalError(err)
		} else {
			return saved, nil
		}
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

	noArgs := make([]interface{}, 0)
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
		lastId, _ := res.LastInsertId()
		rAffect, _ := res.RowsAffected()
		return struct {
			LastInsertId int64 `json:"last_insert_id"`
			RowsAffected int64 `json:"rows_affected"`
		}{
			lastId,
			rAffect,
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

	start := time.Now()
	logRequest := func(status int) {
		if *nolog {
			return
		}
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
	}

	switch r.Method {
	case "GET":
		data, err = read(r)
	case "POST":
		data, err = create(r)
	case "PUT":
		data, err = update(r)
	case "DELETE":
		data, err = del(r)
	default:
		status := http.StatusMethodNotAllowed
		w.WriteHeader(status)
		logRequest(status)
		return
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
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
		logRequest(http.StatusOK)
	}
}

// main handles some flag defaults, connects to the database,
// and starts the http server.
func main() {
	flag.Usage = usage
	flag.Parse()

	var err error
	db, err = initDB()
	if err != nil {
		fmt.Printf("Unable to connect to database: %s\n", err)
		os.Exit(1)
	}

	http.HandleFunc("/", handleQuery)
	fmt.Println(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
