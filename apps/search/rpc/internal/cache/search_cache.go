package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// SearchCacheKey 生成搜索结果缓存 key
func SearchCacheKey(keyword string, pageNum, pageSize int64, sort int32) string {
	return fmt.Sprintf("search:result:%s:%d:%d:%d", keyword, pageNum, pageSize, sort)
}

// GetSearchResult 从 Redis 获取缓存的搜索结果
func GetSearchResult(ctx context.Context, r *redis.Redis, key string) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	data, err := r.GetCtx(ctx, key)
	if err != nil {
		logx.Errorf("Get search cache error: %v", err)
		return nil, err
	}
	if data == "" {
		return nil, nil // go-zero 的 GetCtx 在 key 不存在时返回 "", nil
	}
	return []byte(data), nil
}

// SetSearchResult 将搜索结果缓存到 Redis
func SetSearchResult(ctx context.Context, r *redis.Redis, key string, data []byte, expire time.Duration) error {
	if r == nil {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	if err := r.SetexCtx(ctx, key, string(data), int(expire.Seconds())); err != nil {
		logx.Errorf("Set search cache error: %v", err)
		return err
	}
	return nil
}

// WithCacheSearch 带缓存的搜索执行器
// 如果缓存命中，直接返回；否则执行搜索并缓存结果
func WithCacheSearch(ctx context.Context, r *redis.Redis, cacheKey string, searchFunc func() ([]byte, error), cacheExpire time.Duration) ([]byte, error) {
	// 1. 尝试从缓存获取
	if r != nil {
		cachedData, err := GetSearchResult(ctx, r, cacheKey)
		if err == nil && cachedData != nil {
			logx.Infof("Search cache hit: %s", cacheKey)
			return cachedData, nil
		}
	}

	// 2. 执行搜索（带重试）
	var result []byte
	var lastErr error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		result, lastErr = searchFunc()
		if lastErr == nil {
			break
		}
		logx.Errorf("Search failed, retry %d/%d: %v", i+1, maxRetries, lastErr)
		// 指数退避: 100ms, 200ms, 400ms
		time.Sleep(time.Duration(100*(i+1)*(i+1)) * time.Millisecond)
	}

	if lastErr != nil {
		return nil, lastErr
	}

	// 3. 缓存结果
	if r != nil && len(result) > 0 {
		_ = SetSearchResult(ctx, r, cacheKey, result, cacheExpire)
	}

	return result, nil
}
