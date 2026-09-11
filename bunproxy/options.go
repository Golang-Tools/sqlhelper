package bunproxy

import (
	"database/sql"
	"time"

	"github.com/Golang-Tools/optparams"
)

// DefaultPingTimeout Init时探测连接可用性的默认超时时间
const DefaultPingTimeout = 5 * time.Second

// Options 数据库代理的配置项
type Options struct {
	//Parallelcallback 为true时Init执行后的回调函数并行执行,默认串行执行
	Parallelcallback bool

	//QueryTimeout 默认的查询超时时间,为0表示不设置超时
	//只在Init方法中生效
	QueryTimeout time.Duration

	//MaxOpenConns 连接池的最大连接数,为0表示使用database/sql的默认行为
	MaxOpenConns int
	//ConnMaxLifetime 连接的最大存活时间,为0表示不设置
	ConnMaxLifetime time.Duration
	//MaxIdleConns 连接池的最大空闲连接数,为0表示使用database/sql的默认行为
	MaxIdleConns int
	//ConnMaxIdleTime 连接的最大空闲时间,为0表示不设置
	ConnMaxIdleTime time.Duration

	//DiscardUnknownColumns 为true时查询结果中存在未知列不会报错
	DiscardUnknownColumns bool

	//QueryLog 为true时开启SQL日志,默认关闭以避免SQL与参数泄漏
	QueryLog bool
	//QueryLogArgs 为true时SQL日志会带上参数值,默认只记录脱敏后的SQL模板
	//QueryLogArgs 只在QueryLog为true时生效
	QueryLogArgs bool

	//DisablePingOnInit 为true时Init不再校验连接的可用性
	DisablePingOnInit bool
	//PingTimeout Init时校验连接可用性的超时时间,为0时使用DefaultPingTimeout
	PingTimeout time.Duration

	//IgnoreCallbackError 为true时Init不会因为回调函数报错而失败
	//该配置用于兼容旧版本"回调错误只记日志"的行为
	IgnoreCallbackError bool
}

// DefaultOpts 当NewDB未传入配置时使用的默认配置
// 注意Init方法不会使用该配置,以保证Options零值时的行为与老版本一致
var DefaultOpts = Options{
	MaxOpenConns:    10,
	MaxIdleConns:    10,
	ConnMaxLifetime: time.Hour,
	ConnMaxIdleTime: 10 * time.Minute,
	PingTimeout:     DefaultPingTimeout,
}

// pingTimeout 返回实际生效的Ping超时时间
func (o *Options) pingTimeout() time.Duration {
	if o == nil || o.PingTimeout <= 0 {
		return DefaultPingTimeout
	}
	return o.PingTimeout
}

// applyPool 将连接池配置应用到*sql.DB上
func (o *Options) applyPool(sqldb *sql.DB) {
	if sqldb == nil {
		return
	}
	if o == nil {
		o = &DefaultOpts
	}
	if o.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(o.MaxIdleConns)
	}
	if o.ConnMaxIdleTime > 0 {
		sqldb.SetConnMaxIdleTime(o.ConnMaxIdleTime)
	}
	if o.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(o.MaxOpenConns)
	}
	if o.ConnMaxLifetime > 0 {
		sqldb.SetConnMaxLifetime(o.ConnMaxLifetime)
	}
}

// WithQueryTimeoutMS 设置最大请求超时,单位ms
func WithQueryTimeoutMS(QueryTimeout int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.QueryTimeout = time.Duration(QueryTimeout) * time.Millisecond
	})
}

// WithQueryTimeout 设置最大请求超时
func WithQueryTimeout(QueryTimeout time.Duration) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		if QueryTimeout > 0 {
			o.QueryTimeout = QueryTimeout
		}
	})
}

// WithParallelCallback 设置初始化后回调并行执行而非串行执行
func WithParallelCallback() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.Parallelcallback = true
	})
}

// WithMaxOpenConns 设置连接池的最大连接数
func WithMaxOpenConns(MaxOpenConns int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.MaxOpenConns = MaxOpenConns
	})
}

// WithConnMaxLifetimeMS 设置连接池的最大连接超时时间,单位ms
func WithConnMaxLifetimeMS(ConnMaxLifetimeMS int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.ConnMaxLifetime = time.Duration(ConnMaxLifetimeMS) * time.Millisecond
	})
}

// WithConnMaxLifetime 设置连接池的最大连接超时时间
func WithConnMaxLifetime(ConnMaxLifetime time.Duration) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		if ConnMaxLifetime > 0 {
			o.ConnMaxLifetime = ConnMaxLifetime
		}
	})
}

// WithMaxIdleConns 设置连接池的最大空闲连接数
func WithMaxIdleConns(MaxIdleConns int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.MaxIdleConns = MaxIdleConns
	})
}

// WithConnMaxIdleTimeMS 设置连接池的最大空闲连接超时时间,单位ms
func WithConnMaxIdleTimeMS(ConnMaxIdleTimeMS int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.ConnMaxIdleTime = time.Duration(ConnMaxIdleTimeMS) * time.Millisecond
	})
}

// WithConnMaxIdleTime 设置连接池的最大空闲连接超时时间
func WithConnMaxIdleTime(ConnMaxIdleTime time.Duration) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		if ConnMaxIdleTime > 0 {
			o.ConnMaxIdleTime = ConnMaxIdleTime
		}
	})
}

// WithPoolConfig 一次性设置连接池的核心参数
func WithPoolConfig(MaxOpenConns int, MaxIdleConns int, ConnMaxLifetime time.Duration, ConnMaxIdleTime time.Duration) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.MaxOpenConns = MaxOpenConns
		o.MaxIdleConns = MaxIdleConns
		if ConnMaxLifetime > 0 {
			o.ConnMaxLifetime = ConnMaxLifetime
		}
		if ConnMaxIdleTime > 0 {
			o.ConnMaxIdleTime = ConnMaxIdleTime
		}
	})
}

// WithDiscardUnknownColumns 设置当有未知列时不报错
func WithDiscardUnknownColumns() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.DiscardUnknownColumns = true
	})
}

// WithQueryLog 开启SQL日志,日志中只包含脱敏后的SQL模板
func WithQueryLog() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.QueryLog = true
	})
}

// WithQueryLogArgs 让SQL日志带上参数值,存在敏感信息泄漏风险,请谨慎使用
func WithQueryLogArgs() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.QueryLog = true
		o.QueryLogArgs = true
	})
}

// WithPingTimeoutMS 设置Init时校验连接可用性的超时时间,单位ms
func WithPingTimeoutMS(PingTimeoutMS int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		if PingTimeoutMS > 0 {
			o.PingTimeout = time.Duration(PingTimeoutMS) * time.Millisecond
		}
	})
}

// WithDisablePingOnInit 关闭Init时的连接可用性校验
func WithDisablePingOnInit() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.DisablePingOnInit = true
	})
}

// WithIgnoreCallbackError 让Init忽略回调函数的错误,兼容旧版本行为
func WithIgnoreCallbackError() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.IgnoreCallbackError = true
	})
}

// WithOptions 用给定配置整体覆盖已有配置
// 选项是合并语义,单个WithXxx无法把布尔开关改回false,
// 需要关闭某个开关时(例如WithParallelCallback之后改回串行)可以用本选项重置配置
func WithOptions(opts Options) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		*o = opts
	})
}
