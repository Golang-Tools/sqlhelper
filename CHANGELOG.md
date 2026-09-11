# v3.0.0

本次为现代化改造版本,遵循"尽量不改 API 形状,重点修 bug、补工程能力"的原则。模块路径变更为 `github.com/Golang-Tools/sqlhelper/v3`。

## bug修复

+ 修复选项静默失效的问题。`optparams` 从 v1.0.0 起 `GetOption` 变为拷贝语义,需要改用原地语义的 `Apply`,否则所有 `WithXxx` 选项都不会生效
+ 修复 `SQLite` 连接串已带 `file:` 前缀时被重复拼接的问题(例如 `sqlite://file::memory:?cache=shared` 旧实现会静默落盘成一个名为 `file::memory:` 的磁盘文件)
+ 修复 `MySQL` 连接串不带库名时被判为非法DSN的问题(`mysql://user:pwd@host:port` 以前会报 `invalid DSN: missing the slash separating the database name`)
+ 修复重复 `Init` 会先创建一个随后被丢弃的连接、并污染 `proxy.Opt` 配置的问题
+ 修复 `SetConnect`/`Init` 回调失败时未回滚代理状态的问题
+ 修复 `SetPool` 与 `NewDB` 在 `opts` 为 `nil` 时的空指针 panic
+ 修复 `Init` 在连接校验失败或 `SetConnect` 失败时泄漏连接池的问题,现在失败会主动 `Close`
+ 修复回调函数 panic 会导致进程崩溃的问题,现在会被捕获并转换为 `ErrCallbackPanic`
+ 修复并行回调"发射后不管"的问题,现在 `Init` 会等待全部回调结束后才返回
+ 修复回调返回的错误被静默吞掉的问题,现在会聚合返回,可用 `errors.Is/As` 判断
+ 修复 `Proxy` 在并发访问 `DB`/`Opt`/`callBacks` 时的数据竞争,补齐读写锁保护
+ 修复 SQLite 内存库(`:memory:`)多连接导致数据不可见的问题,现在限制为单连接
+ 修复 v2 中错误信息与注释残留的 `redis` 描述
+ 修复 `Err...Allready...`、`ErrUnSupport...` 等拼写问题(旧变量保留为别名)
+ 修复 README 中 Go 版本声明与实际 `go.mod` 不一致的问题

## 新增

+ 生命周期接口:`Proxy.Close`、`Proxy.PingContext`、`Proxy.Health`、`Proxy.Client`
+ 初始化连通性校验:默认开启,可通过 `WithDisablePingOnInit()` 关闭,超时可用 `WithPingTimeoutMS()` 调整
+ 上下文:`Proxy.NewCtxWithParent(ctx)`、`Proxy.DefaultQueryTimeout()`
+ 回调相关:`Proxy.Register`(规范命名)、`WithIgnoreCallbackError()`(兼容旧行为)
+ 连接池相关:`WithPoolConfig`、`WithConnMaxLifetime`、`WithConnMaxIdleTime`、`WithQueryTimeout`(Duration 版本)
+ SQL 日志:`WithQueryLog()`、`WithQueryLogArgs()`,以及自带脱敏能力的日志钩子(替换 `logrusbun`)
+ 脱敏工具:`RedactDSN`、`SanitizeSQL`、`RedactedPlaceholder`
+ 结构化错误:`CallbackError`、`CallbackPanicError`、`ErrEmptyURL`、`ErrNilDB`、`ErrNilCallback`、`ErrPingFailed`、`ErrCallbackPanic`
+ 测试与基准:覆盖初始化、生命周期、回调、并发、日志脱敏、ORM 链路等场景,并提供 `BenchmarkNewDBSQLiteMemory`
+ 真实数据库集成测试(`bunproxy/integration_test.go`),通过 `SQLHELPER_TEST_*_URL` 环境变量开启,覆盖 MySQL/PostgreSQL/SQLServer/SQLite 的建表、写入、查询、统计、事务、超时上下文与生命周期
+ 选项 `WithOptions`,支持整体覆盖配置(用于把布尔开关改回 `false`)
+ godoc 可执行示例(`bunproxy/example_test.go`)与可运行示例(`examples/sqlite`、`examples/callbacks`、`examples/querylog`)
+ CI:格式化检查、`go mod tidy` 校验、`go vet`、`go test -race`、覆盖率、benchmark、lint、govulncheck,以及基于 service 容器的真实数据库集成测试任务

## 变更

+ 模块路径由 `github.com/Golang-Tools/sqlhelper/v2` 变更为 `.../v3`
+ 依赖 `github.com/Golang-Tools/loggerhelper` 由 v2 升级到 v4(slog 化)
+ 依赖 `github.com/Golang-Tools/optparams` 由 v0.0.1 升级到 v1.0.0
+ 移除 `github.com/oiime/logrusbun` 依赖,改用内置脱敏日志钩子
+ SQL 日志默认关闭,开启后默认不记录参数值
+ `Init` 默认会校验连接可用性,并在失败时返回错误(旧行为可用 `WithDisablePingOnInit()` 恢复)
+ `Init`/`SetConnect` 在回调失败时会回滚连接状态并返回错误(旧行为可用 `WithIgnoreCallbackError()` 恢复)
+ `NewDB(url, nil)` 现在使用 `DefaultOpts` 中的连接池默认值
+ 不再在包 `init()` 中修改全局 logger 配置
+ 无回调注册时不再输出回调相关的 debug 日志
+ `Close` 不再把内嵌的连接指针置空,只关闭连接池并置位内部状态,消除关闭路径与业务查询之间的数据竞争
+ `Close` 会清空已注册的回调,避免关闭后重新 `Init` 时重复执行回调中的副作用操作
+ 重复 `Init` 现在会在创建连接前直接返回 `ErrProxyAlreadySetClient`
+ `Init` 成功后才写入本次选项,失败不会污染已有配置
+ 移除 v0 时代的废弃测试备份 `bunproxy/proxy_test.go_bak`,其 ORM 场景已移植为 v3 测试

## 兼容性

+ 保留 API:`New`、`Init`、`SetConnect`、`Regist`、`IsOk`、`NewCtx`、`NewDB`、`SetPool`、`Default`、`DefaultOpts`、`Options` 全部字段及所有 `WithXxx` 选项
+ 保留错误变量并使其与新命名等价:`ErrProxyAllreadySettedUniversalClient`、`ErrProxyNotYetSettedUniversalClient`、`ErrUnSupportSchema`
+ 详见 [MIGRATION_v2_to_v3.md](./MIGRATION_v2_to_v3.md)

# v2.0.0

+ 大规模接口变更,使用grpc风格的接口设计,同时将proxy改作子模块
+ 使用泛型语法,因此取消对低于go1.18版本的支持
+ 更新依赖版本

# v0.0.7

## 接口优化

+ 新增了接口`NewDB`用于创建`*bun.DB`的实例
+ 将Init中创建`*bun.DB`实例的逻辑移出到`NewDB`中
+ 增加参数`WithInstance(cli bun.IDB)`可以直接设置连接对象
+ 不再保存`Init`的参数

## 文档优化

+ 增加接口文档

# v0.0.6

## 新增接口

新增接口`SetQueryTimeout(timeout time.Duration)`用于为请求修改默认请求超时

# v0.0.5

## bug修复

修复mysql的url解析后密码有特殊字符时无法连接的问题

## 接口优化

参数`WithQueryTimeout`改为`WithQueryTimeoutMS`以明确单位为ms

## 新增接口

新增参数`WithLogger`用于为请求添加log用于debug

## 更新依赖

`github.com/uptrace/bun`更新至1.0.8

# v0.0.4

## bug修复

修复了mysql的url无法使用的问题

# v0.0.3

## bug修复

修正了gomod设置之前的版本都没法用

# v0.0.2

新增对`WithDiscardUnknownColumns`的支持

# v0.0.1

项目创
