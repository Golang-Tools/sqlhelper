package bunproxy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/Golang-Tools/loggerhelper/v4"
	"github.com/Golang-Tools/optparams"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mssqldialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// Logger 本包使用的日志对象
var Logger = log.Export()

// Callback 在连接建立后执行的回调函数
// 回调函数中不应调用代理的Init/Close方法,否则可能造成死锁
type Callback func(cli *bun.DB) error

// CallbackContext 带上下文的回调函数
// 适合需要超时保护的操作;超时时间由CallbackTimeout/WithCallbackTimeout配置,为0表示不限制
type CallbackContext func(ctx context.Context, cli *bun.DB) error

// callbackEntry 内部统一存储的回调条目,普通回调会被适配成相同签名
type callbackEntry func(ctx context.Context, cli *bun.DB) error

// Proxy bun客户端的代理
// 除Init/Close外的方法都是并发安全的,Init与Close应在应用启动/退出阶段串行调用
// 内嵌的*bun.DB在初始化成功后不再变更(Close只关闭连接池并置位closed),因此
// 业务查询与外发提升方法在稳态下不会因为Close而产生数据竞争
// 但直接访问proxy.DB仍然会绕过内部读锁,并发Init场景请改用Client()
type Proxy struct {
	mu sync.RWMutex
	*bun.DB
	closed    bool
	callBacks []callbackEntry
	Opt       Options
}

// New 创建一个新的数据库客户端代理
func New() *Proxy {
	proxy := new(Proxy)
	return proxy
}

// IsOk 检查代理是否已经可用
func (proxy *Proxy) IsOk() bool {
	return proxy.Client() != nil
}

// Client 返回底层bun.DB对象,代理未初始化或已关闭时返回nil
func (proxy *Proxy) Client() *bun.DB {
	proxy.mu.RLock()
	defer proxy.mu.RUnlock()
	if proxy.closed {
		return nil
	}
	return proxy.DB
}

// SetConnect 设置连接的客户端
// @params cli *bun.DB bun的DB对象
func (proxy *Proxy) SetConnect(cli *bun.DB) error {
	proxy.mu.RLock()
	parallel := proxy.Opt.Parallelcallback
	ignoreErr := proxy.Opt.IgnoreCallbackError
	callbackTimeout := proxy.Opt.CallbackTimeout
	proxy.mu.RUnlock()
	return proxy.setConnect(cli, parallel, ignoreErr, callbackTimeout)
}

// setConnect 使用显式指定的回调配置建立连接
// parallel/ignoreErr/callbackTimeout由调用方传入,便于Init在不污染proxy.Opt的前提下使用本次入参
func (proxy *Proxy) setConnect(cli *bun.DB, parallel bool, ignoreErr bool, callbackTimeout time.Duration) error {
	if cli == nil {
		return ErrNilDB
	}
	proxy.mu.Lock()
	//已关闭的代理允许重新设置连接
	if proxy.DB != nil && !proxy.closed {
		proxy.mu.Unlock()
		return ErrProxyAlreadySetClient
	}
	proxy.DB = cli
	proxy.closed = false
	callbacks := make([]callbackEntry, len(proxy.callBacks))
	copy(callbacks, proxy.callBacks)
	proxy.mu.Unlock()

	//回调在锁外执行,避免用户回调重入代理方法时死锁
	errs := runCallbacks(cli, callbacks, parallel, callbackTimeout)
	if len(callbacks) > 0 {
		for _, err := range errs {
			log.Error("regist callback get error", log.Dict{"err": err.Error()})
		}
		log.Debug("regist callback done", log.Dict{"total": len(callbacks), "failed": len(errs)})
	}

	if len(errs) > 0 && !ignoreErr {
		//回调失败时回滚连接,保证SetConnect失败后代理仍处于未初始化状态
		proxy.mu.Lock()
		if proxy.DB == cli {
			proxy.closed = true
		}
		proxy.mu.Unlock()
		return errors.Join(errs...)
	}
	return nil
}

// Close 关闭代理持有的连接并释放连接池
// 关闭后代理会回到未初始化状态(Client/IsOk 不再返回连接),可以重新Init
// 已注册的回调会在关闭时被清空,避免重新Init时重复执行回调中的副作用操作
// 重复调用Close会返回ErrProxyNotSetClient
func (proxy *Proxy) Close() error {
	proxy.mu.Lock()
	cli := proxy.DB
	if cli == nil || proxy.closed {
		proxy.mu.Unlock()
		return ErrProxyNotSetClient
	}
	//只关闭连接池并置位状态,不改写内嵌指针,避免关闭路径与业务查询产生数据竞争
	proxy.closed = true
	proxy.callBacks = nil
	proxy.mu.Unlock()
	return cli.Close()
}

// PingContext 检查底层连接是否可用
func (proxy *Proxy) PingContext(ctx context.Context) error {
	cli := proxy.Client()
	if cli == nil {
		return ErrProxyNotSetClient
	}
	return cli.PingContext(nonNilContext(ctx))
}

// Health 检查代理是否健康
// 超时使用配置的PingTimeout(默认DefaultPingTimeout),与Init的连通性校验保持一致
func (proxy *Proxy) Health(ctx context.Context) error {
	ctx = nonNilContext(ctx)
	ctx, cancel := context.WithTimeout(ctx, proxy.PingTimeout())
	defer cancel()
	return proxy.PingContext(ctx)
}

// PingTimeout 返回实际生效的连通性探测超时时间
func (proxy *Proxy) PingTimeout() time.Duration {
	proxy.mu.RLock()
	opt := proxy.Opt
	proxy.mu.RUnlock()
	return opt.pingTimeout()
}

// SetPool 设置连接池信息,opts为nil时使用DefaultOpts
func SetPool(sqldb *sql.DB, opts *Options) {
	opts.applyPool(sqldb)
}

// NewDB 根据连接串创建bun.DB对象
// opts为nil时使用DefaultOpts,调用方负责在不再使用时调用Close释放连接池
func NewDB(URL string, dopts *Options) (*bun.DB, error) {
	opts := dopts
	if opts == nil {
		defaultOpts := DefaultOpts
		opts = &defaultOpts
	}
	trimmedURL := strings.TrimSpace(URL)
	if trimmedURL == "" {
		return nil, ErrEmptyURL
	}
	U, err := url.Parse(trimmedURL)
	if err != nil {
		return nil, fmt.Errorf("解析数据库连接串失败,URL: %s, %w", RedactDSN(trimmedURL), err)
	}

	var cli *bun.DB
	switch U.Scheme {
	case "postgres":
		{
			sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(trimmedURL)))
			opts.applyPool(sqldb)
			if opts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, pgdialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, pgdialect.New())
			}
		}
	case "mysql":
		{
			userinfo := ""
			username := U.User.Username()
			pwd, ok := U.User.Password()
			if ok && username != "" {
				userinfo = fmt.Sprintf("%s:%s@", username, pwd)
			} else if ok && username == "" {
				userinfo = fmt.Sprintf(":%s@", pwd)
			} else if !ok && username != "" {
				userinfo = fmt.Sprintf("%s@", username)
			}
			//MySQL的DSN形如 user:pass@tcp(host:port)/db?params,即使不指定库名也必须保留斜杠
			dbPath := U.Path
			if dbPath == "" {
				dbPath = "/"
			}
			dataSourceName := fmt.Sprintf("%stcp(%s)%s?%s", userinfo, U.Host, dbPath, U.RawQuery)
			sqldb, err := sql.Open("mysql", dataSourceName)
			if err != nil {
				return nil, fmt.Errorf("创建 %s 数据库连接失败,DSN: %s, %w", U.Scheme, RedactDSN(trimmedURL), err)
			}
			opts.applyPool(sqldb)
			if opts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, mysqldialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, mysqldialect.New())
			}
		}
	case "sqlserver":
		{
			sqldb, err := sql.Open("sqlserver", trimmedURL)
			if err != nil {
				return nil, fmt.Errorf("创建 %s 数据库连接失败,DSN: %s, %w", U.Scheme, RedactDSN(trimmedURL), err)
			}
			opts.applyPool(sqldb)
			if opts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, mssqldialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, mssqldialect.New())
			}
		}
	case "sqlite":
		{
			//只去掉scheme前缀,避免误伤路径中出现的同名内容
			dataSourceName := strings.TrimPrefix(trimmedURL, fmt.Sprintf("%s://", U.Scheme))
			//sqlite的DSN为 file:xxx?params 形式,已经带file:前缀时不再重复添加
			dsn := dataSourceName
			if !strings.HasPrefix(dsn, "file:") {
				dsn = "file:" + dsn
			}
			sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
			if err != nil {
				return nil, fmt.Errorf("创建 %s 数据库连接失败,DSN: %s, %w", U.Scheme, RedactDSN(trimmedURL), err)
			}
			switch {
			case !strings.Contains(dataSourceName, ":memory:"),
				strings.Contains(dataSourceName, "cache=shared"):
				opts.applyPool(sqldb)
			default:
				//内存数据库的每个连接都是独立的库,需要限制为单连接以免不同连接看到不同的数据
				sqldb.SetMaxIdleConns(1)
				sqldb.SetMaxOpenConns(1)
				sqldb.SetConnMaxLifetime(0)
			}
			if opts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, sqlitedialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, sqlitedialect.New())
			}
		}
	default:
		{
			return nil, fmt.Errorf("%w: %s", ErrUnsupportedSchema, U.Scheme)
		}
	}

	if opts.QueryLog {
		cli.AddQueryHook(newQueryLogHook(opts.QueryLogArgs))
	}

	return cli, nil
}

// Init 初始化代理对象
// 初始化时会校验连接可用性,校验失败或回调失败都会释放已创建的连接
// 已经初始化过时直接返回ErrProxyAlreadySetClient,不会重复创建连接
func (proxy *Proxy) Init(URL string, opts ...optparams.Option[Options]) error {
	if proxy.Client() != nil {
		return ErrProxyAlreadySetClient
	}

	proxy.mu.RLock()
	//optparams从v1.0.0开始GetOption变为纯函数语义,这里基于现有配置合并本次入参
	opt := *optparams.GetOption(&proxy.Opt, opts...)
	proxy.mu.RUnlock()

	cli, err := proxy.dial(URL, &opt)
	if err != nil {
		return err
	}

	if err := proxy.setConnect(cli, opt.Parallelcallback, opt.IgnoreCallbackError, opt.CallbackTimeout); err != nil {
		//设置失败时释放连接,避免连接池泄漏
		_ = cli.Close()
		return err
	}

	//连接建立成功后才把本次选项写入代理,初始化失败不会污染已有配置
	proxy.mu.Lock()
	proxy.Opt = opt
	proxy.mu.Unlock()
	return nil
}

// dial 建立连接并按配置做连通性校验与重试
// 只有连通性校验失败才会重试,DSN非法等配置类错误会直接返回
func (proxy *Proxy) dial(URL string, opt *Options) (*bun.DB, error) {
	attempts := opt.attempts()
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			interval := opt.retryInterval(attempt)
			log.Warn("重试建立数据库连接", log.Dict{
				"attempt":  attempt,
				"total":    attempts,
				"interval": interval.String(),
				"URL":      RedactDSN(URL),
			})
			time.Sleep(interval)
		}

		cli, err := NewDB(URL, opt)
		if err != nil {
			return nil, err
		}

		if opt.DisablePingOnInit {
			//关闭校验时没有可重试的判定依据,直接返回
			return cli, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), opt.pingTimeout())
		lastErr = cli.PingContext(ctx)
		cancel()
		if lastErr == nil {
			return cli, nil
		}
		//校验失败时释放连接,避免连接池泄漏
		_ = cli.Close()
	}

	return nil, fmt.Errorf("%w, URL: %s, %w", ErrPingFailed, RedactDSN(URL), lastErr)
}

// Regist 注册回调函数,在init执行后执行回调函数
// 如果对象已经设置了被代理客户端则无法再注册回调函数
func (proxy *Proxy) Regist(cb Callback) error {
	if cb == nil {
		return ErrNilCallback
	}
	return proxy.register(func(ctx context.Context, cli *bun.DB) error {
		return cb(cli)
	})
}

// Register Regist的别名,命名更符合Go的惯例
func (proxy *Proxy) Register(cb Callback) error {
	return proxy.Regist(cb)
}

// RegisterContext 注册带上下文的回调函数
// 与Regist注册的回调按注册顺序统一执行,执行时会带上CallbackTimeout配置的超时
func (proxy *Proxy) RegisterContext(cb CallbackContext) error {
	if cb == nil {
		return ErrNilCallback
	}
	return proxy.register(callbackEntry(cb))
}

// register 把回调加入待执行列表
func (proxy *Proxy) register(entry callbackEntry) error {
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.DB != nil && !proxy.closed {
		return ErrProxyAlreadySetClient
	}
	proxy.callBacks = append(proxy.callBacks, entry)
	return nil
}

// NewCtx 根据注册的超时时间构造一个上下文
func (proxy *Proxy) NewCtx() (ctx context.Context, cancel context.CancelFunc) {
	return proxy.NewCtxWithParent(context.Background())
}

// NewCtxWithParent 基于父上下文构造一个带超时的上下文
// 父上下文为nil时会退化为context.Background
func (proxy *Proxy) NewCtxWithParent(parent context.Context) (ctx context.Context, cancel context.CancelFunc) {
	parent = nonNilContext(parent)
	if timeout := proxy.DefaultQueryTimeout(); timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

// DefaultQueryTimeout 返回配置的默认查询超时时间
func (proxy *Proxy) DefaultQueryTimeout() time.Duration {
	proxy.mu.RLock()
	defer proxy.mu.RUnlock()
	return proxy.Opt.QueryTimeout
}

// nonNilContext 保证上下文非nil
func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// execCallback 执行单个回调函数,捕获panic并包装错误
// timeout大于0时会为回调构造带超时的上下文
func execCallback(cli *bun.DB, cb callbackEntry, index int, timeout time.Duration) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &CallbackPanicError{Index: index, Panic: r}
		}
	}()
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if err := cb(ctx, cli); err != nil {
		return &CallbackError{Index: index, Err: err}
	}
	return nil
}

// runCallbacks 执行回调函数,返回按注册顺序排列的错误列表
// parallel为true时并行执行并等待全部回调结束,保证Init返回时回调已经执行完毕
func runCallbacks(cli *bun.DB, callbacks []callbackEntry, parallel bool, timeout time.Duration) []error {
	if len(callbacks) == 0 {
		return nil
	}
	results := make([]error, len(callbacks))
	run := func(index int, cb callbackEntry) {
		if cb == nil {
			return
		}
		results[index] = execCallback(cli, cb, index, timeout)
	}
	if parallel {
		var wg sync.WaitGroup
		wg.Add(len(callbacks))
		for index, cb := range callbacks {
			go func(index int, cb callbackEntry) {
				defer wg.Done()
				run(index, cb)
			}(index, cb)
		}
		wg.Wait()
	} else {
		for index, cb := range callbacks {
			run(index, cb)
		}
	}
	errs := make([]error, 0, len(callbacks))
	for _, err := range results {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// Default 默认的数据库代理对象
var Default = New()
