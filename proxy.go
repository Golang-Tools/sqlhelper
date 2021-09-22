package sqlhelper

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	log "github.com/Golang-Tools/loggerhelper"

	_ "github.com/go-sql-driver/mysql"
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
	opts      Options
	callBacks []Callback
}

// New 创建一个新的数据库客户端代理
func New() *Proxy {
	proxy := new(Proxy)
	proxy.opts = DefaultOpts
	return proxy
}

// IsOk 检查代理是否已经可用
func (proxy *Proxy) IsOk() bool {
	return proxy.DB != nil
}

//SetConnect 设置连接的客户端
//@params cli *bun.DB bun的DB对象
func (proxy *Proxy) SetConnect(cli *bun.DB) error {
	if proxy.IsOk() {
		return ErrProxyAllreadySettedUniversalClient
	}
	proxy.DB = cli
	if proxy.opts.Parallelcallback {
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

func (proxy *Proxy) SetPool(sqldb *sql.DB) {
	if proxy.opts.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(proxy.opts.MaxIdleConns)
	}
	if proxy.opts.ConnMaxIdleTime > 0 {
		sqldb.SetConnMaxIdleTime(proxy.opts.ConnMaxIdleTime)
	}
	if proxy.opts.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(proxy.opts.MaxOpenConns)
	}
	if proxy.opts.ConnMaxLifetime > 0 {
		sqldb.SetConnMaxLifetime(proxy.opts.ConnMaxLifetime)
	}
}

func (proxy *Proxy) Init(opts ...Option) error {
	for _, opt := range opts {
		opt.Apply(&proxy.opts)
	}
	var cli *bun.DB
	U, err := url.Parse(proxy.opts.URL)
	if err != nil {
		return err
	}
	switch U.Scheme {
	case "postgres":
		{
			sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(proxy.opts.URL)))
			proxy.SetPool(sqldb)
			if proxy.opts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, pgdialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, pgdialect.New())
			}
		}
	case "mysql":
		{
			dataSourceName := fmt.Sprintf("%s@tcp(%s)%s?%s", U.User.String(), U.Host, U.Path, U.RawQuery)
			sqldb, err := sql.Open("mysql", dataSourceName)
			if err != nil {
				return err
			}
			proxy.SetPool(sqldb)
			if proxy.opts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, mysqldialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, mysqldialect.New())
			}
		}
	case "sqlite":
		{
			dataSourceName := strings.ReplaceAll(proxy.opts.URL, fmt.Sprintf("%s://", U.Scheme), "")
			sqldb, err := sql.Open(sqliteshim.ShimName, fmt.Sprintf("file:%s", dataSourceName))
			if err != nil {
				return err
			}
			if !strings.Contains(dataSourceName, ":memory:") {
				proxy.SetPool(sqldb)
			} else {
				sqldb.SetMaxIdleConns(1000)
				sqldb.SetConnMaxLifetime(0)
			}
			if proxy.opts.DiscardUnknownColumns {
				cli = bun.NewDB(sqldb, sqlitedialect.New(), bun.WithDiscardUnknownColumns())
			} else {
				cli = bun.NewDB(sqldb, sqlitedialect.New())
			}
		}
	default:
		{
			return ErrUnSupportSchema
		}
	}
	return proxy.SetConnect(cli)
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
	if proxy.opts.QueryTimeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), proxy.opts.QueryTimeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	return
}

//DB 默认的数据库代理对象
var DB = New()
