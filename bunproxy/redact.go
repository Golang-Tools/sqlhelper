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
	//sqlRedactPattern 匹配SQL中需要脱敏的字面量
	//bun的查询构造器会把参数值内联进SQL,因此除了字符串字面量,数字字面量也需要遮蔽
	//注意:必须把postgres的位置占位符($1)放在数字之前,避免占位符被当成数字遮蔽
	sqlRedactPattern = regexp.MustCompile(
		`'(?:[^'\\]|\\.|'')*'` + // 字符串字面量
			`|\$[0-9]+` + // postgres位置占位符,保持原样
			`|0[xX][0-9a-fA-F]+` + // 十六进制字面量
			`|\b[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?\b`, // 数字字面量
	)
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

// SanitizeSQL 脱敏SQL语句,遮蔽字面量并压缩空白,用于日志输出
// 字符串字面量替换为'?',数字字面量替换为?;SQL关键字、标识符(含带数字的列名)与
// 位置占位符($1)保持原样,NULL/TRUE/FALSE等关键字不做替换(不包含敏感信息)
func SanitizeSQL(query string) string {
	if query == "" {
		return ""
	}
	sanitized := sqlRedactPattern.ReplaceAllStringFunc(query, func(literal string) string {
		switch {
		case strings.HasPrefix(literal, "$"):
			//位置占位符保持原样,便于对照日志与SQL
			return literal
		case strings.HasPrefix(literal, "'"):
			return "'?'"
		default:
			return "?"
		}
	})
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
