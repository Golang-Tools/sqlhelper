#!/usr/bin/env bash
#
# 本地校验脚本:不依赖 GitHub Actions,也不依赖任何外部工具,只用 go 工具链。
#
# 用法:
#   ./scripts/check.sh                          # 格式化 + tidy + build + vet + test -race
#   SQLHELPER_TEST_SQLITE_URL=... \
#   SQLHELPER_TEST_MYSQL_URL=... ./scripts/check.sh   # 额外跑真实数据库集成测试
#
# 可选环境变量:
#   SQLHELPER_TEST_*_URL     设置后执行对应数据库的集成测试
#   SQLHELPER_STRICT_VULN=1  安装了 govulncheck 时,发现漏洞即判定失败(默认只报告)
#
# 说明:
#   - 仓库是多模块结构,脚本会逐个模块执行检查
#   - 真实数据库集成测试在 driver/all 中,只有配置了 SQLHELPER_TEST_*_URL 才会执行
#   - 漏洞扫描是可选步骤:仅当本机 PATH 中存在 govulncheck 时执行
#   - 任一环节失败即退出并返回非零状态,可直接用于本地或第三方 CI(hook/定时任务)

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MODULES=(. driver/postgres driver/mysql driver/sqlserver driver/sqlite driver/all example)

for module in "${MODULES[@]}"; do
	printf '===== %s =====\n' "$module"
	cd "$ROOT/$module"

	unformatted=$(gofmt -l .)
	if [ -n "$unformatted" ]; then
		printf '以下文件未通过 gofmt:\n%s\n' "$unformatted" >&2
		exit 1
	fi

	go mod tidy
	if ! git diff --quiet -- go.mod go.sum; then
		printf 'go.mod/go.sum 不是 tidy 状态,差异如下:\n' >&2
		git --no-pager diff -- go.mod go.sum >&2
		exit 1
	fi

	go build ./...
	go vet ./...
	go test -race -count=1 -timeout 180s ./...
done

# 依赖瘦身校验:核心模块与各驱动模块不得混入其它后端的依赖
printf '===== 依赖瘦身校验 =====\n'
check_no_deps() {
	local module="$1"
	shift
	cd "$ROOT/$module"
	local deps
	deps=$(go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./...)
	local forbidden=""
	local pattern
	for pattern in "$@"; do
		if printf '%s\n' "$deps" | grep -qF "$pattern"; then
			forbidden="$forbidden $pattern"
		fi
	done
	if [ -n "$forbidden" ]; then
		printf '%s 中出现了其它后端的依赖:%s\n' "$module" "$forbidden" >&2
		exit 1
	fi
	printf -- '--- %s 无其它后端依赖 ---\n' "$module"
}
check_no_deps . 'go-sql-driver/mysql' 'go-mssqldb' 'modernc.org/sqlite' 'github.com/jackc/pgx'
check_no_deps driver/postgres 'go-sql-driver/mysql' 'go-mssqldb' 'modernc.org/sqlite'
check_no_deps driver/mysql 'go-mssqldb' 'modernc.org/sqlite' 'github.com/jackc/pgx'
check_no_deps driver/sqlserver 'go-sql-driver/mysql' 'modernc.org/sqlite' 'github.com/jackc/pgx'
check_no_deps driver/sqlite 'go-sql-driver/mysql' 'go-mssqldb' 'github.com/jackc/pgx'

if [ -n "${SQLHELPER_TEST_SQLITE_URL:-}${SQLHELPER_TEST_MYSQL_URL:-}${SQLHELPER_TEST_POSTGRES_URL:-}${SQLHELPER_TEST_SQLSERVER_URL:-}" ]; then
	printf '===== 真实数据库集成测试 (driver/all) =====\n'
	cd "$ROOT/driver/all"
	go test -race -count=1 -timeout 300s -v -run TestIntegration ./...
fi

# 漏洞扫描(可选):本机安装了 govulncheck 才会执行
#   go install golang.org/x/vuln/cmd/govulncheck@latest
# 默认只报告不失败;标准库漏洞通常升级 Go 工具链即可修复
if command -v govulncheck >/dev/null 2>&1; then
	printf '===== 漏洞扫描 (govulncheck) =====\n'
	vuln_found=0
	for module in "${MODULES[@]}"; do
		printf -- '--- %s ---\n' "$module"
		cd "$ROOT/$module"
		if ! govulncheck ./...; then
			vuln_found=1
		fi
	done
	if [ "$vuln_found" -ne 0 ]; then
		if [ "${SQLHELPER_STRICT_VULN:-0}" = "1" ]; then
			printf '发现漏洞,且已设置 SQLHELPER_STRICT_VULN=1,校验失败\n' >&2
			exit 1
		fi
		printf '注意: 发现漏洞(标准库漏洞升级 Go 工具链即可修复);默认不阻断,可设置 SQLHELPER_STRICT_VULN=1 使其失败\n' >&2
	fi
else
	printf '提示: 未安装 govulncheck,已跳过漏洞扫描(安装: go install golang.org/x/vuln/cmd/govulncheck@latest)\n'
fi

printf '全部校验通过\n'
