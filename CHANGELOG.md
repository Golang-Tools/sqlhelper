# v4.0.0

本次为**破坏性**版本:数据库驱动拆分为按需引入的独立子模块,核心模块不再依赖任何具体数据库实现;代理的用法与设计保持不变(`Proxy` 常驻、`Init` 前即可书写业务逻辑、查询继续走 `proxy.NewSelect()/ExecContext()/RunInTx()` 直调)。

## 破坏性变更

+ 模块路径升级为 `github.com/Golang-Tools/sqlhelper/v4`
+ 驱动独立成子模块,必须空导入才能启用对应 URL scheme:
  `driver/postgres/v4`、`driver/mysql/v4`、`driver/sqlserver/v4`、`driver/sqlite/v4`,或一次性引入 `driver/all/v4`
+ 未导入驱动时 `Init` 返回 `ErrUnsupportedSchema`,错误信息中会列出当前已注册的驱动并提示需要导入的包
+ 移除历史遗留符号 `ErrUnknownClientType`(语义与用途均不明确)
+ `Regist` 与 `IsOk` 标记为 Deprecated(仍可用),推荐 `Register` 与 `IsReady`

## 新增

+ `Driver` 接口与驱动注册表:`RegisterDriver`、`FindDriver`、`RegisteredSchemes`,支持第三方扩展
+ `Proxy.InitContext(ctx, ...)`:可取消/带整体超时的初始化,连通性校验与重试等待都受 ctx 约束
+ `Proxy.IsReady()`:语义更明确的可用性判断
+ `Options.ApplyPool(*sql.DB)`:导出连接池配置逻辑,供驱动实现复用
+ 官方驱动子模块 `driver/{postgres,mysql,sqlserver,sqlite}` 与聚合包 `driver/all`
+ 独立示例模块 `example/`,演示驱动导入、`RegisteredSchemes` 启动自检与多后端切换

## 变更

+ 核心模块依赖大幅精简:移除 `go-sql-driver/mysql`、`go-mssqldb`、`pgdriver`、`sqliteshim`、`modernc.org/sqlite` 等,仅保留 `bun`、`loggerhelper/v4`、`optparams`(`sqlitedialect` 仅用于测试)
+ 核心模块测试改用内置 stub driver(`memory://`),不再依赖真实数据库;真实数据库用例迁移到 `driver/sqlite` 与 `driver/all`
+ 驱动子模块使用独立版本与 tag(`driver/<name>/vX.Y.Z`),并保留 `replace` 便于本地多模块联调
+ 移除对 GitHub Actions 的依赖(`.github/`、`.golangci.yml`),校验改为仓库内置的 `scripts/check.sh`:逐模块执行 gofmt 校验、`go mod tidy` 差异校验、`go build`、`go vet`、`go test -race`,并包含依赖瘦身校验、可选的真实数据库集成测试与可选的 `govulncheck` 漏洞扫描

## 迁移

见 [MIGRATION_v3_to_v4.md](./MIGRATION_v3_to_v4.md)

# v3.1.0

本次为 v3 系列的功能增强版本,**不包含破坏性变更**,全部为新增能力与行为修正。

## 增强

+ `SanitizeSQL` 现在除字符串字面量外还会遮蔽数字字面量(含小数、科学计数法与十六进制),避免 bun 查询构造器内联的数值进入日志;同时保证带数字的标识符(`col1`、`utf8mb4`)、问号占位符与 postgres 位置占位符(`$1`)不被误伤
+ 新增 `CallbackContext` 类型与 `RegisterContext` 方法,支持注册带上下文的回调;与 `Regist` 注册的回调按注册顺序统一执行
+ 新增 `CallbackTimeout` 与选项 `WithCallbackTimeout`/`WithCallbackTimeoutMS`,为带上下文的回调提供超时保护(默认不限制)
+ 新增连接建立重试:`ConnectRetryAttempts`/`ConnectRetryInterval` 与选项 `WithConnectRetry`,只对连通性校验失败重试,间隔按指数退避增长(默认不重试)
+ 新增选项 `WithDefaultOpts`,可一键套用 `DefaultOpts` 中的推荐默认值,与 `NewDB(url, nil)` 的行为对齐
+ 新增 `Proxy.PingTimeout()` 方法,用于读取实际生效的连通性探测超时

## 修正

+ `Health` 的超时来源由 `QueryTimeout` 改为 `PingTimeout`,与 `Init` 的连通性校验保持一致;此前 `WithPingTimeoutMS` 对 `Health` 无效,而 `QueryTimeout` 为 0 时 `Health` 完全没有超时保护
+ 修复 `Init` 首次执行时读不到本次入参中回调相关配置的问题(`CallbackTimeout` 等会在连接建立成功后才写入 `proxy.Opt`)

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
+ 移除 `docs/` 下 1996 个 v0 时代生成的静态 godoc 文件(约 146MB),替换为指向 pkg.go.dev 的落地页,保留 `docs/.nojekyll` 以兼容 GitHub Pages

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
