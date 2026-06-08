package snowflake

import (
	"hash/fnv"
	"os"
	"regexp"
	"strconv"
)

// ResolveNodeID 推导分布式部署下的 NodeID（0..1023），优先级：
//  1. 环境变量 SNOWFLAKE_NODE_ID（显式指定，最高优先）
//  2. 环境变量 POD_NAME 中的 StatefulSet 序号（如 "user-rpc-3" → 3）
//  3. HOSTNAME 的 FNV 哈希取模（保证多副本之间大概率不冲突）
//  4. fallback（来自 config，默认 1）
//
// 仅用于 single-cluster 内部，集群间复用 ID 仍可能冲突。
func ResolveNodeID(fallback int64) int64 {
	const max = 1024

	if v := os.Getenv("SNOWFLAKE_NODE_ID"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id >= 0 && id < max {
			return id
		}
	}

	if pod := os.Getenv("POD_NAME"); pod != "" {
		if id := ordinalFromStatefulSetName(pod); id >= 0 {
			return int64(id) % max
		}
	}

	if host, err := os.Hostname(); err == nil && host != "" {
		if id := ordinalFromStatefulSetName(host); id >= 0 {
			return int64(id) % max
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(host))
		return int64(h.Sum32() % max)
	}

	if fallback < 0 || fallback >= max {
		return 0
	}
	return fallback
}

var statefulSetOrdinalRe = regexp.MustCompile(`-(\d+)$`)

// "user-rpc-3" -> 3；不匹配返回 -1
func ordinalFromStatefulSetName(name string) int {
	m := statefulSetOrdinalRe.FindStringSubmatch(name)
	if len(m) != 2 {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return n
}
