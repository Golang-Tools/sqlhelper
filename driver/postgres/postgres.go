// Package postgres 为 bunproxy 提供 PostgreSQL 驱动实现。
//
// 应用侧空导入本包即可启用 postgres:// 连接串:
//
//	import _ "github.com/Golang-Tools/sqlhelper/driver/postgres/v4"
package postgres

import (
	"database/sql"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/schema"
)

// Scheme postgres 驱动支持的 URL scheme
const Scheme = "postgres"

func init() {
	bunproxy.RegisterDriver(driver{})
}

// driver bunproxy.Driver 的 postgres 实现
type driver struct{}

// Scheme 返回该驱动支持的 scheme
func (driver) Scheme() string { return Scheme }

// Dialect 返回 bun 的 postgres 方言
func (driver) Dialect() schema.Dialect { return pgdialect.New() }

// NewPool 依据连接串创建连接池
// pgdriver 是惰性连接,连接串非法时错误会在首次使用时暴露
func (driver) NewPool(rawURL string, opts *bunproxy.Options) (*sql.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(rawURL)))
	opts.ApplyPool(sqldb)
	return sqldb, nil
}
