package bunproxy

import (
	"net/url"
	"regexp"
	"strings"
)

// maxLoggedSQLLength 日志中SQL的最大长度,超出部分会被截断
const maxLoggedSQLLength = 2000

// RedactedPlaceholder 连接串脱敏后替换密码的占位符
// 这里不使用"******"是为了避免url编码后变成"%2A%2A%2A%2A%2A%2A"
const RedactedPlaceholder = "REDACTED"

var (
	//dsnPasswordPattern 匹配scheme://user:password@host形式的连接串
	dsnPasswordPattern = regexp.MustCompile(`://([^:/?#]*):([^@/?#]*)@`)
	//sqlStringLiteralPattern 匹配SQL中的字符串字面量
	sqlStringLiteralPattern = regexp.MustCompile(`'(?:[^'\\]|\\.|'')*'`)
	//sqlWhitespacePattern 匹配SQL中的连续空白字符
	sqlWhitespacePattern = regexp.MustCompile(`\s+`)
)

// RedactDSN 隐藏连接串中的密码等敏感信息,用于日志和错误输出
func RedactDSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return ""
	}
	if U, err := url.Parse(trimmed); err == nil && U.Scheme != "" {
		if _, ok := U.User.Password(); ok {
			U.User = url.UserPassword(U.User.Username(), RedactedPlaceholder)
			return U.String()
		}
		query := U.Query()
		redacted := false
		for _, key := range []string{"password", "passwd", "pwd"} {
			if query.Has(key) {
				query.Set(key, RedactedPlaceholder)
				redacted = true
			}
		}
		if redacted {
			U.RawQuery = query.Encode()
			return U.String()
		}
		return trimmed
	}
	return dsnPasswordPattern.ReplaceAllString(trimmed, "://$1:"+RedactedPlaceholder+"@")
}

// SanitizeSQL 脱敏SQL语句,遮蔽字符串字面量并压缩空白,用于日志输出
func SanitizeSQL(query string) string {
	if query == "" {
		return ""
	}
	sanitized := sqlStringLiteralPattern.ReplaceAllString(query, "'?'")
	sanitized = sqlWhitespacePattern.ReplaceAllString(sanitized, " ")
	sanitized = strings.TrimSpace(sanitized)
	return truncateRunes(sanitized, maxLoggedSQLLength)
}

// truncateRunes 按rune截断字符串,避免截断出现非法utf8
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "...(truncated)"
}
