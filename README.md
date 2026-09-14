# sqlhelper/v4

`uptrace/bun` 的代理对象,用于解决 PostgreSQL、MySQL、SQL Server 与 SQLite 的连接、连接池与生命周期管理问题。

v4 的核心变化:**数据库驱动改为按需引入的独立子模块**,核心模块不再依赖任何具体数据库实现。

- **用法与 v3 基本一致**:代理对象常驻,数据库还没连上时业务代码就可以正常书写、构建查询
- **需要哪个后端就导入哪个驱动**,不需要的后端不会进入你的依赖(例如只用 postgres 时不会引入 SQLite 的 `modernc` 依赖)
- 核心模块只依赖 `bun` 与 `database/sql`

> API 文档见 [pkg.go.dev](https://pkg.go.dev/github.com/Golang-Tools/sqlhelper/v4/bunproxy);从 v3 升级请参考 [MIGRATION_v3_to_v4.md](./MIGRATION_v3_to_v4.md),变更明细见 [CHANGELOG.md](./CHANGELOG.md)。

## 为什么选择 bun

1. bun 对 postgresql 有更好的支持,原生支持 jsonb、array 等数据类型
2. 使用标准库 `database/sql` 的接口定义
3. 文档可读性更好些,虽然是英文

## 安装

```bash
go get github.com/Golang-Tools/sqlhelper/v4
```

然后按需引入驱动(空导入即启用对应的 URL scheme):

```bash
go get github.com/Golang-Tools/sqlhelper/driver/postgres/v4
```

本模块要求 **Go 1.25.0 及以上**(由 `modernc.org/sqlite` 与 `golang.org/x/*` 依赖链决定)。

## 驱动

| 后端 | 模块路径 | URL scheme | 发布 tag |
| --- | --- | --- | --- |
| PostgreSQL | `github.com/Golang-Tools/sqlhelper/driver/postgres/v4` | `postgres` | `driver/postgres/v4.0.0` |
| MySQL | `github.com/Golang-Tools/sqlhelper/driver/mysql/v4` | `mysql` | `driver/mysql/v4.0.0` |
| SQL Server | `github.com/Golang-Tools/sqlhelper/driver/sqlserver/v4` | `sqlserver` | `driver/sqlserver/v4.0.0` |
| SQLite | `github.com/Golang-Tools/sqlhelper/driver/sqlite/v4` | `sqlite` | `driver/sqlite/v4.0.0` |
| 全部后端 | `github.com/Golang-Tools/sqlhelper/driver/all/v4` | 上述全部 | `driver/all/v4.0.0` |

```go
import (
	// 需要哪个就导哪个;一次性引入全部可用 driver/all/v4
	_ "github.com/Golang-Tools/sqlhelper/driver/postgres/v4"
	_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
)
```

未导入的 scheme 在 `Init` 时会返回 `ErrUnsupportedSchema`,错误信息里会列出**当前已注册的驱动**并提示需要导入哪个包——服务启动阶段即可发现"配置与二进制不匹配"。

### 同一个二进制支持多个后端

这是本模块的常见用法:**停机改配置、重启即切换后端**,无需改动代码或重新编译(只要该后端在编译期被引入)。

```go
import (
	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	_ "github.com/Golang-Tools/sqlhelper/driver/all/v4" // 一次引入四家
)

func main() {
	// 启动日志里打印实际支持的后端,便于运维核对配置
	log.Println("supported backends:", bunproxy.RegisteredSchemes())

	// URL 来自配置:dev 用 sqlite,staging 用 mysql,prod 用 postgres
	if err := bunproxy.Default.Init(cfg.DatabaseURL); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
}
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"

	//空导入即启用 sqlite://,换成其它后端只需改这一行
	_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
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

## 支持的连接串

| 数据库 | 连接串示例 |
| --- | --- |
| PostgreSQL | `postgres://user:pwd@localhost:5432/db?sslmode=disable` |
| MySQL | `mysql://user:pwd@localhost:3306/db?charset=utf8mb4` |
| SQL Server | `sqlserver://sa:pwd@localhost:1433?database=master` |
| SQLite | `sqlite://test.db`、`sqlite://:memory:` |

补充说明:

- MySQL 的连接串**可以不带库名**(如 `mysql://root:pwd@localhost:3306`),库名在 `?` 参数或 SQL 中指定即可
- SQLite 支持直接写原生 DSN:已带 `file:` 前缀时不会被重复拼接,例如 `sqlite://file::memory:?cache=shared`
- SQLite 的私有内存库(`:memory:` 且未声明 `cache=shared`)会被自动限制为单连接,避免不同连接看到不同的数据库副本

## 生命周期

```go
proxy := bunproxy.New()      // 创建代理,此时不可用但可以持有、可以提前构建查询
proxy.Register(cb)           // 注册回调(只能在初始化前)
proxy.Init(url, opts...)     // 建立连接 -> 校验可用性 -> 执行回调
proxy.IsReady()              // 是否已经可用(IsOk 为兼容别名)
proxy.Client()               // 获取 *bun.DB
proxy.PingContext(ctx)       // 连通性探测
proxy.Health(ctx)            // 带连通性探测超时的健康检查
proxy.Close()                // 关闭连接池并回到未初始化状态
```

约定:

- **代理常驻**:`Proxy`(尤其是 `bunproxy.Default`)从进程启动就存在且地址稳定,业务代码可以长期持有;`Init` 只是"稍后把连接接上"
- **`Init`/`Close` 是生命周期操作**:约定只在应用启动/退出阶段串行调用,不要与业务查询并发
- **初始化可取消**:需要整体超时或可取消的初始化用 `proxy.InitContext(ctx, url, ...)`
- **创建者负责关闭**:`NewDB` 返回的 `*bun.DB` 在不再使用时必须 `Close`;`Init` 创建的连接由代理负责关闭
- **失败即释放**:`Init` 在连接校验失败或回调失败时会主动关闭刚创建的连接池
- **关闭会清空回调**:避免重新 `Init` 时重复执行回调里的副作用操作;需要时再次 `Register` 即可

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
_ = proxy.Register(func(cli *bun.DB) error {
	_, err := cli.ExecContext(context.Background(), "CREATE TABLE IF NOT EXISTS t_user (id INTEGER)")
	return err
})
_ = proxy.Init("sqlite://test.db")
```

需要超时保护的操作可以用带上下文的回调:

```go
_ = proxy.RegisterContext(func(ctx context.Context, cli *bun.DB) error {
	// ctx 会带上 CallbackTimeout 配置的超时
	return cli.PingContext(ctx)
})
_ = proxy.Init(url, bunproxy.WithCallbackTimeoutMS(5000))
```

- 回调只能在初始化前注册,初始化后再注册会返回 `ErrProxyAlreadySetClient`
- `Register` 与 `RegisterContext` 注册的回调**按注册顺序统一执行**;`WithParallelCallback()` 时并行执行,但 `Init` 会等待全部回调结束
- 回调 panic 会被捕获并转换为 `ErrCallbackPanic`,不会导致进程崩溃
- 回调返回的错误会被聚合,可以通过 `errors.Is/As` 获取原始错误与回调下标(`*CallbackError`)
- 需要兼容旧版本"回调错误只记日志"的行为时,使用 `WithIgnoreCallbackError()`

## 超时与上下文

```go
ctx, cancel := proxy.NewCtx()                 // 基于配置的 QueryTimeout
ctx, cancel := proxy.NewCtxWithParent(reqCtx) // 继承请求上下文,推荐
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
	bunproxy.WithConnectRetry(5, 500*time.Millisecond),
	bunproxy.WithPingTimeoutMS(2000),
); err != nil {
	// 重试耗尽后会返回 ErrPingFailed
}
```

- 重试只针对**连通性校验失败**(`ErrPingFailed`);DSN 非法、scheme 不支持等配置类错误会立即返回
- 等待时间按指数退避增长,单次上限 5s;`ConnectRetryAttempts` 为 `0`/`1` 时保持原有的一次性行为
- 使用 `InitContext` 时,重试等待会随 ctx 一起取消

## 日志与安全

默认情况下**不开启** SQL 日志,避免 SQL 与参数值泄漏到日志中。开启后日志只包含脱敏后的 SQL 模板:

```go
bunproxy.WithQueryLog()       // 记录 操作类型/耗时/脱敏SQL
bunproxy.WithQueryLogArgs()   // 额外记录参数值,存在泄漏风险,谨慎使用
```

- SQL 中的字符串字面量会被替换为 `'?'`、数字字面量会被替换为 `?`,连续空白会被压缩,超长 SQL 会被截断
- 带数字的标识符(如 `col1`、`utf8mb4`)、问号占位符与 postgres 的位置占位符(`$1`)不会被误伤
- `bunproxy.RedactDSN(dsn)` 用于对连接串脱敏,`bunproxy.SanitizeSQL(sql)` 用于对 SQL 脱敏
- `Init` 的错误信息中的连接串已经过脱敏处理,不会带出明文密码
- 动态输入必须走 bun 的参数化 API,禁止使用 `fmt.Sprintf` 拼接 SQL

## 错误处理

| 错误 | 说明 |
| --- | --- |
| `ErrEmptyURL` | 连接串为空 |
| `ErrUnsupportedSchema` | 未支持的数据库类型(通常是漏了驱动导入) |
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

## 常见问题

**启动时报 `ErrUnsupportedSchema`?**
通常是二进制里没有编入该后端的驱动。错误信息会列出当前已注册的驱动,对照配置的 URL scheme 补上对应的 `import` 即可;也可以导入 `driver/all/v4` 一次性支持全部后端。

**`Init` 之前就调用查询会怎样?**
构建查询(`proxy.NewSelect()`、`proxy.NewRaw()`)没有问题,但**执行**会因为底层连接不存在而 panic。请确保在执行前完成初始化,或先用 `proxy.IsReady()` 判断。

**`Init` 与业务查询能并发吗?**
不推荐。`Init`/`Close` 属于生命周期操作,约定只在启动/退出阶段串行调用。

**需要用 `*bun.DB` 的完整能力?**
通过 `proxy.Client()` 或内嵌字段 `proxy.DB` 都可以拿到原始 `*bun.DB`,所有 bun 的方法都可用。

## 开发

仓库是**多模块**结构,以下命令需在对应目录执行:

```bash
# 核心模块
go build ./... && go vet ./... && go test -race ./...

# 各驱动与示例
for m in driver/postgres driver/mysql driver/sqlserver driver/sqlite driver/all example; do
  (cd "$m" && go build ./... && go vet ./... && go test -race ./...)
done
```

### 一键本地校验

仓库不依赖 GitHub Actions,校验全部在本地完成,只需 Go 工具链:

```bash
./scripts/check.sh
```

脚本会逐个模块执行 `gofmt` 检查、`go mod tidy` 差异校验、`go build`、`go vet`、`go test -race`,任一环节失败即返回非零状态。此外还有:

- **依赖瘦身校验**:核心模块与各驱动模块的依赖中不得出现其它后端(如 postgres 模块里不得有 `modernc.org/sqlite`)
- **真实数据库集成测试**:设置了 `SQLHELPER_TEST_*_URL` 时自动追加执行(用法见下节)
- **漏洞扫描**:本机 PATH 中存在 `govulncheck` 时自动执行(`go install golang.org/x/vuln/cmd/govulncheck@latest`);默认只报告不阻断,设置 `SQLHELPER_STRICT_VULN=1` 可让发现漏洞时校验失败。若命中的是 **Go 标准库**漏洞,升级本地 Go 工具链即可(仓库代码本身应保持 0 漏洞)

### 真实数据库集成测试

跨后端集成测试位于 `driver/all`,通过环境变量开关,未配置的数据库自动跳过:

| 环境变量 | 示例 |
| --- | --- |
| `SQLHELPER_TEST_SQLITE_URL` | `sqlite:///tmp/sqlhelper_it.db` |
| `SQLHELPER_TEST_MYSQL_URL` | `mysql://root:root@127.0.0.1:3306/sqlhelper?charset=utf8mb4` |
| `SQLHELPER_TEST_POSTGRES_URL` | `postgres://postgres:postgres@127.0.0.1:5432/sqlhelper?sslmode=disable` |
| `SQLHELPER_TEST_SQLSERVER_URL` | `sqlserver://sa:Your_password123@127.0.0.1:1433?database=master` |

本地用容器跑一遍:

```bash
docker run -d --name sqlhelper-it-mysql \
  -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=sqlhelper -p 13306:3306 mysql:8.4
docker run -d --name sqlhelper-it-postgres \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=sqlhelper -p 15432:5432 postgres:17

cd driver/all
SQLHELPER_TEST_SQLITE_URL='sqlite:///tmp/sqlhelper_it.db' \
SQLHELPER_TEST_MYSQL_URL='mysql://root:root@127.0.0.1:13306/sqlhelper?charset=utf8mb4' \
SQLHELPER_TEST_POSTGRES_URL='postgres://postgres:postgres@127.0.0.1:15432/sqlhelper?sslmode=disable' \
  go test -race -count=1 -v -run 'TestIntegration' ./...

docker rm -f sqlhelper-it-mysql sqlhelper-it-postgres
```

### 可运行示例

```bash
cd example
go run ./sqlite      # 基础用法:建表/写入/查询/健康检查
go run ./callbacks   # 回调注册、并行执行、错误聚合
go run ./querylog    # SQL 日志开关与脱敏
```

`./scripts/check.sh` 覆盖的检查项:

- **依赖瘦身校验**:核心与各驱动模块的依赖中不得出现其它后端
- **真实数据库集成**:设置了 `SQLHELPER_TEST_*_URL` 时额外执行 MySQL / PostgreSQL / SQLite 用例
- **漏洞扫描**:本机装有 `govulncheck` 时逐模块扫描

## 从 v3 迁移

驱动 import、命名变化与行为差异见 [MIGRATION_v3_to_v4.md](./MIGRATION_v3_to_v4.md)。
