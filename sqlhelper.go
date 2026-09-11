// Package sqlhelper 是关系型数据库访问的辅助模块。
//
// 本包只用于承载模块声明,不提供任何导出接口。真正的实现位于子包
// github.com/Golang-Tools/sqlhelper/v3/bunproxy,请在代码中直接导入该子包。
//
//	v3 与 v2 的模块路径不同,因此可以同时存在于同一个项目中,便于灰度迁移。
//	迁移说明见仓库根目录的 MIGRATION_v2_to_v3.md。
package sqlhelper
