sqld
====

Expose a database with an http server

Install
-------
```
go get github.com/mmaelzer/sqld
```

Usage
-----
```
Usage of 'sqld':
	sqld -user=root -name=table_name -port=8000

Flags:
  -dsn string
    	database source name (default "localhost:3306")
  -name string
    	database name
  -pass string
    	database password
  -port int
    	http port (default 8080)
  -type string
    	database type (default "mysql")
  -user string
    	database username (default "root")
```

Query
-----
Interact with the database via URLs.
```
http://localhost:8080/table_name
```

### With ID
The following equivalent to a request with `table_name?id=10`
```
http://localhost:8080/table_name/10
```

### Filtering
```
http://localhost:8080/table_name?id=10
http://localhost:8080/table_name?name=fred&age=67
```
### Limit
```
http://localhost:8080/table_name?__limit__=20&name=bob
```

### Offset
```
http://localhost:8080/table_name?__limit__=20&__offset__=100
```

Create
------
Create rows in the database via POST requests.
```
POST http://localhost:8080/table_name
```
### Request
```json
{
  "name": "jim",
  "age": 54
}
```

### Response (201)
```json
{
  "id": 10,
  "name": "jim",
  "age": 54
}
```

Create multiple rows in the database via a POST request with an array.
```
POST http://localhost:8080/table_name
```
### Request
```json
[
  { "name": "bill" },
  { "name": "nancy" },
  { "name": "chris" }
]
```
### Response (201)
```json
[
  {
    "id": 11,
    "name": "bill",
    "age": null
  },
  {
    "id": 12,
    "name": "nancy",
    "age": null
  },
  {
    "id": 13,
    "name": "chris",
    "age": null
  }
]
```


Update
------
Update a row in the database with PUT requests.
```
PUT http://localhost:8080/table_name/:id?where=clause
```
### Request
```json
{
  "name": "jill"
}
```

### Response (204)
Empty


Delete
------
Delete a row in the database with DELETE requests.
```
DELETE http://localhost:8080/table_name/:id?where=clause
```

### Response (204)
Empty


TODO
----
- [ ] Add proper Postgres support
- [ ] Add config file support
- [ ] Add support for stdin passing of a password
- [ ] Maybe add pagination in responses
- [ ] Add raw query support

License
-------
MIT
