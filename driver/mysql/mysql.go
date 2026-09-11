// Package mysql 为 bunproxy 提供 MySQL 驱动实现。
//
// 应用侧空导入本包即可启用 mysql:// 连接串:
//
//	import _ "github.com/Golang-Tools/sqlhelper/driver/mysql/v4"
package mysql

import (
	"database/sql"
	"fmt"
	"net/url"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/schema"
)

// Scheme mysql 驱动支持的 URL scheme
const Scheme = "mysql"

func init() {
	bunproxy.RegisterDriver(driver{})
}

// driver bunproxy.Driver 的 mysql 实现
type driver struct{}

// Scheme 返回该驱动支持的 scheme
func (driver) Scheme() string { return Scheme }

// Dialect 返回 bun 的 mysql 方言
func (driver) Dialect() schema.Dialect { return mysqldialect.New() }

// NewPool 依据连接串创建连接池
func (driver) NewPool(rawURL string, opts *bunproxy.Options) (*sql.DB, error) {
	dsn, err := buildDSN(rawURL)
	if err != nil {
		return nil, err
	}
	sqldb, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	opts.ApplyPool(sqldb)
	return sqldb, nil
}

// buildDSN 把 URL 形式的连接串转换为 go-sql-driver/mysql 需要的 DSN。
// DSN 形如 user:pass@tcp(host:port)/db?params,即使不指定库名也必须保留斜杠。
func buildDSN(rawURL string) (string, error) {
	U, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	userinfo := ""
	username := U.User.Username()
	pwd, ok := U.User.Password()
	switch {
	case ok && username != "":
		userinfo = fmt.Sprintf("%s:%s@", username, pwd)
	case ok && username == "":
		userinfo = fmt.Sprintf(":%s@", pwd)
	case !ok && username != "":
		userinfo = fmt.Sprintf("%s@", username)
	}
	dbPath := U.Path
	if dbPath == "" {
		dbPath = "/"
	}
	return fmt.Sprintf("%stcp(%s)%s?%s", userinfo, U.Host, dbPath, U.RawQuery), nil
}
