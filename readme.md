sqld
====

Expose a database as a json serving http server

Install
-------
```
go get github.com/mmaelzer/sqld
```

Usage
-----
```
Usage of 'sqld':
	sqld -user=root -name=table_name -dsn=sql.example.com:3306

Flags:
  -dsn string
    	database source name (default: 'localhost:3306')
  -name string
    	database name
  -pass string
    	database password (default: '')
  -port string
    	http port (default: 8080
  -type string
    	database type (default: mysql)
  -user string
    	database username (default: root)
```

TODO
----
- [ ] Add proper Postgres support
- [ ] Add config file support
- [ ] Add support for stdin passing of a password
- [ ] Maybe add pagination in responses

License
-------
MIT
