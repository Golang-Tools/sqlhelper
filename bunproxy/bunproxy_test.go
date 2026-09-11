package bunproxy

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	log "github.com/Golang-Tools/loggerhelper/v4"
	"github.com/uptrace/bun"
)

// memoryURL 核心测试使用的假驱动地址,由 stub_test.go 中注册的 "memory" 驱动提供
const memoryURL = "memory://core-test"

// unreachableURL 模拟数据库不可用的地址,用于验证连通性校验与重试
const unreachableURL = "memory://" + unreachableHost

// newTestDB 创建一个用于测试的数据库连接
func newTestDB(t *testing.T, opts *Options) *bun.DB {
	t.Helper()
	cli, err := NewDB(memoryURL, opts)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = cli.Close()
	})
	return cli
}

func TestNewDBWithNilOptions(t *testing.T) {
	cli := newTestDB(t, nil)
	if err := cli.PingContext(context.Background()); err != nil {
		t.Fatalf("nil配置下创建的连接应可用: %v", err)
	}
}

func TestNewDBEmptyURL(t *testing.T) {
	if _, err := NewDB("   ", nil); !errors.Is(err, ErrEmptyURL) {
		t.Fatalf("空连接串应返回ErrEmptyURL, 实际: %v", err)
	}
}

func TestNewDBUnsupportedSchema(t *testing.T) {
	_, err := NewDB("redis://127.0.0.1:6379", nil)
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("未支持的scheme应返回ErrUnsupportedSchema, 实际: %v", err)
	}
	// 兼容v2的历史命名
	if !errors.Is(err, ErrUnSupportSchema) {
		t.Fatalf("历史错误变量应与新变量等价, 实际: %v", err)
	}
	if !strings.Contains(err.Error(), "redis") {
		t.Fatalf("错误信息中应包含具体的scheme, 实际: %v", err)
	}
}

// 以下用例依赖真实数据库语义,已迁移到对应驱动子模块:
//   - DiscardUnknownColumns / ORM 读写 -> driver/sqlite
//   - MySQL 无库名 DSN / 密码特殊字符 -> driver/mysql
//   - SQLite file: DSN 处理 -> driver/sqlite

func TestProxyInitFailureKeepsOptions(t *testing.T) {
	proxy := New()
	if err := proxy.Init(memoryURL, WithQueryTimeoutMS(1000)); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	err := proxy.Init(memoryURL, WithQueryTimeoutMS(9999))
	if !errors.Is(err, ErrProxyAlreadySetClient) {
		t.Fatalf("重复初始化应返回ErrProxyAlreadySetClient, 实际: %v", err)
	}
	if got := proxy.DefaultQueryTimeout(); got != time.Second {
		t.Fatalf("重复初始化不应修改已有配置, 期望1s, 实际: %v", got)
	}
}

// TestProxyCloseThenReInit 验证关闭后可以重新初始化
func TestProxyCloseThenReInit(t *testing.T) {
	ctx := context.Background()
	proxy := New()
	if err := proxy.Init(memoryURL, WithQueryTimeoutMS(1000)); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	if err := proxy.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("关闭后重新初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if err := proxy.PingContext(ctx); err != nil {
		t.Fatalf("重新初始化后连接应可用: %v", err)
	}
	//未传选项时应沿用上一次成功初始化的配置
	if got := proxy.DefaultQueryTimeout(); got != time.Second {
		t.Fatalf("重新初始化应沿用已有配置, 期望1s, 实际: %v", got)
	}
}

func TestProxyInitAndLifecycle(t *testing.T) {
	ctx := context.Background()
	proxy := New()
	if proxy.IsOk() {
		t.Fatal("新建的代理不应处于可用状态")
	}
	if err := proxy.PingContext(ctx); !errors.Is(err, ErrProxyNotSetClient) {
		t.Fatalf("未初始化时应返回ErrProxyNotSetClient, 实际: %v", err)
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	if !proxy.IsOk() {
		t.Fatal("初始化后代理应处于可用状态")
	}
	if proxy.Client() == nil {
		t.Fatal("初始化后Client()不应为nil")
	}
	if err := proxy.PingContext(ctx); err != nil {
		t.Fatalf("PingContext失败: %v", err)
	}
	if err := proxy.Health(ctx); err != nil {
		t.Fatalf("Health失败: %v", err)
	}
	if err := proxy.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if proxy.IsOk() {
		t.Fatal("关闭后代理应回到未初始化状态")
	}
	if err := proxy.Close(); !errors.Is(err, ErrProxyNotSetClient) {
		t.Fatalf("重复关闭应返回ErrProxyNotSetClient, 实际: %v", err)
	}
}

func TestProxyInitTwice(t *testing.T) {
	proxy := New()
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("首次初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	err := proxy.Init(memoryURL)
	if !errors.Is(err, ErrProxyAlreadySetClient) {
		t.Fatalf("重复初始化应返回ErrProxyAlreadySetClient, 实际: %v", err)
	}
	// 兼容v2的历史命名
	if !errors.Is(err, ErrProxyAllreadySettedUniversalClient) {
		t.Fatalf("历史错误变量应与新变量等价, 实际: %v", err)
	}
}

func TestProxyRegistAfterInit(t *testing.T) {
	proxy := New()
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	err := proxy.Regist(func(cli *bun.DB) error { return nil })
	if !errors.Is(err, ErrProxyAlreadySetClient) {
		t.Fatalf("初始化后注册回调应失败, 实际: %v", err)
	}
}

func TestProxyRegistNilCallback(t *testing.T) {
	proxy := New()
	if err := proxy.Regist(nil); !errors.Is(err, ErrNilCallback) {
		t.Fatalf("注册nil回调应返回ErrNilCallback, 实际: %v", err)
	}
}

func TestProxySetConnectNilDB(t *testing.T) {
	proxy := New()
	if err := proxy.SetConnect(nil); !errors.Is(err, ErrNilDB) {
		t.Fatalf("设置nil连接应返回ErrNilDB, 实际: %v", err)
	}
}

func TestProxyCallbackOrder(t *testing.T) {
	proxy := New()
	var order []int
	for i := 0; i < 3; i++ {
		index := i
		if err := proxy.Register(func(cli *bun.DB) error {
			if cli == nil {
				return errors.New("回调收到的连接为nil")
			}
			order = append(order, index)
			return nil
		}); err != nil {
			t.Fatalf("注册回调失败: %v", err)
		}
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if len(order) != 3 || order[0] != 0 || order[1] != 1 || order[2] != 2 {
		t.Fatalf("串行回调应按注册顺序执行, 实际: %v", order)
	}
}

func TestProxyCallbackError(t *testing.T) {
	sentinel := errors.New("回调失败")
	proxy := New()
	if err := proxy.Regist(func(cli *bun.DB) error { return sentinel }); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	err := proxy.Init(memoryURL)
	if err == nil {
		t.Fatal("回调报错时Init应返回错误")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("错误链中应包含回调的原始错误, 实际: %v", err)
	}
	var cbErr *CallbackError
	if !errors.As(err, &cbErr) {
		t.Fatalf("错误应可断言为*CallbackError, 实际: %v", err)
	}
	if cbErr.Index != 0 {
		t.Fatalf("回调下标应为0, 实际: %d", cbErr.Index)
	}
	if proxy.IsOk() {
		t.Fatal("初始化失败后代理不应处于可用状态")
	}
}

func TestProxyCallbackPanic(t *testing.T) {
	proxy := New()
	if err := proxy.Regist(func(cli *bun.DB) error { panic("回调炸了") }); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	err := proxy.Init(memoryURL)
	if !errors.Is(err, ErrCallbackPanic) {
		t.Fatalf("回调panic应返回ErrCallbackPanic, 实际: %v", err)
	}
	if !strings.Contains(err.Error(), "回调炸了") {
		t.Fatalf("错误信息中应包含panic内容, 实际: %v", err)
	}
	var panicErr *CallbackPanicError
	if !errors.As(err, &panicErr) {
		t.Fatalf("错误应可断言为*CallbackPanicError, 实际: %v", err)
	}
	if proxy.IsOk() {
		t.Fatal("初始化失败后代理不应处于可用状态")
	}
}

func TestProxyIgnoreCallbackError(t *testing.T) {
	proxy := New()
	if err := proxy.Regist(func(cli *bun.DB) error { return errors.New("回调失败") }); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	if err := proxy.Init(memoryURL, WithIgnoreCallbackError()); err != nil {
		t.Fatalf("开启IgnoreCallbackError后Init不应失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if !proxy.IsOk() {
		t.Fatal("初始化后代理应处于可用状态")
	}
}

func TestProxyParallelCallback(t *testing.T) {
	proxy := New()
	var count int64
	firstErr := errors.New("第一个回调失败")
	secondErr := errors.New("第二个回调失败")
	callbacks := []Callback{
		func(cli *bun.DB) error { atomic.AddInt64(&count, 1); return firstErr },
		func(cli *bun.DB) error { atomic.AddInt64(&count, 1); return nil },
		func(cli *bun.DB) error { atomic.AddInt64(&count, 1); return secondErr },
	}
	for _, cb := range callbacks {
		if err := proxy.Regist(cb); err != nil {
			t.Fatalf("注册回调失败: %v", err)
		}
	}
	err := proxy.Init(memoryURL, WithParallelCallback())
	if got := atomic.LoadInt64(&count); got != 3 {
		t.Fatalf("Init返回时所有回调都应执行完毕, 实际执行: %d", got)
	}
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("并行回调的错误都应被聚合返回, 实际: %v", err)
	}
}

func TestProxyPingFailClosesConnection(t *testing.T) {
	proxy := New()
	// 假驱动的 unreachable 地址上连接必定失败
	err := proxy.Init(unreachableURL, WithPingTimeoutMS(500))
	if !errors.Is(err, ErrPingFailed) {
		t.Fatalf("连接不可用时应返回ErrPingFailed, 实际: %v", err)
	}
	if proxy.IsOk() {
		t.Fatal("连接校验失败后代理不应处于可用状态")
	}
	if strings.Contains(err.Error(), "root:root") {
		t.Fatalf("错误信息中不应包含明文密码, 实际: %v", err)
	}
}

func TestProxyDisablePingOnInit(t *testing.T) {
	proxy := New()
	// 关闭校验后Init不报错,便于兼容旧的惰性连接行为
	if err := proxy.Init(unreachableURL, WithDisablePingOnInit()); err != nil {
		t.Fatalf("关闭Ping校验后Init不应失败: %v", err)
	}
	if !proxy.IsOk() {
		t.Fatal("关闭Ping校验后代理应处于可用状态")
	}
	if err := proxy.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
}

func TestProxyNewCtx(t *testing.T) {
	proxy := New()
	if err := proxy.Init(memoryURL, WithQueryTimeoutMS(50)); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if got := proxy.DefaultQueryTimeout(); got != 50*time.Millisecond {
		t.Fatalf("查询超时应为50ms, 实际: %v", got)
	}
	ctx, cancel := proxy.NewCtx()
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("配置了QueryTimeout时NewCtx应带上超时")
	}
	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	child, cancelChild := proxy.NewCtxWithParent(parent)
	defer cancelChild()
	if _, ok := child.Deadline(); !ok {
		t.Fatal("NewCtxWithParent应继承超时配置")
	}
}

func TestProxyNewCtxWithoutTimeout(t *testing.T) {
	proxy := New()
	ctx, cancel := proxy.NewCtx()
	defer cancel()
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("未配置QueryTimeout时NewCtx不应带上超时")
	}
}

func TestProxyConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	proxy := New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = proxy.IsOk()
			_ = proxy.Client()
			_ = proxy.DefaultQueryTimeout()
			_ = proxy.Regist(func(cli *bun.DB) error { return nil })
		}()
	}
	wg.Wait()
	if err := proxy.Init(memoryURL, WithParallelCallback()); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = proxy.IsOk()
			_ = proxy.PingContext(ctx)
			_ = proxy.Health(ctx)
			_ = proxy.Close()
		}()
	}
	wg.Wait()
}

func TestSetPool(t *testing.T) {
	// nil参数不应panic
	SetPool(nil, nil)
	db, err := sql.Open(stubSQLDriverName, "setpool")
	if err != nil {
		t.Fatalf("创建测试连接失败: %v", err)
	}
	defer db.Close()
	SetPool(db, &Options{
		MaxOpenConns:    3,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
	})
	if got := db.Stats().MaxOpenConnections; got != 3 {
		t.Fatalf("最大连接数应为3, 实际: %d", got)
	}
	SetPool(db, nil)
	if got := db.Stats().MaxOpenConnections; got != DefaultOpts.MaxOpenConns {
		t.Fatalf("nil配置时应使用DefaultOpts, 实际: %d", got)
	}
}

func TestRedactDSN(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		secret string
	}{
		{"postgres密码", "postgres://user:secret@localhost:5432/db?sslmode=disable", "secret"},
		{"mysql密码含特殊字符", "mysql://root:p@ss@127.0.0.1:3306/test", "p@ss"},
		{"sqlserver密码", "sqlserver://sa:Passw0rd!@localhost:1433?database=master", "Passw0rd!"},
		{"query中的密码", "postgres://localhost:5432/db?password=secret", "secret"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactDSN(c.in)
			if strings.Contains(got, c.secret) {
				t.Fatalf("脱敏后不应包含明文密码, 输入: %s, 输出: %s", c.in, got)
			}
			if !strings.Contains(got, RedactedPlaceholder) {
				t.Fatalf("脱敏后应包含掩码, 输入: %s, 输出: %s", c.in, got)
			}
		})
	}
	if got := RedactDSN("sqlite://:memory:"); got != "sqlite://:memory:" {
		t.Fatalf("无密码的连接串应原样返回, 实际: %s", got)
	}
	if got := RedactDSN(""); got != "" {
		t.Fatalf("空连接串应返回空字符串, 实际: %s", got)
	}
}

func TestSanitizeSQL(t *testing.T) {
	in := "SELECT  *   FROM users WHERE email = 'a@b.com' AND token = 'abc123'"
	got := SanitizeSQL(in)
	if strings.Contains(got, "a@b.com") || strings.Contains(got, "abc123") {
		t.Fatalf("脱敏后不应包含字符串字面量, 实际: %s", got)
	}
	if !strings.Contains(got, "'?'") {
		t.Fatalf("字符串字面量应被替换为'?', 实际: %s", got)
	}
	if strings.Contains(got, "  ") {
		t.Fatalf("连续空白应被压缩, 实际: %s", got)
	}
}

func TestSanitizeSQLTruncate(t *testing.T) {
	got := SanitizeSQL(strings.Repeat("a", maxLoggedSQLLength+100))
	if !strings.HasSuffix(got, "...(truncated)") {
		t.Fatalf("超长SQL应被截断, 实际长度: %d", len(got))
	}
}

// TestSanitizeSQLMasksLiterals 验证脱敏既遮蔽字面量,又不误伤标识符与占位符
func TestSanitizeSQLMasksLiterals(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "数字字面量",
			in:   "SELECT * FROM users WHERE age > 18 AND score < 99.5",
			want: "SELECT * FROM users WHERE age > ? AND score < ?",
		},
		{
			name: "科学计数法与十六进制",
			in:   "SELECT * FROM t WHERE a = 1e10 AND b = 0xFF",
			want: "SELECT * FROM t WHERE a = ? AND b = ?",
		},
		{
			name: "字符串中的数字一并遮蔽",
			in:   "SELECT * FROM t WHERE phone = '13800000000'",
			want: "SELECT * FROM t WHERE phone = '?'",
		},
		{
			name: "带数字的标识符不受影响",
			in:   "SELECT col1, utf8mb4_text FROM t_user2 WHERE id = 7",
			want: "SELECT col1, utf8mb4_text FROM t_user2 WHERE id = ?",
		},
		{
			name: "postgres位置占位符保持原样",
			in:   "SELECT * FROM users WHERE id = $1 AND name = $2",
			want: "SELECT * FROM users WHERE id = $1 AND name = $2",
		},
		{
			name: "问号占位符与关键字保持原样",
			in:   "SELECT * FROM t WHERE id = ? AND flag IS NOT NULL LIMIT 10",
			want: "SELECT * FROM t WHERE id = ? AND flag IS NOT NULL LIMIT ?",
		},
		{
			name: "空语句",
			in:   "",
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SanitizeSQL(c.in); got != c.want {
				t.Fatalf("脱敏结果不符\n输入: %s\n期望: %s\n实际: %s", c.in, c.want, got)
			}
		})
	}
}

// captureLogOutput 将全局日志输出重定向到buffer,并返回恢复函数
func captureLogOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	log.Set(log.WithOutput(buf), log.WithLevel("debug"))
	t.Cleanup(func() {
		log.Set(log.WithOutput(io.Discard), log.WithLevel("info"))
	})
	return buf
}

func TestQueryLogDoesNotLeakArgs(t *testing.T) {
	buf := captureLogOutput(t)
	ctx := context.Background()
	cli := newTestDB(t, &Options{QueryLog: true})
	if _, err := cli.ExecContext(ctx, "CREATE TABLE t_log (id INTEGER, name TEXT)"); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	const secret = "super_secret_value"
	if _, err := cli.ExecContext(ctx, "INSERT INTO t_log (id, name) VALUES (?, ?)", 1, secret); err != nil {
		t.Fatalf("插入失败: %v", err)
	}
	logged := buf.String()
	if logged == "" {
		t.Skip("当前日志实现未输出Debug日志,跳过泄露检查")
	}
	if strings.Contains(logged, secret) {
		t.Fatalf("默认查询日志不应包含参数值, 实际日志: %s", logged)
	}
	if !strings.Contains(logged, "INSERT INTO t_log") {
		t.Fatalf("查询日志应包含SQL模板, 实际日志: %s", logged)
	}
}

func TestQueryLogWithArgs(t *testing.T) {
	buf := captureLogOutput(t)
	ctx := context.Background()
	cli := newTestDB(t, &Options{QueryLog: true, QueryLogArgs: true})
	if _, err := cli.ExecContext(ctx, "CREATE TABLE t_log_args (id INTEGER)"); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if _, err := cli.ExecContext(ctx, "INSERT INTO t_log_args (id) VALUES (?)", 42); err != nil {
		t.Fatalf("插入失败: %v", err)
	}
	if logged := buf.String(); logged != "" && !strings.Contains(logged, "args") {
		t.Fatalf("开启QueryLogArgs后日志应包含参数, 实际日志: %s", logged)
	}
}

func TestExampleProxyBasicUsage(t *testing.T) {
	ctx := context.Background()
	proxy := New()
	if err := proxy.Init(memoryURL, WithQueryTimeoutMS(3000), WithMaxOpenConns(4)); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	queryCtx, cancel := proxy.NewCtx()
	defer cancel()
	if _, err := proxy.Client().ExecContext(queryCtx, "CREATE TABLE t_example (id INTEGER)"); err != nil {
		t.Fatalf("执行建表语句失败: %v", err)
	}
	if err := proxy.Health(ctx); err != nil {
		t.Fatalf("健康检查失败: %v", err)
	}
}

func TestProxyInitAppliesOptions(t *testing.T) {
	// optparams从v1.0.0起GetOption变为拷贝语义,Apply才是原地语义
	// 这里锁定Init必须把选项真正写入proxy.Opt,否则所有WithXxx都会静默失效
	proxy := New()
	if err := proxy.Init(memoryURL,
		WithQueryTimeoutMS(1500),
		WithMaxOpenConns(7),
		WithMaxIdleConns(5),
		WithConnMaxLifetimeMS(60000),
		WithConnMaxIdleTimeMS(30000),
		WithParallelCallback(),
		WithDiscardUnknownColumns(),
		WithQueryLog(),
		WithPingTimeoutMS(800),
	); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	proxy.mu.RLock()
	opt := proxy.Opt
	proxy.mu.RUnlock()
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"QueryTimeout", opt.QueryTimeout, 1500 * time.Millisecond},
		{"MaxOpenConns", opt.MaxOpenConns, 7},
		{"MaxIdleConns", opt.MaxIdleConns, 5},
		{"ConnMaxLifetime", opt.ConnMaxLifetime, time.Minute},
		{"ConnMaxIdleTime", opt.ConnMaxIdleTime, 30 * time.Second},
		{"Parallelcallback", opt.Parallelcallback, true},
		{"DiscardUnknownColumns", opt.DiscardUnknownColumns, true},
		{"QueryLog", opt.QueryLog, true},
		{"PingTimeout", opt.PingTimeout, 800 * time.Millisecond},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("选项 %s 未生效, 期望: %v, 实际: %v", c.name, c.want, c.got)
		}
	}
}

// TestProxyPromotedQueryContext 验证内嵌的bun.DB方法依然可以直接调用(v2兼容路径)
func TestProxyPromotedQueryContext(t *testing.T) {
	ctx := context.Background()
	proxy := New()
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	rows, err := proxy.QueryContext(ctx, "SELECT 1")
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	defer func() {
		_ = rows.Close()
	}()
	if !rows.Next() {
		t.Fatalf("期望至少返回一行: %v", rows.Err())
	}
	var got int64
	if err := rows.Scan(&got); err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if got != 1 {
		t.Fatalf("查询结果不符, 期望1, 实际: %d", got)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("结果集异常: %v", err)
	}
}

// TestProxyCloseBeforeInitKeepsCallbacks 验证未初始化时调用Close不会清空已注册的回调
func TestProxyCloseBeforeInitKeepsCallbacks(t *testing.T) {
	proxy := New()
	ran := 0
	if err := proxy.Register(func(cli *bun.DB) error {
		ran++
		return nil
	}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	if err := proxy.Close(); !errors.Is(err, ErrProxyNotSetClient) {
		t.Fatalf("未初始化时Close应返回ErrProxyNotSetClient, 实际: %v", err)
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if ran != 1 {
		t.Fatalf("未初始化时Close不应清空回调, 期望执行1次, 实际: %d", ran)
	}
}

// TestProxyCloseClearsCallbacks 验证Close会清空回调,重新Init不重复执行
func TestProxyCloseClearsCallbacks(t *testing.T) {
	proxy := New()
	ran := 0
	if err := proxy.Register(func(cli *bun.DB) error {
		ran++
		return nil
	}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	if err := proxy.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("关闭后重新初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if ran != 1 {
		t.Fatalf("关闭后重新初始化不应重复执行回调, 实际执行: %d", ran)
	}
	err := proxy.Register(func(cli *bun.DB) error { return nil })
	if !errors.Is(err, ErrProxyAlreadySetClient) {
		t.Fatalf("重新初始化后再注册回调应失败, 实际: %v", err)
	}
}

// TestWithOptionsOverridesConfig 验证WithOptions可以整体覆盖配置,把布尔开关改回false
func TestWithOptionsOverridesConfig(t *testing.T) {
	proxy := New()
	if err := proxy.Init(memoryURL, WithParallelCallback(), WithQueryTimeoutMS(1000)); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	if err := proxy.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if err := proxy.Init(memoryURL, WithOptions(Options{QueryTimeout: 2 * time.Second})); err != nil {
		t.Fatalf("重新初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if proxy.Opt.Parallelcallback {
		t.Fatal("WithOptions应能把Parallelcallback改回false")
	}
	if got := proxy.DefaultQueryTimeout(); got != 2*time.Second {
		t.Fatalf("QueryTimeout应为2s, 实际: %v", got)
	}
}

// TestProxyCloseRaceWithQuery 验证关闭与业务查询并发时不会panic
// Close不再置空内嵌连接指针,因此已持有的连接对象在使用时只会得到明确错误
func TestProxyCloseRaceWithQuery(t *testing.T) {
	ctx := context.Background()
	proxy := New()
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	cli := proxy.Client()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				var one int
				_ = cli.QueryRowContext(ctx, "SELECT 1").Scan(&one)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = proxy.Close()
	}()
	wg.Wait()
	if proxy.IsOk() {
		t.Fatal("关闭后代理不应处于可用状态")
	}
}

// TestWithDefaultOpts 验证WithDefaultOpts会整体套用推荐默认值
func TestWithDefaultOpts(t *testing.T) {
	proxy := New()
	if err := proxy.Init(memoryURL, WithDefaultOpts()); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	opt := proxy.Opt
	if opt.MaxOpenConns != DefaultOpts.MaxOpenConns ||
		opt.MaxIdleConns != DefaultOpts.MaxIdleConns ||
		opt.ConnMaxLifetime != DefaultOpts.ConnMaxLifetime ||
		opt.ConnMaxIdleTime != DefaultOpts.ConnMaxIdleTime {
		t.Fatalf("WithDefaultOpts应套用DefaultOpts, 实际: %+v", opt)
	}
}

// TestHealthUsesPingTimeout 验证Health的超时来自PingTimeout而不是QueryTimeout
func TestHealthUsesPingTimeout(t *testing.T) {
	ctx := context.Background()

	//QueryTimeout极小但PingTimeout充足时,健康检查应该成功
	proxy := New()
	if err := proxy.Init(memoryURL, WithOptions(Options{
		QueryTimeout: time.Nanosecond,
		PingTimeout:  30 * time.Second,
	})); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if got := proxy.PingTimeout(); got != 30*time.Second {
		t.Fatalf("PingTimeout应为30s, 实际: %v", got)
	}
	if err := proxy.Health(ctx); err != nil {
		t.Fatalf("Health应使用PingTimeout, 不应受QueryTimeout影响: %v", err)
	}

	//PingTimeout已过期时健康检查应失败
	expired := New()
	if err := expired.Init(memoryURL, WithOptions(Options{
		PingTimeout:       time.Nanosecond,
		DisablePingOnInit: true,
	})); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = expired.Close()
	}()
	err := expired.Health(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("PingTimeout过期时Health应返回DeadlineExceeded, 实际: %v", err)
	}
}

// TestRegisterContext 验证带上下文的回调按注册顺序执行并带上超时
func TestRegisterContext(t *testing.T) {
	proxy := New()
	order := make([]string, 0, 2)
	var (
		gotDeadline bool
		gotTimeout  time.Duration
	)
	if err := proxy.Regist(func(cli *bun.DB) error {
		order = append(order, "plain")
		return nil
	}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	if err := proxy.RegisterContext(func(ctx context.Context, cli *bun.DB) error {
		order = append(order, "context")
		if deadline, ok := ctx.Deadline(); ok {
			gotDeadline = true
			gotTimeout = time.Until(deadline)
		}
		var one int
		return cli.QueryRowContext(ctx, "SELECT 1").Scan(&one)
	}); err != nil {
		t.Fatalf("注册带上下文回调失败: %v", err)
	}
	if err := proxy.Init(memoryURL, WithCallbackTimeoutMS(2000)); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()

	if len(order) != 2 || order[0] != "plain" || order[1] != "context" {
		t.Fatalf("回调应按注册顺序执行, 实际: %v", order)
	}
	if !gotDeadline {
		t.Fatal("配置CallbackTimeout后回调上下文应带超时")
	}
	if gotTimeout <= 0 || gotTimeout > 2*time.Second {
		t.Fatalf("回调超时时间不符, 实际剩余: %v", gotTimeout)
	}

	//未配置CallbackTimeout时回调上下文不设超时
	plain := New()
	hasDeadline := false
	if err := plain.RegisterContext(func(ctx context.Context, cli *bun.DB) error {
		_, hasDeadline = ctx.Deadline()
		return nil
	}); err != nil {
		t.Fatalf("注册带上下文回调失败: %v", err)
	}
	if err := plain.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = plain.Close()
	}()
	if hasDeadline {
		t.Fatal("未配置CallbackTimeout时回调上下文不应带超时")
	}
}

// TestRegisterContextErrors 验证RegisterContext的入参校验与状态校验
func TestRegisterContextErrors(t *testing.T) {
	proxy := New()
	if err := proxy.RegisterContext(nil); !errors.Is(err, ErrNilCallback) {
		t.Fatalf("注册nil回调应返回ErrNilCallback, 实际: %v", err)
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	err := proxy.RegisterContext(func(ctx context.Context, cli *bun.DB) error { return nil })
	if !errors.Is(err, ErrProxyAlreadySetClient) {
		t.Fatalf("初始化后注册带上下文回调应失败, 实际: %v", err)
	}
}

// TestCallbackTimeoutInterrupts 验证回调超时能中断卡住的回调
func TestCallbackTimeoutInterrupts(t *testing.T) {
	proxy := New()
	if err := proxy.RegisterContext(func(ctx context.Context, cli *bun.DB) error {
		<-ctx.Done()
		return ctx.Err()
	}); err != nil {
		t.Fatalf("注册带上下文回调失败: %v", err)
	}
	start := time.Now()
	err := proxy.Init(memoryURL, WithCallbackTimeoutMS(50))
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("回调超时应返回DeadlineExceeded, 实际: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("回调超时未生效, 耗时: %v", elapsed)
	}
	if proxy.IsOk() {
		t.Fatal("回调失败后代理不应处于可用状态")
	}
}

// TestConnectRetry 验证连通性校验失败会按配置重试
func TestConnectRetry(t *testing.T) {
	proxy := New()
	start := time.Now()
	err := proxy.Init(unreachableURL,
		WithPingTimeoutMS(200),
		WithConnectRetry(3, 30*time.Millisecond),
	)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrPingFailed) {
		t.Fatalf("重试耗尽后应返回ErrPingFailed, 实际: %v", err)
	}
	//重试间隔为30ms与60ms,总耗时必然大于等于60ms
	if elapsed < 60*time.Millisecond {
		t.Fatalf("重试未生效, 耗时: %v", elapsed)
	}
	if proxy.IsOk() {
		t.Fatal("重试全部失败后代理不应处于可用状态")
	}
}

// TestConnectRetrySkipsConfigError 验证配置类错误不会触发重试
func TestConnectRetrySkipsConfigError(t *testing.T) {
	proxy := New()
	start := time.Now()
	err := proxy.Init("redis://127.0.0.1:6379", WithConnectRetry(5, 600*time.Millisecond))
	elapsed := time.Since(start)
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("应返回ErrUnsupportedSchema, 实际: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("配置类错误不应重试, 耗时: %v", elapsed)
	}
}

// TestInitContextCanceled 验证可取消初始化
func TestInitContextCanceled(t *testing.T) {
	proxy := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := proxy.InitContext(ctx, memoryURL)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的ctx应让InitContext返回context.Canceled, 实际: %v", err)
	}
	if proxy.IsReady() {
		t.Fatal("初始化失败后代理不应处于可用状态")
	}
}

// TestInitContextCanceledDuringRetry 验证重试等待期间可以被ctx打断
func TestInitContextCanceledDuringRetry(t *testing.T) {
	proxy := New()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := proxy.InitContext(ctx, unreachableURL,
		WithPingTimeoutMS(200),
		WithConnectRetry(5, 2*time.Second),
	)
	elapsed := time.Since(start)
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("应返回ctx相关错误, 实际: %v", err)
	}
	if elapsed > time.Second {
		t.Fatalf("重试等待应被ctx打断, 实际耗时: %v", elapsed)
	}
	if proxy.IsReady() {
		t.Fatal("初始化失败后代理不应处于可用状态")
	}
}

// TestIsReadyAlias 验证IsReady与IsOk语义一致
func TestIsReadyAlias(t *testing.T) {
	proxy := New()
	if proxy.IsReady() || proxy.IsOk() {
		t.Fatal("新建代理不应处于可用状态")
	}
	if err := proxy.Init(memoryURL); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer func() {
		_ = proxy.Close()
	}()
	if !proxy.IsReady() || !proxy.IsOk() {
		t.Fatal("初始化后两种判断都应返回可用")
	}
}

func BenchmarkNewDBStub(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cli, err := NewDB(memoryURL, nil)
		if err != nil {
			b.Fatalf("创建数据库失败: %v", err)
		}
		if err := cli.Close(); err != nil {
			b.Fatalf("关闭数据库失败: %v", err)
		}
	}
}

func TestMain(m *testing.M) {
	// 避免测试日志污染标准输出
	log.Set(log.WithOutput(io.Discard), log.WithLevel("info"))
	os.Exit(m.Run())
}
