package postgres

import (
	"testing"

	"github.com/Golang-Tools/sqlhelper/v4/bunproxy"
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

func TestNewDB(t *testing.T) {
	//pgdriver 为惰性连接,这里只验证连接串能被接受
	cli, err := bunproxy.NewDB("postgres://user:pwd@127.0.0.1:5432/db?sslmode=disable", nil)
	if err != nil {
		t.Fatalf("构造连接失败: %v", err)
	}
	defer func() {
		_ = cli.Close()
	}()
}
