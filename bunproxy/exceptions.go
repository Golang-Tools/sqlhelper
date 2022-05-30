package bunproxy

import (
	"errors"
)

//ErrProxyAllreadySettedUniversalClient 代理已经设置过redis客户端对象
var ErrProxyAllreadySettedUniversalClient = errors.New("代理不能重复设置客户端对象")

//ErrProxyNotYetSettedUniversalClient 代理还未设置客户端对象
var ErrProxyNotYetSettedUniversalClient = errors.New("代理还未设置客户端对象")

//ErrUnknownClientType 未知的redis客户端类型
var ErrUnknownClientType = errors.New("未知的redis客户端类型")

//ErrUnSupportSchema 未支持的数据库管理服务类型
var ErrUnSupportSchema = errors.New("未支持的数据库管理服务类型")
