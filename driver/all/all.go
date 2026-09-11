// Package all 一次性引入 bunproxy 的全部官方驱动。
//
// 适合"同一个二进制需要在多个后端之间切换"的场景(停机切配置):
//
//	import (
//		"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
//		_ "github.com/Golang-Tools/sqlhelper/driver/all/v4"
//	)
//
// 只需要单一后端的项目请按需导入对应驱动,以获得更小的依赖:
//
//	import _ "github.com/Golang-Tools/sqlhelper/driver/postgres/v4"
package all

import (
	_ "github.com/Golang-Tools/sqlhelper/driver/mysql/v4"
	_ "github.com/Golang-Tools/sqlhelper/driver/postgres/v4"
	_ "github.com/Golang-Tools/sqlhelper/driver/sqlite/v4"
	_ "github.com/Golang-Tools/sqlhelper/driver/sqlserver/v4"
)
