// Command callbacks 演示 bunproxy 的回调注册、并行执行与错误处理。
//
// 运行方式(在 example 目录下):
//
//	go run ./callbacks
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	"github.com/uptrace/bun"
)

func main() {
	demoSuccess()
	demoCallbackError()
}

// demoSuccess 演示普通回调与带上下文回调的顺序执行
func demoSuccess() {
	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()

	// 回调只能在初始化前注册;初始化后再注册会返回 ErrProxyAlreadySetClient
	_ = proxy.Register(func(cli *bun.DB) error {
		_, err := cli.ExecContext(context.Background(),
			"CREATE TABLE IF NOT EXISTS app_meta (key TEXT PRIMARY KEY, value TEXT)")
		return err
	})
	// 需要超时保护的操作可以用带上下文的回调
	_ = proxy.RegisterContext(func(ctx context.Context, cli *bun.DB) error {
		_, err := cli.ExecContext(ctx,
			"INSERT INTO app_meta (key, value) VALUES (?, ?)", "version", "v4")
		return err
	})

	// WithParallelCallback 让回调并行执行,但 Init 会等待全部回调结束后才返回
	err := proxy.Init("sqlite://:memory:",
		bunproxy.WithParallelCallback(),
		bunproxy.WithCallbackTimeoutMS(5000),
	)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	var value string
	ctx, cancel := proxy.NewCtx()
	defer cancel()
	if err := proxy.Client().QueryRowContext(ctx,
		"SELECT value FROM app_meta WHERE key = ?", "version").Scan(&value); err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	fmt.Printf("回调写入的值: %s\n", value)
}

// demoCallbackError 演示回调失败时的错误语义
func demoCallbackError() {
	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()

	sentinel := errors.New("演示用的回调错误")
	_ = proxy.Register(func(cli *bun.DB) error { return sentinel })

	err := proxy.Init("sqlite://:memory:")
	if err == nil {
		log.Fatal("回调失败时 Init 应该返回错误")
	}

	// 回调的原始错误会保留在错误链上
	fmt.Printf("errors.Is(err, sentinel) = %v\n", errors.Is(err, sentinel))

	// 通过 errors.As 可以拿到失败的回调下标与原始错误
	var cbErr *bunproxy.CallbackError
	if errors.As(err, &cbErr) {
		fmt.Printf("第 %d 个回调失败: %v\n", cbErr.Index, cbErr.Err)
	}

	// 回调失败时连接会被回滚释放,可以直接重试 Init
	fmt.Printf("代理当前是否可用: %v\n", proxy.IsOk())

	// 如果确实需要旧版本"回调错误只记日志"的行为:
	if err := proxy.Init("sqlite://:memory:", bunproxy.WithIgnoreCallbackError()); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}
	fmt.Printf("忽略回调错误后是否可用: %v\n", proxy.IsOk())
}
