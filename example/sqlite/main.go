// Command sqlite 演示 bunproxy 配合 SQLite 的基础用法。
//
// 运行方式(在 example 目录下):
//
//	go run ./sqlite
//
// 运行后会在当前目录生成 example.db 文件。
package main

import (
	"context"
	"fmt"
	"log"

	//空导入即启用 sqlite:// 连接串,换成其它后端只需改这一行
	_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	"github.com/uptrace/bun"
)

// User 演示用的数据模型
type User struct {
	bun.BaseModel `bun:"users,alias:u"`

	ID   int64  `bun:"id,pk,autoincrement"`
	Name string `bun:"name,notnull"`
	Age  int    `bun:"age,notnull"`
}

func main() {
	proxy := bunproxy.New()
	defer func() {
		// Close 会释放连接池;关闭后代理回到未初始化状态,可以重新Init
		if err := proxy.Close(); err != nil {
			log.Printf("关闭代理失败: %v", err)
		}
	}()

	// 启动日志里打印当前二进制支持哪些后端,便于排查"配置与二进制不匹配"
	fmt.Println("已注册的后端:", bunproxy.RegisteredSchemes())

	// Init 会:建立连接 -> 校验连接可用性 -> 执行已注册的回调
	// 校验失败时会主动关闭刚创建的连接池,并返回 ErrPingFailed
	err := proxy.Init("sqlite://example.db",
		bunproxy.WithQueryTimeoutMS(3000),
		bunproxy.WithMaxOpenConns(4),
		bunproxy.WithMaxIdleConns(4),
		bunproxy.WithConnMaxLifetimeMS(60000),
	)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// NewCtx 会带上上面配置好的 QueryTimeout
	ctx, cancel := proxy.NewCtx()
	defer cancel()

	cli := proxy.Client()
	if err := prepare(ctx, cli); err != nil {
		log.Fatalf("准备数据失败: %v", err)
	}

	users, err := listUsers(ctx, cli)
	if err != nil {
		log.Fatalf("查询失败: %v", err)
	}
	for _, u := range users {
		fmt.Printf("user: id=%d name=%s age=%d\n", u.ID, u.Name, u.Age)
	}

	// 健康检查:带默认连通性探测超时
	if err := proxy.Health(context.Background()); err != nil {
		log.Fatalf("健康检查失败: %v", err)
	}
	fmt.Println("健康检查通过")
}

// prepare 建表并写入演示数据
func prepare(ctx context.Context, cli *bun.DB) error {
	if _, err := cli.NewCreateTable().Model((*User)(nil)).IfNotExists().Exec(ctx); err != nil {
		return err
	}
	users := []User{{Name: "alice", Age: 30}, {Name: "bob", Age: 24}}
	if _, err := cli.NewInsert().Model(&users).Exec(ctx); err != nil {
		return err
	}
	return nil
}

// listUsers 按年龄倒序查询用户
func listUsers(ctx context.Context, cli *bun.DB) ([]User, error) {
	users := make([]User, 0, 2)
	if err := cli.NewSelect().Model(&users).OrderExpr("age DESC").Scan(ctx); err != nil {
		return nil, err
	}
	return users, nil
}
