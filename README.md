# sqlhelper

`uptrace/bun`的代理对象,用于解决对pg,mysql和sqlite3的连接问题

本项目使用bun而不是xorm或者gorm这些老牌的项目主要是因为:

1. bun对postgresql有更好的支持
2. 使用标准库`database/sql`的接口定义
3. 文档更好些