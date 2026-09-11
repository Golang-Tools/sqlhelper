// Command querylog 演示 bunproxy 的 SQL 日志开关与脱敏能力。
//
// 运行方式(在 example 目录下):
//
//	go run ./querylog
package main

import (
	"fmt"
	"log"

	loghelper "github.com/Golang-Tools/loggerhelper/v4"
	_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
)

func main() {
	// SQL 日志默认是关闭的;开启后只记录 操作类型/耗时/脱敏后的SQL
	// 这里把日志级别调到 debug 才能看到查询日志
	loghelper.Set(loghelper.WithLevel("debug"))

	proxy := bunproxy.New()
	defer func() {
		_ = proxy.Close()
	}()

	err := proxy.Init("sqlite://:memory:",
		bunproxy.WithQueryLog(), // 只记录脱敏后的SQL模板
		bunproxy.WithQueryTimeoutMS(3000),
	)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	ctx, cancel := proxy.NewCtx()
	defer cancel()

	if _, err := proxy.Client().ExecContext(ctx,
		"CREATE TABLE t_secret (id INTEGER, token TEXT)"); err != nil {
		log.Fatalf("建表失败: %v", err)
	}
	// 参数值默认不会被写入日志;如果确实需要,可以改用 WithQueryLogArgs()
	// 但那会把参数值(可能包含隐私数据)落到日志里,请谨慎评估
	if _, err := proxy.Client().ExecContext(ctx,
		"INSERT INTO t_secret (id, token) VALUES (?, ?)", 1, "sensitive-token"); err != nil {
		log.Fatalf("插入失败: %v", err)
	}

	// 连接串脱敏:拼接错误信息或打印配置时可以用它避免泄漏密码
	fmt.Println(bunproxy.RedactDSN("postgres://user:secret@localhost:5432/db?sslmode=disable"))
	// SQL 脱敏:把字符串与数字字面量替换为占位符
	fmt.Println(bunproxy.SanitizeSQL("SELECT * FROM t_secret WHERE token = 'sensitive-token' AND id = 1"))
}
