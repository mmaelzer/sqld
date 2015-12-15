package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

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
	paths := strings.Split(r.URL.Path, "/")
	table := paths[1]
	query := sq.Select("*").From(table)

	if len(paths) > 2 && paths[2] != "" {
		query = query.Where(sq.Eq{"id": paths[2]})
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
			query = query.Where(sq.Eq{key: val})
		}
	}

	return query.ToSql()
}

func read(w http.ResponseWriter, r *http.Request) {
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

func create(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	paths := strings.Split(r.URL.Path, "/")
	table := paths[1]

	list, ok := data.([]interface{})
	if ok {
		manySaved, err := createMany(table, list)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if len(err) == 0 {
			json.NewEncoder(w).Encode(manySaved)
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errors":  err,
				"objects": manySaved,
			})
		}
		return
	}

	item, ok := data.(map[string]interface{})
	if ok {
		saved, err := createSingle(table, item)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(saved)
		}
	}

	w.WriteHeader(http.StatusBadRequest)
}

func update(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func del(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		read(w, r)
	case "POST":
		create(w, r)
	case "PUT":
		update(w, r)
	case "DELETE":
		del(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
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
