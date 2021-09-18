package sqlhelper

import (
	"time"
)

//Option 设置key行为的选项
//@attribute MaxTTL time.Duration 为0则不设置过期
//@attribute AutoRefresh string 需要为crontab格式的字符串,否则不会自动定时刷新
type Options struct {
	URL                   string
	Parallelcallback      bool
	QueryTimeout          time.Duration
	MaxOpenConns          int
	ConnMaxLifetime       time.Duration
	MaxIdleConns          int
	ConnMaxIdleTime       time.Duration
	DiscardUnknownColumns bool
}

var DefaultOpts = Options{
	URL: "sqlite://:memory:?cache=shared",
}

// Option configures how we set up the connection.
type Option interface {
	Apply(*Options)
}

// func (emptyOption) apply(*Options) {}
type funcOption struct {
	f func(*Options)
}

func (fo *funcOption) Apply(do *Options) {
	fo.f(do)
}

func newFuncOption(f func(*Options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

//WithQueryTimeout 设置最大请求超时,单位ms
func WithQueryTimeout(QueryTimeout time.Duration) Option {
	return newFuncOption(func(o *Options) {
		o.QueryTimeout = QueryTimeout
	})
}

//WithParallelCallback 设置初始化后回调并行执行而非串行执行
func WithParallelCallback() Option {
	return newFuncOption(func(o *Options) {
		o.Parallelcallback = true
	})
}

//WithURL 使用要连接的数据库管理系统的url
func WithURL(URL string) Option {
	return newFuncOption(func(o *Options) {
		o.URL = URL
	})
}

//WithMaxOpenConns 设置连接池的最大连接数
func WithMaxOpenConns(MaxOpenConns int) Option {
	return newFuncOption(func(o *Options) {
		o.MaxOpenConns = MaxOpenConns
	})
}

//WithConnMaxLifetimeMS 设置连接池的最大连接超时时间,单位ms
func WithConnMaxLifetimeMS(ConnMaxLifetimeMS int) Option {
	return newFuncOption(func(o *Options) {
		o.ConnMaxLifetime = time.Duration(ConnMaxLifetimeMS) * time.Millisecond
	})
}

//WithMaxIdleConns 设置连接池的最大空闲连接数
func WithMaxIdleConns(MaxIdleConns int) Option {
	return newFuncOption(func(o *Options) {
		o.MaxIdleConns = MaxIdleConns
	})
}

//WithConnMaxIdleTimeMS 设置连接池的最大空闲连接超时时间,单位ms
func WithConnMaxIdleTimeMS(ConnMaxIdleTimeMS int) Option {
	return newFuncOption(func(o *Options) {
		o.ConnMaxIdleTime = time.Duration(ConnMaxIdleTimeMS) * time.Millisecond
	})
}

//WithDiscardUnknownColumns 设置当有未知列时不报错
func WithDiscardUnknownColumns() Option {
	return newFuncOption(func(o *Options) {
		o.DiscardUnknownColumns = true
	})
}
