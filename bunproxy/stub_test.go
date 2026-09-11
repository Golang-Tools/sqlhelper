package bunproxy

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/url"

	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
)

// 本文件为核心模块提供测试用的假驱动:
//   - stubSQLDriver 是 database/sql 层面的最小实现,不依赖任何真实数据库
//   - stubDriver 是 bunproxy.Driver 层面的实现,注册 scheme "memory"
//
// 这样核心模块的 Proxy 生命周期/回调/并发/超时等测试无需引入真实驱动子模块,
// 真实数据库相关的行为测试请放在 driver/* 子模块中。

const (
	//stubSQLDriverName 假 database/sql 驱动名
	stubSQLDriverName = "bunproxy-stub"
	//unreachableHost 模拟连接不可用的地址
	unreachableHost = "unreachable"
)

// stubConnectorErr 模拟连接失败的错误
var stubConnectorErr = errors.New("stub: 数据库不可用")

// stubDriver bunproxy 层的假驱动
type stubDriver struct{}

func (stubDriver) Scheme() string { return "memory" }

func (stubDriver) Dialect() schema.Dialect { return sqlitedialect.New() }

func (stubDriver) NewPool(rawURL string, opts *Options) (*sql.DB, error) {
	U, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	sqldb, err := sql.Open(stubSQLDriverName, U.Host)
	if err != nil {
		return nil, err
	}
	opts.ApplyPool(sqldb)
	return sqldb, nil
}

// stubSQLDriver database/sql 层的假驱动
type stubSQLDriver struct{}

func (stubSQLDriver) Open(name string) (driver.Conn, error) {
	if name == unreachableHost {
		return nil, stubConnectorErr
	}
	return &stubConn{}, nil
}

// stubConn 假连接:实现 Ping/Query/Exec 的最小集合
type stubConn struct{}

func (c *stubConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }

func (c *stubConn) Close() error { return nil }

func (c *stubConn) Begin() (driver.Tx, error) { return stubTx{}, nil }

func (c *stubConn) Ping(context.Context) error { return nil }

func (c *stubConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &stubRows{}, nil
}

func (c *stubConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

// stubTx 假事务
type stubTx struct{}

func (stubTx) Commit() error { return nil }

func (stubTx) Rollback() error { return nil }

// stubRows 假结果集:始终返回一行一列,值为1
type stubRows struct{ done bool }

func (r *stubRows) Columns() []string { return []string{"n"} }

func (r *stubRows) Close() error { return nil }

func (r *stubRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(1)
	return nil
}

func init() {
	sql.Register(stubSQLDriverName, stubSQLDriver{})
	RegisterDriver(stubDriver{})
}
