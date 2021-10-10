package sqlhelper

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	log "github.com/Golang-Tools/loggerhelper"

	_ "github.com/go-sql-driver/mysql"
	"github.com/oiime/logrusbun"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
)

//Callback redis操作的回调函数
type Callback func(cli *bun.DB) error

//Proxy bun客户端的代理
type Proxy struct {
	*bun.DB
	callBacks    []Callback
	queryTimeout time.Duration
}

// New 创建一个新的数据库客户端代理
func New() *Proxy {
	proxy := new(Proxy)
	return proxy
}

// IsOk 检查代理是否已经可用
func (proxy *Proxy) IsOk() bool {
	return proxy.DB != nil
}

//SetQueryTimeout 设置连接的请求超时
//@params timeout time.Duration
func (proxy *Proxy) SetQueryTimeout(timeout time.Duration) {
	proxy.queryTimeout = timeout
}

//SetConnect 设置连接的客户端
//@params cli *bun.DB bun的DB对象
func (proxy *Proxy) SetConnect(cli *bun.DB, parallelcallback bool) error {
	if proxy.IsOk() {
		return ErrProxyAllreadySettedUniversalClient
	}
	proxy.DB = cli
	if parallelcallback {
		for _, cb := range proxy.callBacks {
			go func(cb Callback) {
				err := cb(proxy.DB)
				if err != nil {
					log.Error("regist callback get error", log.Dict{"err": err})
				} else {
					log.Debug("regist callback done")
				}
			}(cb)
		}
	} else {
		for _, cb := range proxy.callBacks {
			err := cb(proxy.DB)
			if err != nil {
				log.Error("regist callback get error", log.Dict{"err": err})
			} else {
				log.Debug("regist callback done")
			}
		}
	}
	return nil
}

// SetPool 设置连接池信息
func SetPool(sqldb *sql.DB, opts *Options) {
	if opts.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxIdleTime > 0 {
		sqldb.SetConnMaxIdleTime(opts.ConnMaxIdleTime)
	}
	if opts.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.ConnMaxLifetime > 0 {
		sqldb.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}
}

func NewDB(URL string, dopts *Options) (*bun.DB, error) {
	U, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}
	var cli *bun.DB
	switch U.Scheme {
	case "postgres":
		{
			sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dopts.URL)))
			SetPool(sqldb, dopts)
			if dopts.DiscardUnknownColumns {
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
			dataSourceName := fmt.Sprintf("%stcp(%s)%s?%s", userinfo, U.Host, U.Path, U.RawQuery)
			sqldb, err := sql.Open("mysql", dataSourceName)
			if err != nil {
				return nil, err
			}
			SetPool(sqldb, dopts)
			if dopts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, mysqldialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, mysqldialect.New())
			}
		}
	case "sqlite":
		{
			dataSourceName := strings.ReplaceAll(dopts.URL, fmt.Sprintf("%s://", U.Scheme), "")
			sqldb, err := sql.Open(sqliteshim.ShimName, fmt.Sprintf("file:%s", dataSourceName))
			if err != nil {
				return nil, err
			}
			if !strings.Contains(dataSourceName, ":memory:") {
				SetPool(sqldb, dopts)
			} else {
				sqldb.SetMaxIdleConns(1000)
				sqldb.SetConnMaxLifetime(0)
			}
			if dopts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, sqlitedialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, sqlitedialect.New())
			}
		}
	default:
		{
			return nil, ErrUnSupportSchema
		}
	}
	if dopts.Logger != nil {
		cli.AddQueryHook(logrusbun.NewQueryHook(logrusbun.QueryHookOptions{Logger: dopts.Logger}))
	}
	return cli, nil
}

// Init 初始化代理对象
func (proxy *Proxy) Init(opts ...Option) error {
	dopts := DefaultOpts
	for _, opt := range opts {
		opt.Apply(&dopts)
	}
	var cli *bun.DB
	if dopts.Cli != nil {
		cli = dopts.Cli
	} else {
		_cli, err := NewDB(dopts.URL, &dopts)
		if err != nil {
			return err
		}
		cli = _cli
	}
	return proxy.SetConnect(cli, dopts.Parallelcallback)
}

// Regist 注册回调函数,在init执行后执行回调函数
//如果对象已经设置了被代理客户端则无法再注册回调函数
func (proxy *Proxy) Regist(cb Callback) error {
	if proxy.IsOk() {
		return ErrProxyAllreadySettedUniversalClient
	}
	proxy.callBacks = append(proxy.callBacks, cb)
	return nil
}

// NewCtx 根据注册的超时时间构造一个上下文
func (proxy *Proxy) NewCtx() (ctx context.Context, cancel context.CancelFunc) {
	if proxy.queryTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), proxy.queryTimeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	return
}

//DB 默认的数据库代理对象
var DB = New()
