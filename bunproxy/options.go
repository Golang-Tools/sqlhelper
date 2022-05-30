package bunproxy

import (
	"time"

	"github.com/Golang-Tools/optparams"
)

//Option 设置key行为的选项
//@attribute MaxTTL time.Duration 为0则不设置过期
//@attribute AutoRefresh string 需要为crontab格式的字符串,否则不会自动定时刷新
type Options struct {
	Parallelcallback bool          // 只在Init方法中生效
	QueryTimeout     time.Duration // 只在Init方法中生效

	MaxOpenConns          int
	ConnMaxLifetime       time.Duration
	MaxIdleConns          int
	ConnMaxIdleTime       time.Duration
	DiscardUnknownColumns bool
}

var DefaultOpts = Options{}

//WithQueryTimeoutMS 设置最大请求超时,单位ms
func WithQueryTimeoutMS(QueryTimeout int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.QueryTimeout = time.Duration(QueryTimeout) * time.Millisecond
	})
}

//WithParallelCallback 设置初始化后回调并行执行而非串行执行
func WithParallelCallback() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.Parallelcallback = true
	})
}

//WithMaxOpenConns 设置连接池的最大连接数
func WithMaxOpenConns(MaxOpenConns int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.MaxOpenConns = MaxOpenConns
	})
}

//WithConnMaxLifetimeMS 设置连接池的最大连接超时时间,单位ms
func WithConnMaxLifetimeMS(ConnMaxLifetimeMS int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.ConnMaxLifetime = time.Duration(ConnMaxLifetimeMS) * time.Millisecond
	})
}

//WithMaxIdleConns 设置连接池的最大空闲连接数
func WithMaxIdleConns(MaxIdleConns int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.MaxIdleConns = MaxIdleConns
	})
}

//WithConnMaxIdleTimeMS 设置连接池的最大空闲连接超时时间,单位ms
func WithConnMaxIdleTimeMS(ConnMaxIdleTimeMS int) optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.ConnMaxIdleTime = time.Duration(ConnMaxIdleTimeMS) * time.Millisecond
	})
}

//WithDiscardUnknownColumns 设置当有未知列时不报错
func WithDiscardUnknownColumns() optparams.Option[Options] {
	return optparams.NewFuncOption(func(o *Options) {
		o.DiscardUnknownColumns = true
	})
}
