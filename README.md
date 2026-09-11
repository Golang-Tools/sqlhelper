# sqlhelper/v3

`uptrace/bun` 的代理对象,用于解决对常见关系数据库 pg、mysql、sqlserver 和 sqlite3 的连接与生命周期管理问题。

v3 是在 v2 基础上做的一次现代化改造:**尽量保持 API 形状不变,重点修 bug、补并发与生命周期能力、补齐工程基线**。模块路径为 `github.com/Golang-Tools/sqlhelper/v3`,可以与 v2 在同一个项目中并存,便于灰度迁移。

> API 文档见 [pkg.go.dev](https://pkg.go.dev/github.com/Golang-Tools/sqlhelper/v3/bunproxy);迁移请参考 [MIGRATION_v2_to_v3.md](./MIGRATION_v2_to_v3.md),变更明细见 [CHANGELOG.md](./CHANGELOG.md)。

## 为什么选择 bun

1. bun 对 postgresql 有更好的支持,原生支持 jsonb、array 等数据类型
2. 使用标准库 `database/sql` 的接口定义
3. 文档可读性更好些,虽然是英文

## 安装

```bash
go get github.com/Golang-Tools/sqlhelper/v3
```

本模块要求 **Go 1.25.0 及以上**。原因是 `github.com/uptrace/bun/driver/sqliteshim` 依赖的 `modernc.org/sqlite` 以及 `golang.org/x/*` 系列依赖自身声明的最低版本为 Go 1.25。如果需要支持更低的 Go 版本,需要同时降级 sqliteshim / modernc 系列依赖。

## 快速开始

```go
package main

import (
	"context"
	"fmt"

	"github.com/Golang-Tools/sqlhelper/v3/bunproxy"
)

func main() {
	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()

	// 初始化时会校验连接可用性,连接不可用或回调失败都会直接返回错误
	if err := proxy.Init("sqlite://:memory:",
		bunproxy.WithQueryTimeoutMS(3000),
		bunproxy.WithMaxOpenConns(10),
		bunproxy.WithMaxIdleConns(10),
	); err != nil {
		panic(err)
	}

	// NewCtx 会带上配置好的 QueryTimeout
	ctx, cancel := proxy.NewCtx()
	defer cancel()

	if _, err := proxy.Client().ExecContext(ctx, "CREATE TABLE t_user (id INTEGER, name TEXT)"); err != nil {
		panic(err)
	}

	// 健康检查
	if err := proxy.Health(context.Background()); err != nil {
		panic(err)
	}

	fmt.Println("ok")
}
```

## 支持的数据库

| 数据库 | scheme | 连接串示例 |
| --- | --- | --- |
| PostgreSQL | `postgres` | `postgres://user:pwd@localhost:5432/db?sslmode=disable` |
| MySQL | `mysql` | `mysql://user:pwd@localhost:3306/db?charset=utf8mb4` |
| SQL Server | `sqlserver` | `sqlserver://sa:pwd@localhost:1433?database=master` |
| SQLite | `sqlite` | `sqlite://test.db`、`sqlite://:memory:` |

其他 scheme 会返回 `bunproxy.ErrUnsupportedSchema`(可用 `errors.Is` 判断)。

补充说明:

- MySQL 的连接串**可以不带库名**(如 `mysql://root:pwd@localhost:3306`),库名在 `?` 参数或 SQL 中指定即可
- SQLite 的连接串支持直接写 sqlite 原生 DSN:已带 `file:` 前缀时不会被重复拼接,例如 `sqlite://file::memory:?cache=shared`、`sqlite://file:test.db?_pragma=busy_timeout(5000)`
- SQLite 的私有内存库(`:memory:` 且未声明 `cache=shared`)会被自动限制为单连接(`MaxOpenConns(1)`),避免不同连接看到不同的数据库副本;声明了 `cache=shared` 时按配置的连接池参数处理

## 生命周期

```go
proxy := bunproxy.New()      // 创建代理,此时不可用
proxy.Regist(cb)             // 注册回调(只能在初始化前)
proxy.Init(url, opts...)     // 建立连接 -> 校验可用性 -> 执行回调
proxy.IsOk()                 // 是否已经可用
proxy.Client()               // 获取 *bun.DB
proxy.PingContext(ctx)       // 连通性探测
proxy.Health(ctx)            // 带默认查询超时的连通性探测
proxy.Close()                // 关闭连接池并回到未初始化状态
```

约定:

- **创建者负责关闭**:`NewDB` 返回的 `*bun.DB` 在不再使用时必须 `Close`;`Init` 创建的连接由代理负责关闭。
- **失败即释放**:`Init` 在连接校验失败或 `SetConnect` 失败时会主动关闭刚创建的连接池,不会泄漏连接。
- **可以重来**:`Close` 之后代理回到未初始化状态,可以再次 `Init`。
- **关闭会清空回调**:`Close` 会清空已注册的回调,避免重新 `Init` 时重复执行回调里的副作用操作;需要时在重新 `Init` 前再次 `Register` 即可。
- **关闭后的错误可识别**:`Close` 之后 `Client()` 返回 `nil`、`IsOk()` 为 `false`,`PingContext`/`Health`/再次 `Close` 均返回 `ErrProxyNotSetClient`。
- **关闭不再改写连接指针**:`Close` 只关闭连接池并置位内部状态,因此关闭路径与业务查询并发时不会产生数据竞争(关闭后直接使用 `proxy.DB` 会得到 `sql: database is closed` 错误,而不是空指针 panic)。

## 配置项

### Options 字段

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `Parallelcallback` | `bool` | 初始化回调是否并行执行,默认串行 |
| `QueryTimeout` | `time.Duration` | 默认查询超时,`0` 表示不设置 |
| `MaxOpenConns` | `int` | 连接池最大连接数,`0` 表示使用 `database/sql` 默认值 |
| `MaxIdleConns` | `int` | 连接池最大空闲连接数,`0` 表示使用 `database/sql` 默认值 |
| `ConnMaxLifetime` | `time.Duration` | 连接最大存活时间 |
| `ConnMaxIdleTime` | `time.Duration` | 连接最大空闲时间 |
| `DiscardUnknownColumns` | `bool` | 查询结果存在未知列时不报错 |
| `QueryLog` | `bool` | 开启 SQL 日志,默认关闭 |
| `QueryLogArgs` | `bool` | SQL 日志中是否带上参数值,默认关闭 |
| `DisablePingOnInit` | `bool` | 关闭 `Init` 时的连通性校验 |
| `PingTimeout` | `time.Duration` | 连通性校验超时,`0` 表示使用 `DefaultPingTimeout`(5s) |
| `IgnoreCallbackError` | `bool` | 回调报错不影响 `Init` 的返回值(兼容旧行为) |
| `CallbackTimeout` | `time.Duration` | 回调执行的超时,`0` 表示不限制(只对 `RegisterContext` 注册的回调生效) |
| `ConnectRetryAttempts` | `int` | 建立连接的总尝试次数(含首次),`0`/`1` 表示不重试 |
| `ConnectRetryInterval` | `time.Duration` | 重试的首次等待时间,`0` 表示使用 `DefaultConnectRetryInterval`(500ms) |

`NewDB(url, nil)` 时会使用 `bunproxy.DefaultOpts`(最大连接 10、最大空闲 10、存活 1h、空闲 10min)。

### 选项函数

单位带 `MS` 后缀的选项入参为毫秒,不带后缀的入参为 `time.Duration`:

```go
WithQueryTimeoutMS(ms int) / WithQueryTimeout(d time.Duration)
WithMaxOpenConns(n int) / WithMaxIdleConns(n int)
WithConnMaxLifetimeMS(ms int) / WithConnMaxLifetime(d time.Duration)
WithConnMaxIdleTimeMS(ms int) / WithConnMaxIdleTime(d time.Duration)
WithPoolConfig(maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration)
WithDiscardUnknownColumns()
WithParallelCallback()
WithQueryLog() / WithQueryLogArgs()
WithPingTimeoutMS(ms int) / WithDisablePingOnInit()
WithIgnoreCallbackError()
WithOptions(opts Options)     // 整体覆盖配置,用于把布尔开关改回false
WithDefaultOpts()             // 一键套用DefaultOpts(与NewDB(url,nil)一致)
WithCallbackTimeout(d) / WithCallbackTimeoutMS(ms)
WithConnectRetry(attempts int, interval time.Duration)
```

## 回调

```go
proxy := bunproxy.New()
_ = proxy.Regist(func(cli *bun.DB) error {
	_, err := cli.ExecContext(context.Background(), "CREATE TABLE IF NOT EXISTS t_user (id INTEGER)")
	return err
})
_ = proxy.Init("sqlite://test.db")
```

- 回调只能在初始化前注册,初始化后再注册会返回 `ErrProxyAlreadySetClient`
- 串行回调按注册顺序执行;`WithParallelCallback()` 时并行执行,但 `Init` 会**等待全部回调结束**后才返回
- 回调 panic 会被捕获并转换为 `ErrCallbackPanic`,不会导致进程崩溃
- 回调返回的错误会被聚合,可以通过 `errors.Is/As` 获取到原始错误与回调下标(`*CallbackError`)
- 需要兼容旧版本"回调错误只记日志"的行为时,使用 `WithIgnoreCallbackError()`

需要做带超时保护的操作时,用 `RegisterContext` 注册带上下文的回调:

```go
_ = proxy.RegisterContext(func(ctx context.Context, cli *bun.DB) error {
	// ctx 会带上 CallbackTimeout 配置的超时,可用于建表、预热、健康探测等操作
	return cli.PingContext(ctx)
})
_ = proxy.Init(url, bunproxy.WithCallbackTimeoutMS(5000))
```

- `RegisterContext` 与 `Regist` 注册的回调**按注册顺序统一执行**,并行模式下也一并并行
- 只有 `RegisterContext` 注册的回调会收到带超时的上下文;`CallbackTimeout` 为 `0`(默认)表示不限制
- 超时后回调会收到 `context.DeadlineExceeded`,`Init` 会因此失败并回滚连接

## 超时与上下文

```go
ctx, cancel := proxy.NewCtx()                       // 基于配置的 QueryTimeout
ctx, cancel := proxy.NewCtxWithParent(reqCtx)       // 继承请求上下文,推荐
timeout := proxy.DefaultQueryTimeout()
```

`Init` 之后由调用方自行把 `ctx` 传给 `bun` 的 `ExecContext/QueryContext` 等方法。**不要使用不带 `ctx` 的 `Exec/Query`**,否则超时与取消会失效。

各类超时的适用对象:

| 配置 | 适用范围 |
| --- | --- |
| `QueryTimeout` | `NewCtx`/`NewCtxWithParent` 生成的上下文,供业务查询使用 |
| `PingTimeout` | `Init` 的连通性校验与 `Health`,默认 5s,可用 `proxy.PingTimeout()` 读取 |
| `CallbackTimeout` | `RegisterContext` 注册的回调 |
| `ConnectRetryInterval` | `Init` 重试连接时的等待间隔 |

## 连接建立重试

数据库容器或实例启动晚于应用时,可以用重试避免"启动即失败":

```go
if err := proxy.Init(url,
	bunproxy.WithConnectRetry(5, 500*time.Millisecond), // 总尝试5次,首次等待500ms
	bunproxy.WithPingTimeoutMS(2000),
); err != nil {
	// 重试耗尽后会返回 ErrPingFailed
}
```

- 重试只针对**连通性校验失败**(`ErrPingFailed`);DSN 非法、scheme 不支持等配置类错误会立即返回,不会重试
- 等待时间按指数退避增长,单次上限 5s;`ConnectRetryAttempts` 为 `0`/`1` 时保持原有的一次性行为
- `WithDisablePingOnInit()` 关闭了连通性校验时没有可重试的判定依据,重试不会生效

## 日志与安全

默认情况下**不开启** SQL 日志,避免 SQL 与参数值泄漏到日志中。开启后日志只包含脱敏后的 SQL 模板:

```go
bunproxy.WithQueryLog()       // 记录 操作类型/耗时/脱敏SQL
bunproxy.WithQueryLogArgs()   // 额外记录参数值,存在泄漏风险,谨慎使用
```

- SQL 中的字符串字面量会被替换为 `'?'`、数字字面量会被替换为 `?`,连续空白会被压缩,超长 SQL 会被截断
- 带数字的标识符(如 `col1`、`utf8mb4`)、问号占位符与 postgres 的位置占位符(`$1`)不会被误伤,`NULL`/`TRUE` 等关键字保持原样以便对照
- `bunproxy.RedactDSN(dsn)` 用于对连接串脱敏,`bunproxy.SanitizeSQL(sql)` 用于对 SQL 脱敏
- `Init` 的错误信息中的连接串已经过脱敏处理,不会带出明文密码
- 动态输入必须走 bun 的参数化 API,禁止使用 `fmt.Sprintf` 拼接 SQL

## 错误处理

| 错误 | 说明 |
| --- | --- |
| `ErrEmptyURL` | 连接串为空 |
| `ErrUnsupportedSchema` | 未支持的数据库类型 |
| `ErrUnknownClientType` | 未知的客户端类型 |
| `ErrProxyAlreadySetClient` | 代理已经设置过客户端 |
| `ErrProxyNotSetClient` | 代理还未设置客户端 |
| `ErrNilDB` / `ErrNilCallback` | 入参为 nil |
| `ErrPingFailed` | 初始化时的连通性校验失败 |
| `ErrCallbackPanic` | 回调执行时发生 panic |

```go
if err := proxy.Init(url); err != nil {
	if errors.Is(err, bunproxy.ErrPingFailed) {
		// 连接不可用
	}
	var cbErr *bunproxy.CallbackError
	if errors.As(err, &cbErr) {
		log.Printf("第 %d 个回调失败: %v", cbErr.Index, cbErr.Err)
	}
}
```

v2 的历史命名(`ErrProxyAllreadySettedUniversalClient`、`ErrProxyNotYetSettedUniversalClient`、`ErrUnSupportSchema`)仍然保留,并且与新命名指向同一个错误值,`errors.Is` 对两者都成立。

## 并发说明

- `IsOk`、`Client`、`PingContext`、`Health`、`Regist`、`Close`、`DefaultQueryTimeout` 以及回调执行都受内部读写锁保护
- `Init` 与 `Close` 属于生命周期操作,应在应用启动/退出阶段串行调用,不要与业务查询并发执行
- 回调在锁外执行,但**不要在回调里调用代理的 `Init`/`Close`**,否则可能造成死锁

## 示例

仓库提供了可直接运行的示例(godoc 示例见 `bunproxy/example_test.go`):

| 示例 | 说明 | 运行方式 |
| --- | --- | --- |
| [examples/sqlite](./examples/sqlite/main.go) | SQLite 基础用法:建表/插入/查询/健康检查 | `go run ./examples/sqlite` |
| [examples/callbacks](./examples/callbacks/main.go) | 回调注册、并行执行、错误聚合与回滚 | `go run ./examples/callbacks` |
| [examples/querylog](./examples/querylog/main.go) | SQL 日志开关与连接串/SQL 脱敏 | `go run ./examples/querylog` |

## 从 v2 迁移

模块路径改为 `/v3`,因此可以并存。绝大多数代码只需要改 import 路径;需要留意的行为变化见 [MIGRATION_v2_to_v3.md](./MIGRATION_v2_to_v3.md)。

## 开发

```bash
go build ./...
go vet ./...
gofmt -l .                           # 应为空
go test ./...                        # 运行测试
go test -race -covermode=atomic -coverprofile=coverage.out ./...
go test -bench=. -benchmem -run=^$ ./...
```

CI 会执行格式化检查、`go mod tidy` 校验、`go vet`、`go test -race`、覆盖率统计、benchmark,以及(非阻塞的)lint 与 `govulncheck`。

### 真实数据库集成测试

`bunproxy/integration_test.go` 中的集成测试通过环境变量开关,未配置的数据库会自动跳过:

| 环境变量 | 示例 |
| --- | --- |
| `SQLHELPER_TEST_SQLITE_URL` | `sqlite:///tmp/sqlhelper_it.db` |
| `SQLHELPER_TEST_MYSQL_URL` | `mysql://root:root@127.0.0.1:3306/sqlhelper?charset=utf8mb4` |
| `SQLHELPER_TEST_POSTGRES_URL` | `postgres://postgres:postgres@127.0.0.1:5432/sqlhelper?sslmode=disable` |
| `SQLHELPER_TEST_SQLSERVER_URL` | `sqlserver://sa:Your_password123@127.0.0.1:1433?database=master` |

本地用容器跑一遍:

```bash
docker run -d --name sqlhelper-it-mysql \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=sqlhelper \
  -p 13306:3306 mysql:8.4

docker run -d --name sqlhelper-it-postgres \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=sqlhelper \
  -p 15432:5432 postgres:17

SQLHELPER_TEST_MYSQL_URL='mysql://root:root@127.0.0.1:13306/sqlhelper?charset=utf8mb4' \
SQLHELPER_TEST_POSTGRES_URL='postgres://postgres:postgres@127.0.0.1:15432/sqlhelper?sslmode=disable' \
SQLHELPER_TEST_SQLITE_URL='sqlite:///tmp/sqlhelper_it.db' \
  go test -race -count=1 -v -run 'TestIntegration' ./bunproxy/

docker rm -f sqlhelper-it-mysql sqlhelper-it-postgres
```

集成测试覆盖:回调执行、建表、批量写入、单行查询、Count、事务更新、带超时的查询上下文、重复初始化被拒、关闭后重开(不重复执行回调)。CI 中使用 GitHub Actions service 容器运行 MySQL 与 PostgreSQL。
