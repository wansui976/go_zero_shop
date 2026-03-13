package idempotent

import (
	"context"
	//amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"time"
)

type Idempotent struct {
	rdb *redis.Client
}

func NewIdempotent(rdb *redis.Client) *Idempotent {
	return &Idempotent{rdb: rdb}
}

// CheckAndSet 检查并设置（原子操作）
func (i *Idempotent) CheckAndSet(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	result, err := i.rdb.SetNX(ctx, "idempotent:"+key, "1", ttl).Result()
	return result, err
}

/*实现了一个非常经典且实用的 “先占位，后执行” 幂等模式。它利用了 Redis 的原子操作 SETNX（在 Go-Redis 中封装为 SetNX）来确保分布式环境下的唯一性。
这对于处理消息队列（如 RabbitMQ/RocketMQ）中的重复消费问题非常有效。
这个实现的工作流程如下：
原子竞争：当消息到达时，多个消费者（或重试的消息）尝试执行 SETNX。
胜出者：只有第一个成功在 Redis 中写入 idempotent:messageID 的请求会得到 true。
拦截：后续所有携带相同 messageID 的请求都会得到 false，从而被你的 if !ok 逻辑直接拦截并丢弃。
过期保护：ttl（24小时）确保了即便业务处理完后没有清理 key，也不会永久占用 Redis 内存。
*/

// func handleMessage(msg amqp.Delivery) error {
// 	messageID := msg.MessageId
// 	ctx := context.Background()
// 	// 1. 幂等性占位
// 	ok, err := idempotent.CheckAndSet(ctx, messageID, 24*time.Hour)
// 	if err != nil {
// 		return err // Redis 故障，让消息重回队列
// 	}
// 	if !ok {
// 		// 重复消息，直接确认（Ack）掉，不再处理
// 		return nil
// 	}

// 	// 2. 执行业务处理
// 	err = processBusiness(msg)
// 	if err != nil {
// 		// 【关键改进】业务处理失败，删除占位 Key，允许后续重试
// 		// 也可以使用一个特定的失败处理函数
// 		idempotent.Delete(ctx, messageID)
// 		return err // 返回错误，消息队列会根据配置进行重试
// 	}

// 	return nil
// }

// 在 Idempotent 结构体中添加 Delete 方法
func (i *Idempotent) Delete(ctx context.Context, key string) error {
	return i.rdb.Del(ctx, "idempotent:"+key).Err()
}
