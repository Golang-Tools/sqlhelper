# v2 → v3 迁移指南

v3 的模块路径是 `github.com/Golang-Tools/sqlhelper/v3`,与 v2 不同,因此两者可以在同一个项目中并存,便于逐个模块灰度迁移。

改造原则:**尽量不改 API 形状,只修 bug 与补能力**。因此绝大多数使用方只需要修改 import 路径。

## 第一步:改 import 路径

```go
// 旧
import "github.com/Golang-Tools/sqlhelper/v2/bunproxy"

// 新
import "github.com/Golang-Tools/sqlhelper/v3/bunproxy"
```

```bash
go get github.com/Golang-Tools/sqlhelper/v3
```

根包 `sqlhelper` 依然是"仅承载模块声明"的空包,真正的入口是子包 `bunproxy`。

## 第二步:确认行为变化

下面 4 项是**默认行为变化**,如果依赖旧行为,需要显式加选项恢复。

### 1. `Init` 默认会校验连接可用性

v2 只做 `sql.Open`,是惰性连接,连接串写错时 `Init` 依然成功,错误延迟到第一次查询才暴露。v3 在 `Init` 中增加了一次 `PingContext` 校验。

```go
// v3 默认行为:连接不可用 -> Init 返回 ErrPingFailed
if err := proxy.Init(url); err != nil {
	// errors.Is(err, bunproxy.ErrPingFailed)
}

// 需要旧行为时
proxy.Init(url, bunproxy.WithDisablePingOnInit())

// 调整校验超时(默认 5s)
proxy.Init(url, bunproxy.WithPingTimeoutMS(1000))
```

好处是失败时会主动关闭刚创建的连接池,不再泄漏连接。

### 2. 回调错误会影响 `Init` 的返回值

v2 中回调返回的错误只打日志,`Init` 仍然返回 `nil`。v3 会聚合回调错误并返回。

```go
// 需要旧行为时
proxy.Init(url, bunproxy.WithIgnoreCallbackError())
```

同时,回调失败时连接会被回滚释放,`proxy.IsOk()` 为 `false`,可以直接重试。

### 3. SQL 日志默认关闭

v2 无条件挂载 `logrusbun` 的查询钩子,SQL 与参数会进入日志。v3 默认不开启,并提供脱敏的内置钩子。

```go
proxy.Init(url, bunproxy.WithQueryLog())      // 记录 操作/耗时/脱敏SQL
proxy.Init(url, bunproxy.WithQueryLogArgs())  // 额外记录参数,谨慎使用
```

### 4. SQLite 内存库改为单连接

`sqlite://:memory:` 在 v2 中未限制 `MaxOpenConns`,`database/sql` 建立新连接时会得到一个空的数据库副本,容易出现"表不存在"的随机失败。v3 会自动限制为 `MaxOpenConns(1)`。

如果需要多连接共享同一份内存数据,请改用文件库或带 `cache=shared` 的连接串。

## 第三步:可选地使用新增能力

以下都是新增接口,不改变既有行为:

```go
proxy.Close()                       // 关闭连接池,释放资源
proxy.PingContext(ctx)              // 连通性探测
proxy.Health(ctx)                   // 带默认查询超时的连通性探测
proxy.Client()                      // 安全地获取 *bun.DB(未初始化时返回 nil)
proxy.NewCtxWithParent(reqCtx)      // 继承请求上下文的超时上下文
proxy.DefaultQueryTimeout()         // 读取生效的默认查询超时
proxy.Register(cb)                  // Regist 的规范命名别名
```

新增选项:

```go
bunproxy.WithQueryTimeout(d)                        // Duration 版本
bunproxy.WithConnMaxLifetime(d) / WithConnMaxIdleTime(d)
bunproxy.WithPoolConfig(maxOpen, maxIdle, lifetime, idleTime)
bunproxy.WithQueryLog() / WithQueryLogArgs()
bunproxy.WithPingTimeoutMS(ms) / WithDisablePingOnInit()
bunproxy.WithIgnoreCallbackError()
```

新增脱敏工具(可用于自己的日志):

```go
bunproxy.RedactDSN("postgres://user:pwd@host:5432/db")  // pwd -> REDACTED
bunproxy.SanitizeSQL("SELECT * FROM t WHERE a = 'x'")   // 'x' -> '?'
```

## 错误处理的兼容性

v2 的 4 个错误变量全部保留,并且与新命名**指向同一个错误值**,因此 `errors.Is` 对新旧名字都成立:

| v2(保留) | v3(推荐) |
| --- | --- |
| `ErrProxyAllreadySettedUniversalClient` | `ErrProxyAlreadySetClient` |
| `ErrProxyNotYetSettedUniversalClient` | `ErrProxyNotSetClient` |
| `ErrUnSupportSchema` | `ErrUnsupportedSchema` |
| `ErrUnknownClientType` | `ErrUnknownClientType`(语义不变) |

## 依赖变化

| 依赖 | v2 | v3 | 说明 |
| --- | --- | --- | --- |
| `github.com/Golang-Tools/loggerhelper` | v2 | v4 | 升级到 slog 版本 |
| `github.com/Golang-Tools/optparams` | v0.0.1 | v1.0.0 | `GetOption` 变为拷贝语义,包内已改用 `Apply` |
| `github.com/oiime/logrusbun` | v0.1.2 | 已移除 | 改用内置脱敏日志钩子 |

> 自定义封装如果直接使用了 `optparams.GetOption` 来做原地配置,升级到 v1.0.0 后必须改为 `optparams.Apply`,否则选项会静默失效。

### 关于 `bunproxy.Logger`

`Logger` 变量本身依然导出且不为 `nil`,但底层实现随 loggerhelper 由 logrus 换成了 `slog`,因此:

- `bunproxy.Logger.GetLogger()` 的返回类型由 `*logrus.Logger` 变为 `*slog.Logger`
- 包不再在 `init()` 中修改全局 logger 配置;如果原先依赖 `module=bun-proxy` 这个扩展字段,请自行在初始化时配置

## 并发语义

- `IsOk`、`Client`、`PingContext`、`Health`、`Regist`、`Close`、`DefaultQueryTimeout` 与回调执行都受内部读写锁保护
- `Init` / `Close` 属于生命周期操作,应在启动/退出阶段串行调用
- `Proxy` 内嵌的 `*bun.DB` 字段依然可以直接访问(v2 兼容),但并发 `Init`/`Close` 期间直接访问该字段不在保护范围内

## 需要 Go 1.25

v3 要求 Go 1.25.0 及以上,这是依赖链决定的:`bun/driver/sqliteshim` → `modernc.org/sqlite` 与 `golang.org/x/*` 自身声明的最低 Go 版本为 1.25。如果需要支持更低版本,需要同步降级 sqliteshim / modernc 系列依赖。

## 编译期需要留意的地方

v3 保留了 v2 的全部导出符号,但有三处会让**使用方代码**在编译或 `go vet` 阶段出现差异:

1. **`bunproxy.Logger` 的底层类型换了模块**

   v2 是 `github.com/Golang-Tools/loggerhelper/v2` 的 `*log.Log`,v3 是 `.../loggerhelper/v4` 的 `*log.Log`。如果使用方在变量声明、函数签名或结构体字段里显式写了 v2 的类型,需要把 import 路径改成 v4;`Logger.GetLogger()` 的返回类型也由 `*logrus.Logger` 变为 `*slog.Logger`。

2. **不要用位置初始化 `Options`**

   v3 的 `Options` 新增了 `QueryLog`、`QueryLogArgs`、`DisablePingOnInit`、`PingTimeout`、`IgnoreCallbackError` 字段。使用 `Options{true, 0, 10}` 这种不带字段名的写法会编译失败,请改为具名字段:

   ```go
   // 不兼容写法
   opts := bunproxy.Options{false, time.Second, 10}

   // 推荐写法
   opts := bunproxy.Options{QueryTimeout: time.Second, MaxOpenConns: 10}
   ```

3. **`Proxy` 现在包含互斥锁,按值传递会触发 `copylocks`**

   v3 的 `Proxy` 内部新增了 `sync.RWMutex`,因此请始终使用 `*Proxy`(与 v2 一样通过 `bunproxy.New()` 获取)。按值拷贝会让 `go vet` 报 `copylocks`,并且拷贝出来的副本无法正确共享状态。

## 自检清单

- [ ] import 路径已改为 `/v3`
- [ ] 确认 `Init` 的连通性校验符合预期(必要时加 `WithDisablePingOnInit()`)
- [ ] 确认回调错误返回符合预期(必要时加 `WithIgnoreCallbackError()`)
- [ ] 确认是否有依赖 SQL 日志的逻辑(必要时加 `WithQueryLog()`)
- [ ] SQLite 内存库确认是否受单连接限制影响
- [ ] 使用到 `Proxy` 跨 goroutine 的代码已按并发语义检查
- [ ] 显式引用 `loggerhelper` 类型的地方已切到 v4
- [ ] `Options` 字面量均已改为具名字段
- [ ] 所有 `Proxy` 均以指针方式传递
