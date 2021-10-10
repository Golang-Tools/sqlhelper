# v0.0.7

## 接口优化

+ 增加参数`WithInstance(cli bun.IDB)`可以直接设置连接对象
+ 不再保存`Init`的参数

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

项目创建