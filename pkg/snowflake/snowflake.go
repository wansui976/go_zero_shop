package snowflake

import (
	"errors"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
)

var globalNode *snowflake.Node

func Init(nodeID int64) error {
	if nodeID < 0 || nodeID > 1023 {
		return errors.New("NodeID 应该在 0-1023")
	}
	if globalNode != nil {
		return errors.New("已经初始化过了")
	}
	var err error
	globalNode, err = snowflake.NewNode(nodeID)
	return err
}

func GenIDInt() (int64, error) {
	if globalNode == nil {
		return 0, errors.New("snowflake not initialized")
	}
	return globalNode.Generate().Int64(), nil
}

func GenIDString() (string, error) {
	if globalNode == nil {
		return "", errors.New("snowflake not initialized")
	}
	return globalNode.Generate().String(), nil
}

// Snowflake ID生成器
type SnowflakeGenerator struct {
	mu        sync.Mutex
	epoch     int64 // 起始时间戳：2024-01-01
	timestamp int64 // 上一次生成ID的毫秒时间
	workerId  int64 // 机器ID (0-31)
	sequence  int64 // 序列号 (0-4095)
}

func (s *SnowflakeGenerator) NextID() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if now == s.timestamp {
		s.sequence = (s.sequence + 1) & 4095
		if s.sequence == 0 {
			// 序列号用尽，等待下一毫秒
			for now <= s.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}

	s.timestamp = now

	// 64位组成：
	// 1位符号 + 41位时间戳 + 5位机器ID + 5位数据中心ID + 12位序列号
	id := ((now - s.epoch) << 22) |
		(s.workerId << 12) |
		s.sequence

	return id
}
