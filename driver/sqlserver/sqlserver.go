// Package sqlserver 为 bunproxy 提供 SQL Server 驱动实现。
//
// 应用侧空导入本包即可启用 sqlserver:// 连接串:
//
//	import _ "github.com/Golang-Tools/sqlhelper/driver/sqlserver/v4"
package sqlserver

import (
	"database/sql"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	_ "github.com/denisenkom/go-mssqldb"
	"github.com/uptrace/bun/dialect/mssqldialect"
	"github.com/uptrace/bun/schema"
)

// Scheme sqlserver 驱动支持的 URL scheme
const Scheme = "sqlserver"

// driverName go-mssqldb 注册的 database/sql 驱动名
const driverName = "sqlserver"

func init() {
	bunproxy.RegisterDriver(driver{})
}

// driver bunproxy.Driver 的 sqlserver 实现
type driver struct{}

// Scheme 返回该驱动支持的 scheme
func (driver) Scheme() string { return Scheme }

// Dialect 返回 bun 的 sqlserver 方言
func (driver) Dialect() schema.Dialect { return mssqldialect.New() }

// NewPool 依据连接串创建连接池
func (driver) NewPool(rawURL string, opts *bunproxy.Options) (*sql.DB, error) {
	sqldb, err := sql.Open(driverName, rawURL)
	if err != nil {
		return nil, err
	}
	opts.ApplyPool(sqldb)
	return sqldb, nil
}
