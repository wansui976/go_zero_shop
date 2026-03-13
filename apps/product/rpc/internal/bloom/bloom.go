package bloom

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"sync"
)

// 简单的内存型布隆过滤器实现
type Bloom struct {
	m     uint        // 位数组大小
	k     uint        // 哈希函数个数
	bits  []uint64    // 位数组
	mutex sync.RWMutex // 读写锁，保证并发安全
}

// New 创建适用于 n 个元素、假阳率为 p 的布隆过滤器
// n: 预计插入元素个数; p: 期望的假阳率(如 0.01 表示 1%)
func New(n uint, p float64) *Bloom {
	m := optimalM(n, p)           // 计算所需位数
	k := optimalK(m, n)           // 计算最佳哈希函数个数
	words := (m + 63) / 64        // 64 位为一个单位
	return &Bloom{
		m:    m,
		k:    k,
		bits: make([]uint64, words),
	}
}

// optimalM 返回所需的布隆位数
func optimalM(n uint, p float64) uint {
	m := -1 * float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)
	return uint(math.Ceil(m))
}

// optimalK 返回最优哈希函数个数
func optimalK(m, n uint) uint {
	k := (float64(m) / float64(n)) * math.Ln2
	return uint(math.Ceil(k))
}

// Add 将字节切片 data 加入 Bloom 过滤器
func (b *Bloom) Add(data []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	locations := b.locations(data)
	for _, idx := range locations {
		b.bits[idx/64] |= 1 << (idx % 64)
	}
}

// Test 检查 data 是否可能存在于 Bloom 中（可能有误判）
func (b *Bloom) Test(data []byte) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	locations := b.locations(data)
	for _, idx := range locations {
		if (b.bits[idx/64] & (1 << (idx % 64))) == 0 {
			return false // 某位未被置位，一定不存在
		}
	}
	return true // 所有位置都被置位，可能存在
}

// locations 用于获得 data 在布隆数组上散列到的多个位置
func (b *Bloom) locations(data []byte) []uint {
	locs := make([]uint, 0, b.k)
	// 使用 FNV-1a 基础 hash 做种
	h := fnv.New64a()
	h.Write(data)
	sum := h.Sum64()
	for i := uint(0); i < b.k; i++ {
		// 不同哈希函数通过种子扰动得到
		x := sum + uint64(i*0x9e3779b97f4a7c15)
		idx := uint(x % uint64(b.m))
		locs = append(locs, idx)
	}
	return locs
}

// AddInt64 辅助函数，方便加入 int64 型 id
func (b *Bloom) AddInt64(x int64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(x))
	b.Add(buf[:])
}

// TestInt64 辅助函数，判断 int64 是否被加入过
func (b *Bloom) TestInt64(x int64) bool {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(x))
	return b.Test(buf[:])
}
