package all

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	"github.com/uptrace/bun"
)

// 本文件是跨后端集成测试:未配置对应环境变量时整组自动跳过。
//
//	SQLHELPER_TEST_SQLITE_URL=sqlite:///tmp/sqlhelper_it.db
//	SQLHELPER_TEST_MYSQL_URL=mysql://root:root@127.0.0.1:3306/sqlhelper?charset=utf8mb4
//	SQLHELPER_TEST_POSTGRES_URL=postgres://postgres:postgres@127.0.0.1:5432/sqlhelper?sslmode=disable
//	SQLHELPER_TEST_SQLSERVER_URL=sqlserver://sa:Your_password123@127.0.0.1:1433?database=master
type integrationUser struct {
	bun.BaseModel `bun:"it_users"`

	ID   int64  `bun:"id,pk"`
	Name string `bun:"name,notnull"`
	Age  int    `bun:"age,notnull"`
}

func TestRegisteredSchemes(t *testing.T) {
	schemes := bunproxy.RegisteredSchemes()
	for _, want := range []string{"mysql", "postgres", "sqlite", "sqlserver"} {
		if !contains(schemes, want) {
			t.Fatalf("导入 driver/all 后应注册 %s, 当前: %v", want, schemes)
		}
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestIntegrationRealDatabases(t *testing.T) {
	cases := []struct {
		name string
		env  string
	}{
		{"sqlite", "SQLHELPER_TEST_SQLITE_URL"},
		{"mysql", "SQLHELPER_TEST_MYSQL_URL"},
		{"postgres", "SQLHELPER_TEST_POSTGRES_URL"},
		{"sqlserver", "SQLHELPER_TEST_SQLSERVER_URL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := os.Getenv(c.env)
			if raw == "" {
				t.Skipf("未配置 %s, 跳过真实数据库集成测试", c.env)
			}
			runIntegration(t, raw)
		})
	}
}

func runIntegration(t *testing.T, raw string) {
	t.Helper()
	ctx := context.Background()

	//容器化环境里数据库可能比测试启动得晚,这里做一次有上限的等待
	waitForDatabase(t, raw, 60*time.Second)

	//用独立连接负责清理测试表,不受代理生命周期影响
	defer func() {
		clean, err := bunproxy.NewDB(raw, nil)
		if err != nil {
			return
		}
		_, _ = clean.NewDropTable().Model((*integrationUser)(nil)).IfExists().Exec(context.Background())
		_ = clean.Close()
	}()

	proxy := bunproxy.New()
	callbackCount := 0
	if err := proxy.Register(func(cli *bun.DB) error {
		callbackCount++
		var one int
		return cli.QueryRowContext(ctx, "SELECT 1").Scan(&one)
	}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}

	if err := proxy.Init(raw,
		bunproxy.WithQueryTimeoutMS(15000),
		bunproxy.WithMaxOpenConns(4),
		bunproxy.WithMaxIdleConns(4),
		bunproxy.WithQueryLog(),
	); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}

	if callbackCount != 1 {
		t.Fatalf("初始化时应执行1次回调, 实际: %d", callbackCount)
	}
	if !proxy.IsOk() {
		t.Fatal("初始化后代理应处于可用状态")
	}
	if err := proxy.PingContext(ctx); err != nil {
		t.Fatalf("PingContext失败: %v", err)
	}
	if err := proxy.Health(ctx); err != nil {
		t.Fatalf("Health失败: %v", err)
	}

	cli := proxy.Client()
	if cli == nil {
		t.Fatal("Client()不应返回nil")
	}

	//准备干净的测试表
	if _, err := cli.NewDropTable().Model((*integrationUser)(nil)).IfExists().Exec(ctx); err != nil {
		t.Fatalf("清理测试表失败: %v", err)
	}
	if _, err := cli.NewCreateTable().Model((*integrationUser)(nil)).Exec(ctx); err != nil {
		t.Fatalf("建表失败: %v", err)
	}

	//写入与查询
	users := []integrationUser{
		{ID: 1, Name: "alice", Age: 30},
		{ID: 2, Name: "bob", Age: 24},
	}
	if _, err := cli.NewInsert().Model(&users).Exec(ctx); err != nil {
		t.Fatalf("插入失败: %v", err)
	}

	single := integrationUser{}
	if err := cli.NewSelect().Model(&single).Where("id = ?", int64(1)).Scan(ctx); err != nil {
		t.Fatalf("单行查询失败: %v", err)
	}
	if single.Name != "alice" || single.Age != 30 {
		t.Fatalf("单行查询结果不符: %+v", single)
	}

	count, err := cli.NewSelect().Model((*integrationUser)(nil)).Count(ctx)
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	if count != 2 {
		t.Fatalf("统计结果不符, 期望2, 实际: %d", count)
	}

	//事务
	err = cli.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().Model((*integrationUser)(nil)).
			Set("age = ?", 40).
			Where("id = ?", int64(2)).
			Exec(ctx)
		return err
	})
	if err != nil {
		t.Fatalf("事务执行失败: %v", err)
	}
	updated := integrationUser{}
	if err := cli.NewSelect().Model(&updated).Where("id = ?", int64(2)).Scan(ctx); err != nil {
		t.Fatalf("事务后查询失败: %v", err)
	}
	if updated.Age != 40 {
		t.Fatalf("事务更新未生效, 实际年龄: %d", updated.Age)
	}

	//带超时的上下文
	queryCtx, cancel := proxy.NewCtx()
	defer cancel()
	var one int
	if err := cli.QueryRowContext(queryCtx, "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("带超时的查询失败: %v", err)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 结果不符: %d", one)
	}

	//重复初始化应被拒绝
	if err := proxy.Init(raw); !errors.Is(err, bunproxy.ErrProxyAlreadySetClient) {
		t.Fatalf("重复初始化应返回ErrProxyAlreadySetClient, 实际: %v", err)
	}

	//关闭与重开
	if err := proxy.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if proxy.IsOk() {
		t.Fatal("关闭后代理不应处于可用状态")
	}
	if err := proxy.PingContext(ctx); !errors.Is(err, bunproxy.ErrProxyNotSetClient) {
		t.Fatalf("关闭后Ping应返回ErrProxyNotSetClient, 实际: %v", err)
	}
	if err := proxy.Close(); !errors.Is(err, bunproxy.ErrProxyNotSetClient) {
		t.Fatalf("重复关闭应返回ErrProxyNotSetClient, 实际: %v", err)
	}

	//关闭时会清空回调,重新初始化不应重复执行回调
	if err := proxy.Init(raw); err != nil {
		t.Fatalf("关闭后重新初始化失败: %v", err)
	}
	if callbackCount != 1 {
		t.Fatalf("关闭后重新初始化不应重复执行回调, 实际执行次数: %d", callbackCount)
	}
	if err := proxy.PingContext(ctx); err != nil {
		t.Fatalf("重新初始化后Ping失败: %v", err)
	}
}

// TestIntegrationMultiBackendSameBinary 验证"同一个二进制 + 停机切配置"的核心场景:
// 同一进程内可以按不同 URL 依次初始化不同后端
func TestIntegrationMultiBackendSameBinary(t *testing.T) {
	type target struct {
		name string
		env  string
	}
	targets := []target{
		{"sqlite", "SQLHELPER_TEST_SQLITE_URL"},
		{"postgres", "SQLHELPER_TEST_POSTGRES_URL"},
	}
	configured := make([]target, 0, len(targets))
	for _, tg := range targets {
		if os.Getenv(tg.env) != "" {
			configured = append(configured, tg)
		}
	}
	if len(configured) < 2 {
		t.Skipf("需要至少配置两个后端(当前 %d 个), 跳过同二进制多后端切换测试", len(configured))
	}

	ctx := context.Background()
	proxy := bunproxy.New()
	for _, tg := range configured {
		raw := os.Getenv(tg.env)
		if err := proxy.Init(raw, bunproxy.WithPingTimeoutMS(5000)); err != nil {
			t.Fatalf("切换到 %s 后端失败: %v", tg.name, err)
		}
		var one int
		if err := proxy.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
			t.Fatalf("%s 后端查询失败: %v", tg.name, err)
		}
		if err := proxy.Close(); err != nil {
			t.Fatalf("关闭 %s 后端失败: %v", tg.name, err)
		}
	}
}

// TestIntegrationMySQLDSNVariants 覆盖真实 MySQL 下几种连接串写法
func TestIntegrationMySQLDSNVariants(t *testing.T) {
	raw := os.Getenv("SQLHELPER_TEST_MYSQL_URL")
	if raw == "" {
		t.Skipf("未配置 SQLHELPER_TEST_MYSQL_URL, 跳过真实MySQL集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	//不带库名的连接串在 MySQL 上是合法的
	noDatabase, err := stripMySQLDatabase(raw)
	if err != nil {
		t.Skipf("无法从 %s 推导不带库名的连接串: %v", raw, err)
	}
	cli, err := bunproxy.NewDB(noDatabase, nil)
	if err != nil {
		t.Fatalf("不带库名的连接串构造失败: %v", err)
	}
	defer func() {
		_ = cli.Close()
	}()
	if err := cli.PingContext(ctx); err != nil {
		t.Fatalf("不带库名的连接串无法连接: %v", err)
	}
}

// stripMySQLDatabase 去掉MySQL连接串中的库名,用于验证无库名场景
func stripMySQLDatabase(raw string) (string, error) {
	U, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	U.Path = ""
	return U.String(), nil
}

// waitForDatabase 等待数据库可连接,超时后直接让用例失败
func waitForDatabase(t *testing.T, raw string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		lastErr = probeDatabase(raw)
		if lastErr == nil {
			return
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("等待数据库就绪超时, URL: %s, 最后一次错误: %v", bunproxy.RedactDSN(raw), lastErr)
		}
		time.Sleep(time.Second)
	}
}

// probeDatabase 尝试建立一次连接并探测可用性
func probeDatabase(raw string) error {
	opts := bunproxy.DefaultOpts
	cli, err := bunproxy.NewDB(raw, &opts)
	if err != nil {
		return err
	}
	defer func() {
		_ = cli.Close()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return cli.PingContext(ctx)
}
