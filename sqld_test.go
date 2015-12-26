package main

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewError(t *testing.T) {
	assert := assert.New(t)

	err := NewError(errors.New("uh oh"), 400)
	assert.Equal(err.Code, 400)
	assert.Equal(err.Error(), "uh oh")
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

	req, _ := http.NewRequest("PUT", "http://example.com/user/8?__limit__=1", nil)
	sql, args, err := buildUpdateQuery(req, data)
	assert.Nil(err)
	assert.Equal(args, []interface{}{"jack", "8"})
	assert.Equal(sql, "UPDATE user SET name = ? WHERE id = ? LIMIT 1")
}
