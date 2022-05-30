# sqlhelper/v2

`uptrace/bun`的代理对象,用于解决对常见关系数据库pg,mysql,sqlserver和sqlite3的连接问题.

v2版本使用了泛型语法,因此只支持go 1.18+的编译器,如果go版本低于1.18请使用v0版本

本项目使用bun而不是xorm或者gorm这些老牌的项目主要是因为:

1. bun对postgresql有更好的支持,原生支持jsonb,array等数据类型
2. 使用标准库`database/sql`的接口定义
3. 文档可读性更好些,虽然是英文
