package bunproxy

import (
	"database/sql"
	"fmt"
	"sort"
	"sync"

	"github.com/uptrace/bun/schema"
)

// Driver 描述一个数据库驱动实现。
//
// 核心包不再内置任何具体数据库实现,postgres/mysql/sqlserver/sqlite 分别由
// github.com/Golang-Tools/sqlhelper/driver/<name>/v4 子模块实现并在 init 中
// 调用 RegisterDriver 注册;应用侧按需空导入对应驱动包即可启用。
//
// 同一个 scheme 只允许注册一次,重复注册会 panic,避免静默覆盖。
type Driver interface {
	// Scheme 返回该驱动支持的 URL scheme,例如 "postgres"、"sqlite"
	Scheme() string
	// NewPool 依据连接串创建底层连接池,并按驱动自身特性应用连接池配置
	// 实现方应在创建失败时保证不泄漏连接池,并自行处理该数据库特有的 DSN 规则
	NewPool(rawURL string, opts *Options) (*sql.DB, error)
	// Dialect 返回该驱动对应的 bun 方言
	Dialect() schema.Dialect
}

var (
	driversMu sync.RWMutex
	drivers   = map[string]Driver{}
)

// RegisterDriver 注册一个数据库驱动,供 driver 子模块在 init 中调用。
// 重复注册同一个 scheme 会 panic;nil 驱动会被忽略。
func RegisterDriver(d Driver) {
	if d == nil {
		return
	}
	scheme := d.Scheme()
	if scheme == "" {
		panic("bunproxy: 驱动的 scheme 不能为空")
	}
	driversMu.Lock()
	defer driversMu.Unlock()
	if _, exists := drivers[scheme]; exists {
		panic(fmt.Sprintf("bunproxy: scheme %q 已经注册过驱动", scheme))
	}
	drivers[scheme] = d
}

// FindDriver 查询指定 scheme 对应的驱动
func FindDriver(scheme string) (Driver, bool) {
	driversMu.RLock()
	defer driversMu.RUnlock()
	d, ok := drivers[scheme]
	return d, ok
}

// RegisteredSchemes 返回当前已注册的 scheme 列表(按字典序排列)。
// 适合在服务启动时打印,用于确认二进制内实际包含哪些数据库后端。
func RegisteredSchemes() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	schemes := make([]string, 0, len(drivers))
	for scheme := range drivers {
		schemes = append(schemes, scheme)
	}
	sort.Strings(schemes)
	return schemes
}
