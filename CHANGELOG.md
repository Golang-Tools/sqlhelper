# v2.0.0

+ 大规模接口变更,使用grpc风格的接口设计,同时将proxy改作子模块
+ 使用泛型语法,因此取消对低于go1.18版本的支持
+ 更新依赖版本

# v0.0.7

## 接口优化

+ 新增了接口`NewDB`用于创建`*bun.DB`的实例
+ 将Init中创建`*bun.DB`实例的逻辑移出到`NewDB`中
+ 增加参数`WithInstance(cli bun.IDB)`可以直接设置连接对象
+ 不再保存`Init`的参数

## 文档优化

+ 增加接口文档

# v0.0.6

## 新增接口

新增接口`SetQueryTimeout(timeout time.Duration)`用于为请求修改默认请求超时

# v0.0.5

## bug修复

修复mysql的url解析后密码有特殊字符时无法连接的问题

## 接口优化

参数`WithQueryTimeout`改为`WithQueryTimeoutMS`以明确单位为ms

## 新增接口

新增参数`WithLogger`用于为请求添加log用于debug

## 更新依赖

`github.com/uptrace/bun`更新至1.0.8

# v0.0.4

## bug修复

修复了mysql的url无法使用的问题

# v0.0.3

## bug修复

修正了gomod设置之前的版本都没法用

# v0.0.2

新增对`WithDiscardUnknownColumns`的支持

# v0.0.1

项目创
