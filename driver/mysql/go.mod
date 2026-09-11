module github.com/Golang-Tools/sqlhelper/driver/mysql/v4

go 1.25.0

require (
	github.com/Golang-Tools/sqlhelper/v4 v4.0.0
	github.com/go-sql-driver/mysql v1.9.3
	github.com/uptrace/bun v1.2.18
	github.com/uptrace/bun/dialect/mysqldialect v1.2.18
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/Golang-Tools/loggerhelper/v4 v4.0.0 // indirect
	github.com/Golang-Tools/optparams v1.0.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
)

replace github.com/Golang-Tools/sqlhelper/v4 => ../../
