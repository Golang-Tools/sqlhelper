// Package sqlite 为 bunproxy 提供 SQLite 驱动实现。
//
// 应用侧空导入本包即可启用 sqlite:// 连接串:
//
//	import _ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
package sqlite

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/schema"
)

// Scheme sqlite 驱动支持的 URL scheme
const Scheme = "sqlite"

func init() {
	bunproxy.RegisterDriver(driver{})
}

// driver bunproxy.Driver 的 sqlite 实现
type driver struct{}

// Scheme 返回该驱动支持的 scheme
func (driver) Scheme() string { return Scheme }

// Dialect 返回 bun 的 sqlite 方言
func (driver) Dialect() schema.Dialect { return sqlitedialect.New() }

// NewPool 依据连接串创建连接池
func (driver) NewPool(rawURL string, opts *bunproxy.Options) (*sql.DB, error) {
	U, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	//只去掉 scheme 前缀,避免误伤路径中出现的同名内容
	dataSourceName := strings.TrimPrefix(rawURL, fmt.Sprintf("%s://", U.Scheme))
	//sqlite 的 DSN 为 file:xxx?params 形式,已经带 file: 前缀时不再重复添加
	dsn := dataSourceName
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		return nil, err
	}
	switch {
	case !strings.Contains(dataSourceName, ":memory:"),
		strings.Contains(dataSourceName, "cache=shared"):
		opts.ApplyPool(sqldb)
	default:
		//内存数据库的每个连接都是独立的库,需要限制为单连接以免不同连接看到不同的数据
		sqldb.SetMaxIdleConns(1)
		sqldb.SetMaxOpenConns(1)
		sqldb.SetConnMaxLifetime(0)
	}
	return sqldb, nil
}
