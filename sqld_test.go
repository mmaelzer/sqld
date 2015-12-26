package main

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestData struct {
	A string `db:"a"`
	B string `db:"b"`
}

func createDB() {
	*dbtype = "sqlite3"
	*dsn = ":memory:"
	db, _ = initDB()
	db.MustExec("CREATE TABLE t1(a, b PRIMARY KEY)")
	db.MustExec("INSERT INTO t1 (a, b) VALUES ('hi', 'there')")
	db.MustExec("INSERT INTO t1 (a, b) VALUES ('how', 'dy')")
}

func TestNewError(t *testing.T) {
	assert := assert.New(t)

	err := NewError(errors.New("uh oh"), 400)
	assert.Equal(err.Code, 400)
	assert.Equal(err.Error(), "uh oh")

	assert.Equal(NewError(nil, 100).Error(), "")
}

func TestBadRequest(t *testing.T) {
	assert := assert.New(t)

	err := BadRequest(errors.New("bad request"))
	assert.Equal(err.Code, 400)
	assert.Equal(err.Error(), "bad request")
}

func TestNotFound(t *testing.T) {
	assert := assert.New(t)

	err := NotFound(errors.New("not found"))
	assert.Equal(err.Code, 404)
	assert.Equal(err.Error(), "not found")
}

func TestInternalError(t *testing.T) {
	assert := assert.New(t)

	err := InternalError(errors.New("internal error"))
	assert.Equal(err.Code, 500)
	assert.Equal(err.Error(), "internal error")
}

func TestBuildDSN(t *testing.T) {
	assert := assert.New(t)

	*dsn = "user:pass@localhost:3306/user"
	d := buildDSN()
	assert.Equal(*dsn, d)

	*dsn = ""
	*host = "localhost:3306"
	*dbtype = "mysql"
	*dbname = "user"

	d = buildDSN()
	assert.Equal(d, "root:@(localhost:3306)/user?parseTime=true")

	*host = ""
	d = buildDSN()
	assert.Equal(d, "root:@(localhost:3306)/user?parseTime=true")

	*dbtype = "postgres"
	*host = ""
	d = buildDSN()
	assert.Equal(d, "postgres://root:@localhost:5432/user?sslmode=disable")

	*dbtype = "sqlite3"
	*host = ""
	*dsn = ":memory:"
	d = buildDSN()
	assert.Equal(d, ":memory:")

	*dsn = ""
	*user = ""
	buildDSN()
	assert.Equal(*user, "root")

	*dbtype = "unknown"
	assert.Equal(buildDSN(), "")
}

func TestBuildSelectQuery(t *testing.T) {
	assert := assert.New(t)

	*dbtype = "sqlite3"
	*dsn = ":memory:"
	// This is needed to setup the squirrel query building package
	initDB()

	req, _ := http.NewRequest("GET", "http://example.com/user", nil)
	sql, args, err := buildSelectQuery(req)

	assert.Nil(err)
	assert.Nil(args)
	assert.Equal(sql, "SELECT * FROM user")

	req, _ = http.NewRequest("GET", "http://example.com/user?name=jim", nil)
	sql, args, err = buildSelectQuery(req)

	assert.Nil(err)
	assert.Equal(args, []interface{}{"jim"})
	assert.Equal(sql, "SELECT * FROM user WHERE name IN (?)")

	req, _ = http.NewRequest("GET", "http://example.com/user?__limit__=100&__offset__=200", nil)
	sql, args, err = buildSelectQuery(req)

	assert.Nil(err)
	assert.Nil(args)
	assert.Equal(sql, "SELECT * FROM user LIMIT 100 OFFSET 200")

	req, _ = http.NewRequest("GET", "http://example.com/user/10", nil)
	sql, args, err = buildSelectQuery(req)

	assert.Nil(err)
	assert.Equal(args, []interface{}{"10"})
	assert.Equal(sql, "SELECT * FROM user WHERE id = ?")
}

func TestBuildUpdateQuery(t *testing.T) {
	assert := assert.New(t)

	*dbtype = "sqlite3"
	*dsn = ":memory:"
	// This is needed to setup the squirrel query building package
	initDB()

	data := map[string]interface{}{
		"name": "jack",
	}

	req, _ := http.NewRequest("PUT", "http://example.com/user/8", nil)
	sql, args, err := buildUpdateQuery(req, data)
	assert.Nil(err)
	assert.Equal(args, []interface{}{"jack", "8"})
	assert.Equal(sql, "UPDATE user SET name = ? WHERE id = ?")

	data = map[string]interface{}{
		"age": 66,
	}
	req, _ = http.NewRequest("PUT", "http://example.com/user?name=jill&__limit__=5", nil)
	sql, args, err = buildUpdateQuery(req, data)
	assert.Nil(err)
	assert.Equal(args, []interface{}{66, "jill"})
	assert.Equal(sql, "UPDATE user SET age = ? WHERE name IN (?) LIMIT 5")
}

func TestBuildDeleteQuery(t *testing.T) {
	assert := assert.New(t)

	*dbtype = "sqlite3"
	*dsn = ":memory:"
	// This is needed to setup the squirrel query building package
	initDB()

	req, _ := http.NewRequest("DELETE", "http://example.com/user/8", nil)
	sql, args, err := buildDeleteQuery(req)
	assert.Nil(err)
	assert.Equal(args, []interface{}{"8"})
	assert.Equal(sql, "DELETE FROM user WHERE id = ?")

	req, _ = http.NewRequest("DELETE", "http://example.com/user?name=jill&__limit__=5", nil)
	sql, args, err = buildDeleteQuery(req)
	assert.Nil(err)
	assert.Equal(args, []interface{}{"jill"})
	assert.Equal(sql, "DELETE FROM user WHERE name IN (?) LIMIT 5")
}

func TestReadQuery(t *testing.T) {
	assert := assert.New(t)

	createDB()
	defer closeDB()

	args := make([]interface{}, 0)
	data, err := readQuery("SELECT * FROM t1", args)
	assert.Nil(err)
	assert.Contains([]string{"hi", "how"}, data[0]["a"])
	assert.Contains([]string{"there", "dy"}, data[0]["b"])
}

func TestRead(t *testing.T) {
	assert := assert.New(t)

	createDB()
	defer closeDB()

	req, _ := http.NewRequest("GET", "http://example.com/t1", nil)
	d, err := read(req)
	data := d.([]map[string]interface{})
	assert.Nil(err)
	assert.Contains([]string{"hi", "how"}, data[0]["a"])
	assert.Contains([]string{"there", "dy"}, data[0]["b"])
}

func TestCreateSingle(t *testing.T) {
	assert := assert.New(t)

	createDB()
	defer closeDB()

	data, err := createSingle("t1", map[string]interface{}{
		"a": "boop",
		"b": "doop",
	})

	assert.Nil(err)
	assert.Equal(data["a"], "boop")
	assert.Equal(data["b"], "doop")
	assert.True(data["id"].(int64) > 0)
}

func TestCreate(t *testing.T) {
	assert := assert.New(t)

	createDB()
	defer closeDB()

	b := bytes.NewBufferString(`{
		"a": "boop",
		"b": "doop"
	}`)
	req, _ := http.NewRequest("POST", "http://example.com/t1", b)
	d, err := create(req)
	data := d.(map[string]interface{})
	assert.Nil(err)
	assert.Equal(data["a"], "boop")
	assert.Equal(data["b"], "doop")
	assert.NotNil(data["id"])

	b = bytes.NewBufferString(`{
		"a": "boop",
		"b": 
	`)
	req, _ = http.NewRequest("POST", "http://example.com/t1", b)
	d, err = create(req)
	assert.Nil(d)
	assert.Equal(err.Code, 400)

	b = bytes.NewBufferString(`
	[
		{ "a": "b" },
		{ "c": "d" },
		{ "e": "f" }
	]
	`)
	req, _ = http.NewRequest("POST", "http://example.com/t1", b)
	d, err = create(req)
	assert.Nil(d)
	assert.Equal(err.Code, 400)
}

func TestUpdate(t *testing.T) {
	assert := assert.New(t)

	createDB()
	defer closeDB()

	b := bytes.NewBufferString(`{
		"b": "updated"
	}`)
	req, _ := http.NewRequest("PUT", "http://example.com/t1?a=hi", b)
	d, err := update(req)
	assert.Nil(err)
	assert.Nil(d)

	data := TestData{}
	db.Get(&data, "SELECT * FROM t1 WHERE a=?", "hi")
	assert.Equal(data.A, "hi")
	assert.Equal(data.B, "updated")

	b = bytes.NewBufferString(`{
		"a": "boop",
		"b": 
	`)
	req, _ = http.NewRequest("PUT", "http://example.com/t1/t1?a=hi", b)
	d, err = update(req)
	assert.Nil(d)
	assert.Equal(err.Code, 400)
}

func TestDel(t *testing.T) {
	assert := assert.New(t)

	createDB()
	defer closeDB()

	req, _ := http.NewRequest("DELETE", "http://example.com/t1?a=hi", nil)
	d, sqldErr := del(req)

	assert.Nil(d)
	assert.Nil(sqldErr)

	data := TestData{}
	err := db.Get(&data, "SELECT * FROM t1 WHERE a=?", "hi")
	assert.Equal(err.Error(), "sql: no rows in result set")
	assert.Equal(data.A, "")
	assert.Equal(data.B, "")
}
