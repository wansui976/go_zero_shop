package cache

import (
	"context"
	"math/rand"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// SetHashWithRandomExpire 写入哈希并设置带随机抖动的过期时间，防止缓存雪崩。
// baseExpire: 基础过期时间（如 7*24*time.Hour）
// jitterSeconds: 随机抖动上限（秒）
func SetHashWithRandomExpire(ctx context.Context, r *redis.Redis, key string, fields map[string]string, baseExpire time.Duration, jitterSeconds int) error {
	if r == nil {
		return nil
	}
	// 写入哈希
	if err := r.HmsetCtx(ctx, key, fields); err != nil {
		return err
	}
	// 随机抖动
	j := 0
	if jitterSeconds > 0 {
		j = rand.Intn(jitterSeconds)
	}
	totalSeconds := int(baseExpire.Seconds()) + j
	return r.ExpireCtx(ctx, key, totalSeconds)
}

// DeleteWithDelay 延迟双删 - 防止并发下的缓存一致性问题
// 在更新数据库前后各删除一次缓存
// delay: 延迟时间（如 500*time.Millisecond）
func DeleteWithDelay(ctx context.Context, r *redis.Redis, key string, delay time.Duration) {
	if r == nil {
		return
	}
	// 第一次删除（更新数据库前）
	_, _ = r.DelCtx(ctx, key)
	logx.Infof("Cache deleted (first pass): %s", key)

	// 延迟删除（更新数据库后，防止并发读取旧数据后重新写入缓存）
	time.AfterFunc(delay, func() {
		_, _ = r.DelCtx(ctx, key)
		logx.Infof("Cache deleted (second pass): %s", key)
	})
}
