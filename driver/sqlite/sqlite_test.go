package sqlite

import (
	"context"
	"os"
	"testing"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	"github.com/uptrace/bun"
)

const memoryURL = "sqlite://:memory:"

// testUser 测试用数据模型
type testUser struct {
	bun.BaseModel `bun:"it_users"`

	ID   int64  `bun:"id,pk"`
	Name string `bun:"name,notnull"`
	Age  int    `bun:"age,notnull"`
}

func TestDriverRegistered(t *testing.T) {
	d, ok := bunproxy.FindDriver(Scheme)
	if !ok || d == nil {
		t.Fatalf("%s 驱动应已注册", Scheme)
	}
	if d.Scheme() != Scheme {
		t.Fatalf("scheme不符, 实际: %s", d.Scheme())
	}
	if d.Dialect() == nil {
		t.Fatal("方言不应为nil")
	}
}

// TestNewDBWithNilOptions 验证 nil 配置下也能正常建连
func TestNewDBWithNilOptions(t *testing.T) {
	cli, err := bunproxy.NewDB(memoryURL, nil)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}
	defer func() {
		_ = cli.Close()
	}()
	if err := cli.PingContext(context.Background()); err != nil {
		t.Fatalf("nil配置下创建的连接应可用: %v", err)
	}
}

// TestFileDSNNotDoubled 验证已带 file: 前缀的 DSN 不会被重复加前缀
// 旧实现会把 file::memory:?cache=shared 拼成 file:file::memory:?...,导致静默落盘成同名文件
func TestFileDSNNotDoubled(t *testing.T) {
	ctx := context.Background()
	dsn := "sqlite://file::memory:?cache=shared"
	cli, err := bunproxy.NewDB(dsn, nil)
	if err != nil {
		t.Fatalf("构造连接失败: %v", err)
	}
	if err := cli.PingContext(ctx); err != nil {
		t.Fatalf("内存库应该可用: %v", err)
	}
	if _, err := cli.ExecContext(ctx, "CREATE TABLE t_shared (id INTEGER)"); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if err := cli.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	//确认没有把DSN当成文件名落到磁盘上
	if _, err := os.Stat("file::memory:"); err == nil {
		_ = os.Remove("file::memory:")
		t.Fatal("内存库不应在磁盘上生成名为 file::memory: 的文件")
	}
}

// TestMemoryDBSingleConn 验证私有内存库被限制为单连接
func TestMemoryDBSingleConn(t *testing.T) {
	cli, err := bunproxy.NewDB(memoryURL, nil)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}
	defer func() {
		_ = cli.Close()
	}()
	sqldb := cli.DB
	if got := sqldb.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("私有内存库应限制为单连接, 实际: %d", got)
	}
}

// TestProxyORMRoundTrip 覆盖建表/插入/查询/统计链路(原核心模块用例迁移至此)
func TestProxyORMRoundTrip(t *testing.T) {
	ctx := context.Background()
	proxy := bunproxy.New()
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	cli := proxy.Client()
	if _, err := cli.NewCreateTable().Model((*testUser)(nil)).IfNotExists().Exec(ctx); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	users := []testUser{{ID: 1, Name: "a", Age: 11}, {ID: 2, Name: "b", Age: 11}}
	if _, err := cli.NewInsert().Model(&users).Exec(ctx); err != nil {
		t.Fatalf("插入失败: %v", err)
	}
	single := testUser{}
	if err := cli.NewSelect().Model(&single).Where("id = ?", int64(1)).Scan(ctx); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if single.Age != 11 || single.Name != "a" {
		t.Fatalf("查询结果不符: %+v", single)
	}
	count, err := cli.NewSelect().Model((*testUser)(nil)).Where("age = ?", 11).Count(ctx)
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("统计结果不符, 期望2, 实际: %d", count)
	}
}

// TestDiscardUnknownColumns 验证开启后未知列不会报错
func TestDiscardUnknownColumns(t *testing.T) {
	ctx := context.Background()
	run := func(t *testing.T, discard bool) error {
		t.Helper()
		cli, err := bunproxy.NewDB(memoryURL, &bunproxy.Options{DiscardUnknownColumns: discard})
		if err != nil {
			t.Fatalf("创建测试数据库失败: %v", err)
		}
		defer func() {
			_ = cli.Close()
		}()
		if _, err := cli.ExecContext(ctx, "CREATE TABLE t_discard (id INTEGER)"); err != nil {
			t.Fatalf("建表失败: %v", err)
		}
		if _, err := cli.ExecContext(ctx, "INSERT INTO t_discard (id) VALUES (1)"); err != nil {
			t.Fatalf("插入失败: %v", err)
		}
		var m struct {
			Id int64 `bun:"id"`
		}
		return cli.NewRaw("SELECT id, 'extra_value' AS extra_col FROM t_discard").Scan(ctx, &m)
	}
	if err := run(t, false); err == nil {
		t.Skip("当前bun版本在未开启DiscardUnknownColumns时也不会因未知列报错,该选项无可见差异")
	}
	if err := run(t, true); err != nil {
		t.Fatalf("开启DiscardUnknownColumns后不应因未知列报错: %v", err)
	}
}
