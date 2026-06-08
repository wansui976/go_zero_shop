// Package envcfg 提供启动时从环境变量覆盖敏感配置的能力。
// 设计目标：将 K8s Secret 注入到容器 env 后，由 Go 代码在加载完 yaml/ConfigMap
// 之后对内存中的 config 结构体进行覆盖，避免将密码、token 等明文落到 ConfigMap/yaml。
package envcfg

import (
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Getenv 返回环境变量的值，若未设置返回 def。
func Getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// 匹配标准 MySQL DSN：user:pass@tcp(host:port)/db?...
// pass 可能为空（user:@tcp(...)）；user 至少 1 个非冒号字符。
var mysqlDSNRe = regexp.MustCompile(`^([^:@]+):([^@]*)@(.*)$`)

// InjectMySQLDSNPassword 用环境变量中的密码覆盖 DSN 中的密码段。
//   - 若 envKey 未设置或为空，直接返回原 DSN。
//   - 仅当 DSN 形如 "user:pass@..." 时生效，否则返回原值。
//   - 密码会被 URL-escape，避免特殊字符破坏 DSN。
func InjectMySQLDSNPassword(dsn, envKey string) string {
	pwd := os.Getenv(envKey)
	if pwd == "" {
		return dsn
	}
	m := mysqlDSNRe.FindStringSubmatch(dsn)
	if len(m) != 4 {
		return dsn
	}
	user := m[1]
	rest := m[3]
	return user + ":" + url.QueryEscape(pwd) + "@" + rest
}

// OverrideRedisConf 用 env 覆盖 Redis 密码（in-place）。
func OverrideRedisConf(c *redis.RedisConf, envKey string) {
	if c == nil {
		return
	}
	if pwd := os.Getenv(envKey); pwd != "" {
		c.Pass = pwd
	}
}

// OverrideCacheConf 用 env 覆盖 CacheRedis 列表的所有节点密码（in-place）。
func OverrideCacheConf(c cache.CacheConf, envKey string) {
	pwd := os.Getenv(envKey)
	if pwd == "" {
		return
	}
	for i := range c {
		c[i].Pass = pwd
	}
}

// OverrideString 若 env 已设置则返回 env 值，否则返回原 cur。
func OverrideString(cur, envKey string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return cur
}

// MustNonEmpty 在生产模式下（PROD_MODE=true）强制要求 env 必须非空，
// 否则 panic，避免悄悄使用 yaml 中的默认值。
func MustNonEmpty(envKey string) {
	if strings.EqualFold(os.Getenv("PROD_MODE"), "true") {
		if v := os.Getenv(envKey); v == "" {
			panic("envcfg: required secret env not set in PROD_MODE: " + envKey)
		}
	}
}
