package bunproxy_test

import (
	"fmt"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
)

func ExampleNew() {
	//代理对象在初始化之前就可以创建并持有,业务代码无需等待数据库连接
	proxy := bunproxy.New()
	fmt.Println("新创建的代理是否可用:", proxy.IsOk())

	// Output:
	// 新创建的代理是否可用: false
}

func ExampleRedactDSN() {
	fmt.Println(bunproxy.RedactDSN("postgres://user:secret@localhost:5432/db"))
	fmt.Println(bunproxy.RedactDSN("sqlite://:memory:"))

	// Output:
	// postgres://user:REDACTED@localhost:5432/db
	// sqlite://:memory:
}

func ExampleSanitizeSQL() {
	fmt.Println(bunproxy.SanitizeSQL("SELECT * FROM users WHERE name = 'alice' AND age > 18"))
	fmt.Println(bunproxy.SanitizeSQL("SELECT * FROM users WHERE id = $1"))

	// Output:
	// SELECT * FROM users WHERE name = '?' AND age > ?
	// SELECT * FROM users WHERE id = $1
}
