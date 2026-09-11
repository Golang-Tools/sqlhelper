package bunproxy

import (
	"errors"
	"fmt"
)

// 本包中的错误变量分为两组:
//   - ErrProxy.../ErrUnsupportedSchema 等为v3的规范命名,推荐使用
//   - Err...Allready.../ErrUnSupport... 为v2的历史命名,保留用于兼容,后续版本会移除
var (
	//ErrProxyAlreadySetClient 代理已经设置过客户端对象
	ErrProxyAlreadySetClient = errors.New("代理不能重复设置客户端对象")

	//ErrProxyNotSetClient 代理还未设置客户端对象
	ErrProxyNotSetClient = errors.New("代理还未设置客户端对象")

	//ErrUnknownClientType 未知的客户端类型
	ErrUnknownClientType = errors.New("未知的客户端类型")

	//ErrUnsupportedSchema 未支持的数据库管理服务类型
	ErrUnsupportedSchema = errors.New("未支持的数据库管理服务类型")

	//ErrNilDB 客户端对象为nil
	ErrNilDB = errors.New("客户端对象不能为nil")

	//ErrNilCallback 回调函数为nil
	ErrNilCallback = errors.New("回调函数不能为nil")

	//ErrEmptyURL 数据库连接地址为空
	ErrEmptyURL = errors.New("数据库连接地址不能为空")

	//ErrPingFailed Init时校验连接可用性失败
	ErrPingFailed = errors.New("数据库连接校验失败")

	//ErrCallbackPanic 回调函数执行时发生panic
	ErrCallbackPanic = errors.New("回调函数执行时发生panic")
)

// 以下变量为v2的历史命名,存在拼写或语义问题,保留用于兼容
var (
	//Deprecated: 拼写错误,请使用 ErrProxyAlreadySetClient
	ErrProxyAllreadySettedUniversalClient = ErrProxyAlreadySetClient

	//Deprecated: 命名不一致,请使用 ErrProxyNotSetClient
	ErrProxyNotYetSettedUniversalClient = ErrProxyNotSetClient

	//Deprecated: 拼写错误,请使用 ErrUnsupportedSchema
	ErrUnSupportSchema = ErrUnsupportedSchema
)

// CallbackError 回调函数执行失败的错误
// 可以通过 errors.As 获取失败的回调下标与原始错误
type CallbackError struct {
	//Index 回调函数注册时的下标
	Index int
	//Err 回调函数返回的错误
	Err error
}

// Error 实现error接口
func (e *CallbackError) Error() string {
	return fmt.Sprintf("第%d个回调函数执行失败: %s", e.Index, e.Err)
}

// Unwrap 支持 errors.Is/errors.As 链式判断
func (e *CallbackError) Unwrap() error {
	return e.Err
}

// CallbackPanicError 回调函数panic产生的错误
type CallbackPanicError struct {
	//Index 回调函数注册时的下标
	Index int
	//Panic 回调函数panic的内容
	Panic any
}

// Error 实现error接口
func (e *CallbackPanicError) Error() string {
	return fmt.Sprintf("第%d个回调函数执行时发生panic: %v", e.Index, e.Panic)
}

// Unwrap 支持 errors.Is 判断为 ErrCallbackPanic
func (e *CallbackPanicError) Unwrap() error {
	return ErrCallbackPanic
}
