# v3 → v4 迁移指南

v4 是破坏性版本,但**破坏面集中在"驱动引入方式"与少数命名**,代理的用法与设计完全保持:

- `Proxy`(尤其是 `bunproxy.Default`)依然常驻,`Init` 之前就可以书写业务逻辑
- 查询依然走 `proxy.NewSelect()`、`proxy.ExecContext(...)`、`proxy.RunInTx(...)` 这类直调
- `Init` 只在启动阶段、`Close` 只在退出阶段的约定不变

## 第一步:升级模块路径

```bash
go get github.com/Golang-Tools/sqlhelper/v4
```

```go
// 旧
import "github.com/Golang-Tools/sqlhelper/v3/bunproxy"

// 新
import "github.com/Golang-Tools/sqlhelper/v4/bunproxy"
```

## 第二步:引入需要的驱动(必须做)

v4 的核心模块不再内置任何数据库实现,需要显式空导入:

| 后端 | 需要新增的 import |
| --- | --- |
| PostgreSQL | `_ "github.com/Golang-Tools/sqlhelper/driver/postgres/v4"` |
| MySQL | `_ "github.com/Golang-Tools/sqlhelper/driver/mysql/v4"` |
| SQL Server | `_ "github.com/Golang-Tools/sqlhelper/driver/sqlserver/v4"` |
| SQLite | `_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"` |
| 全部后端 | `_ "github.com/Golang-Tools/sqlhelper/driver/all/v4"` |

```bash
go get github.com/Golang-Tools/sqlhelper/driver/postgres/v4
```

如果业务代码需要"同一个二进制支持多个后端、停机改配置切换",直接引入 `driver/all/v4`,或在编译期把可能用到的后端都引入:

```go
import (
	_ "github.com/Golang-Tools/sqlhelper/driver/mysql/v4"
	_ "github.com/Golang-Tools/sqlhelper/driver/postgres/v4"
	_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
)
```

漏导驱动时,`Init` 会返回 `ErrUnsupportedSchema`,错误信息形如:

```
未支持的数据库管理服务类型: "mysql" (当前已注册的驱动: [postgres sqlite],是否漏了对应 driver 子模块的导入?)
```

建议在启动日志中打印 `bunproxy.RegisteredSchemes()`,便于运维核对"配置与二进制是否匹配"。

## 第三步:命名调整(旧名仍可用)

| v3 | v4 推荐 | 说明 |
| --- | --- | --- |
| `proxy.Regist(cb)` | `proxy.Register(cb)` | `Regist` 保留为 Deprecated 别名 |
| `proxy.IsOk()` | `proxy.IsReady()` | `IsOk` 保留为 Deprecated 别名 |
| `ErrProxyAllreadySettedUniversalClient` | `ErrProxyAlreadySetClient` | v3 起即为等值别名,`errors.Is` 对新旧都成立 |
| `ErrProxyNotYetSettedUniversalClient` | `ErrProxyNotSetClient` | 同上 |
| `ErrUnSupportSchema` | `ErrUnsupportedSchema` | 同上 |
| `ErrUnknownClientType` | — | **已移除**(无实际用途) |

## 第四步:可选地使用新能力

```go
// 可取消/带整体超时的初始化(启动脚本常需要)
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
err := proxy.InitContext(ctx, url, bunproxy.WithConnectRetry(3, 500*time.Millisecond))

// 语义更明确的可用性判断
if proxy.IsReady() { ... }

// 启动自检:打印当前二进制支持的后端
log.Println("supported backends:", bunproxy.RegisteredSchemes())
```

自定义驱动(第三方数据库)可以自行实现并注册:

```go
type myDriver struct{}

func (myDriver) Scheme() string { return "mydb" }
func (myDriver) Dialect() schema.Dialect { return myDialect{} }
func (myDriver) NewPool(rawURL string, opts *bunproxy.Options) (*sql.DB, error) {
	sqldb, err := sql.Open("mydb", rawURL)
	if err != nil {
		return nil, err
	}
	opts.ApplyPool(sqldb) // 复用统一的连接池配置语义
	return sqldb, nil
}

func init() { bunproxy.RegisterDriver(myDriver{}) }
```

## 行为差异(与 v3 相比)

除驱动引入方式外,v4 的运行时行为与 v3.1 一致:

- 默认不打印 SQL 日志(`WithQueryLog()` 开启)
- `Init` 默认校验连通性(`WithDisablePingOnInit()` 关闭)
- 回调错误会让 `Init` 失败(`WithIgnoreCallbackError()` 恢复旧行为)
- `Close` 会清空回调,`Close` 后可重新 `Init`

## 依赖变化(顺带的好处)

v4 的核心模块只依赖 `bun`、`loggerhelper/v4`、`optparams`;每个驱动子模块自带所需的数据库依赖。只使用单一后端的项目不再被其它后端的依赖牵连:

```bash
# 只引入核心 + postgres 时,依赖列表中不会出现 modernc、go-mssqldb、go-sql-driver 等
go list -m all
```

## 自检清单

- [ ] import 路径已改为 `/v4`
- [ ] 已引入业务用到的后端驱动(或用 `driver/all/v4`)
- [ ] 启动日志中确认 `RegisteredSchemes()` 与配置匹配
- [ ] `Regist`/`IsOk` 已替换为 `Register`/`IsReady`
- [ ] 若引用了 `ErrUnknownClientType`,改用 `ErrUnsupportedSchema`
- [ ] 多模块构建脚本/CI 已按新的模块布局调整(核心、各 driver、example 各自成模块)
