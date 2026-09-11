package bunproxy_test

import (
	"context"
	"errors"
	"fmt"
	"os"

	log "github.com/Golang-Tools/loggerhelper/v4"
	"github.com/Golang-Tools/sqlhelper/v3/bunproxy"
	"github.com/uptrace/bun"
)

// memoryURL 示例统一使用的SQLite内存库
const memoryURL = "sqlite://:memory:"

func ExampleNew() {
	proxy := bunproxy.New()
	fmt.Println("新创建的代理是否可用:", proxy.IsOk())

	// Output:
	// 新创建的代理是否可用: false
}

func ExampleNewDB() {
	cli, err := bunproxy.NewDB(memoryURL, nil)
	if err != nil {
		fmt.Println("创建失败:", err)
		return
	}
	// NewDB 返回的连接由调用方负责关闭
	defer func() {
		_ = cli.Close()
	}()
	fmt.Println("连接是否可用:", cli.PingContext(context.Background()) == nil)

	// Output:
	// 连接是否可用: true
}

func ExampleProxy_Init() {
	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()

	// 初始化:建立连接 -> 校验连接可用性 -> 执行已注册的回调
	if err := proxy.Init(memoryURL, bunproxy.WithQueryTimeoutMS(3000)); err != nil {
		fmt.Println("初始化失败:", err)
		return
	}

	// NewCtx 会带上配置好的查询超时
	ctx, cancel := proxy.NewCtx()
	defer cancel()

	cli := proxy.Client()
	if _, err := cli.ExecContext(ctx, "CREATE TABLE t_user (id INTEGER, name TEXT)"); err != nil {
		fmt.Println("建表失败:", err)
		return
	}
	if _, err := cli.ExecContext(ctx, "INSERT INTO t_user (id, name) VALUES (?, ?)", 1, "alice"); err != nil {
		fmt.Println("写入失败:", err)
		return
	}
	var name string
	if err := cli.QueryRowContext(ctx, "SELECT name FROM t_user WHERE id = ?", 1).Scan(&name); err != nil {
		fmt.Println("查询失败:", err)
		return
	}
	fmt.Println("查询结果:", name)
	fmt.Println("代理是否可用:", proxy.IsOk())

	// Output:
	// 查询结果: alice
	// 代理是否可用: true
}

func ExampleProxy_Register() {
	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()

	// 回调只能在初始化前注册
	err := proxy.Register(func(cli *bun.DB) error {
		_, err := cli.ExecContext(context.Background(), "CREATE TABLE t_meta (key TEXT)")
		return err
	})
	if err != nil {
		fmt.Println("注册回调失败:", err)
		return
	}

	if err := proxy.Init(memoryURL); err != nil {
		fmt.Println("初始化失败:", err)
		return
	}

	// 初始化后再注册会失败
	err = proxy.Register(func(cli *bun.DB) error { return nil })
	fmt.Println("初始化后再注册是否报错:", errors.Is(err, bunproxy.ErrProxyAlreadySetClient))

	// Output:
	// 初始化后再注册是否报错: true
}

func ExampleProxy_NewCtxWithParent() {
	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()
	if err := proxy.Init(memoryURL, bunproxy.WithQueryTimeoutMS(3000)); err != nil {
		fmt.Println("初始化失败:", err)
		return
	}

	// 继承上游请求上下文,同时叠加配置好的查询超时
	ctx, cancel := proxy.NewCtxWithParent(context.Background())
	defer cancel()
	_, hasDeadline := ctx.Deadline()
	fmt.Println("上下文是否带上超时:", hasDeadline)

	// Output:
	// 上下文是否带上超时: true
}

func ExampleWithQueryLog() {
	// 把日志输出固定到stderr,避免示例输出里混入日志
	log.Set(log.WithOutput(os.Stderr), log.WithLevel("debug"))

	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()
	// SQL日志默认关闭;开启后只记录操作类型/耗时/脱敏后的SQL模板,不记录参数值
	if err := proxy.Init(memoryURL, bunproxy.WithQueryLog()); err != nil {
		fmt.Println("初始化失败:", err)
		return
	}
	ctx, cancel := proxy.NewCtx()
	defer cancel()
	cli := proxy.Client()
	if _, err := cli.ExecContext(ctx, "CREATE TABLE t_log (id INTEGER)"); err != nil {
		fmt.Println("建表失败:", err)
		return
	}
	_, err := cli.ExecContext(ctx, "INSERT INTO t_log (id) VALUES (?)", 1)
	fmt.Println("写入是否成功:", err == nil)

	// Output:
	// 写入是否成功: true
}

func ExampleRedactDSN() {
	fmt.Println(bunproxy.RedactDSN("postgres://user:secret@localhost:5432/db"))
	fmt.Println(bunproxy.RedactDSN("sqlite://:memory:"))

	// Output:
	// postgres://user:REDACTED@localhost:5432/db
	// sqlite://:memory:
}

func ExampleSanitizeSQL() {
	fmt.Println(bunproxy.SanitizeSQL("SELECT * FROM users WHERE name = 'alice' AND age > 18"))
	fmt.Println(bunproxy.SanitizeSQL("SELECT * FROM users WHERE id = $1"))

	// Output:
	// SELECT * FROM users WHERE name = '?' AND age > ?
	// SELECT * FROM users WHERE id = $1
}
