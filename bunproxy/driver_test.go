package bunproxy

import (
	"database/sql"
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
)

// fakeDriver 用于注册表测试的假驱动
type fakeDriver struct{ scheme string }

func (d fakeDriver) Scheme() string { return d.scheme }

func (fakeDriver) Dialect() schema.Dialect { return sqlitedialect.New() }

func (fakeDriver) NewPool(string, *Options) (*sql.DB, error) { return nil, errors.New("unused") }

func TestFindDriver(t *testing.T) {
	//stub_test.go 中注册的 memory 驱动应该可以查到
	d, ok := FindDriver("memory")
	if !ok || d == nil {
		t.Fatal("memory 驱动应已注册")
	}
	if d.Scheme() != "memory" {
		t.Fatalf("驱动的scheme不符, 实际: %s", d.Scheme())
	}
	if _, ok := FindDriver("not-exists"); ok {
		t.Fatal("未注册的scheme不应返回驱动")
	}
}

func TestRegisteredSchemes(t *testing.T) {
	schemes := RegisteredSchemes()
	if !slices.Contains(schemes, "memory") {
		t.Fatalf("已注册列表应包含memory, 实际: %v", schemes)
	}
	if !sort.StringsAreSorted(schemes) {
		t.Fatalf("已注册列表应按字典序排列, 实际: %v", schemes)
	}
	//返回的应是副本,外部修改不应影响注册表
	schemes[0] = "mutated"
	if got := RegisteredSchemes(); got[0] == "mutated" {
		t.Fatal("RegisteredSchemes应返回副本")
	}
}

func TestRegisterDriverDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("重复注册同一个scheme应该panic")
		}
	}()
	//memory 已被 stub 驱动注册
	RegisterDriver(fakeDriver{scheme: "memory"})
}

func TestRegisterDriverEmptySchemePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("注册scheme为空的驱动应该panic")
		}
	}()
	RegisterDriver(fakeDriver{scheme: ""})
}

func TestRegisterDriverNilIsIgnored(t *testing.T) {
	//不应panic
	RegisterDriver(nil)
}

func TestNewDBUnsupportedSchemaMentionsDrivers(t *testing.T) {
	_, err := NewDB("oracle://127.0.0.1:1521/db", nil)
	if !errors.Is(err, ErrUnsupportedSchema) {
		t.Fatalf("应返回ErrUnsupportedSchema, 实际: %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "oracle") {
		t.Fatalf("错误信息应包含具体的scheme, 实际: %s", msg)
	}
	//错误信息应给出可操作的提示,便于排查"漏引导入驱动"
	for _, want := range []string{"已注册的驱动", "driver"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("错误信息应包含 %q 以便排查, 实际: %s", want, msg)
		}
	}
}
