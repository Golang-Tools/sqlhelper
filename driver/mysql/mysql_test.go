package mysql

import (
	"testing"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestDriverRegistered(t *testing.T) {
	d, ok := bunproxy.FindDriver(Scheme)
	if !ok || d == nil {
		t.Fatalf("%s 驱动应已注册", Scheme)
	}
	if d.Scheme() != Scheme {
		t.Fatalf("scheme不符, 实际: %s", d.Scheme())
	}
	if d.Dialect() == nil {
		t.Fatal("方言不应为nil")
	}
}

// TestBuildDSN 覆盖 URL 到 DSN 的转换,重点是不带库名与密码含特殊字符的场景
func TestBuildDSN(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "标准连接串",
			in:   "mysql://root:root@127.0.0.1:3306/test",
			want: "root:root@tcp(127.0.0.1:3306)/test?",
		},
		{
			name: "带查询参数",
			in:   "mysql://root:root@127.0.0.1:3306/test?charset=utf8mb4",
			want: "root:root@tcp(127.0.0.1:3306)/test?charset=utf8mb4",
		},
		{
			name: "不带库名也要保留斜杠",
			in:   "mysql://root:root@127.0.0.1:3306",
			want: "root:root@tcp(127.0.0.1:3306)/?",
		},
		{
			name: "只有用户名",
			in:   "mysql://root@127.0.0.1:3306/test",
			want: "root@tcp(127.0.0.1:3306)/test?",
		},
		{
			name: "只有密码",
			in:   "mysql://:pwd@127.0.0.1:3306/test",
			want: ":pwd@tcp(127.0.0.1:3306)/test?",
		},
		{
			name: "百分比编码的密码会被还原",
			in:   "mysql://root:p%40ss@127.0.0.1:3306/test",
			want: "root:p@ss@tcp(127.0.0.1:3306)/test?",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := buildDSN(c.in)
			if err != nil {
				t.Fatalf("转换失败: %v", err)
			}
			if got != c.want {
				t.Fatalf("DSN不符\n期望: %s\n实际: %s", c.want, got)
			}
			//生成的 DSN 必须能被驱动自身解析,这是历史 bug 的回归保护
			if _, err := mysqldriver.ParseDSN(got); err != nil {
				t.Fatalf("驱动无法解析生成的DSN: %v", err)
			}
		})
	}
}

func TestBuildDSNInvalidURL(t *testing.T) {
	if _, err := buildDSN("mysql://user:pwd@127.0.0.1:not-a-port/db"); err == nil {
		t.Fatal("端口非法时应返回错误")
	}
}

func TestNewDB(t *testing.T) {
	//go-sql-driver/mysql 会立即解析 DSN,连接串非法时 NewDB 就会报错
	for _, raw := range []string{
		"mysql://root:root@127.0.0.1:3306/test?charset=utf8mb4",
		"mysql://root:root@127.0.0.1:3306",
		"mysql://root:p%40ss@127.0.0.1:3306/test",
	} {
		cli, err := bunproxy.NewDB(raw, nil)
		if err != nil {
			t.Fatalf("构造连接失败, URL: %s, err: %v", raw, err)
		}
		_ = cli.Close()
	}
}
