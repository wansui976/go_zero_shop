package bacher

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/contextx"
)

/*
batcher 包是一套 通用的批量消息聚合框架，
核心能力是将零散、高频的消息请求按 数量阈值 或 时间间隔 聚合为批量，
再通过自定义逻辑统一处理（如批量投递 Kafka、批量写入数据库），
以减少 IO 次数、降低系统开销，适配秒杀、日志收集等高并发场景
*/
var ErrFull = errors.New("channel is full")

type Option interface {
	apply(*options)
}

type options struct {
	size     int           // 批量大小（累计多少条消息触发处理）
	buffer   int           // 每个分片通道的缓冲区大小
	worker   int           // 工作协程数（分片数）
	interval time.Duration // 时间间隔（不足批量大小时，多久触发一次处理）
}

func (o options) check() {
	if o.size <= 0 {
		o.size = 100
	}
	if o.buffer <= 0 {
		o.buffer = 100
	}
	if o.worker <= 0 {
		o.worker = 5
	}
	if o.interval <= 0 {
		o.interval = time.Second
	}
}

type funcOption struct {
	f func(*options)
}

func (fo *funcOption) apply(o *options) {
	fo.f(o)
}

func newOption(f func(*options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

func WithSize(s int) Option {
	return newOption(func(o *options) {
		o.size = s
	})
}

func WithBuffer(b int) Option {
	return newOption(func(o *options) {
		o.buffer = b
	})
}

func WithWorker(w int) Option {
	return newOption(func(o *options) {
		o.worker = w
	})
}

func WithInterval(i time.Duration) Option {
	return newOption(func(o *options) {
		o.interval = i
	})
}

type msg struct {
	key string      // 分片键（如商品ID、用户ID）
	val interface{} // 消息内容（业务自定义类型，如KafkaData）
	ctx context.Context
}

// Batcher 批量聚合核心结构体
type Batcher struct {
	opts     options                                                 // 配置参数（批量大小、协程数等）
	Do       func(ctx context.Context, val map[string][]interface{}) // 批量处理逻辑（业务自定义）
	Sharding func(key string) int                                    // 分片规则（按key分配到不同通道）
	chans    []chan *msg                                             // 分片通道数组（每个协程对应一个通道）
	wait     sync.WaitGroup                                          // 等待组（用于优雅关闭协程）
}

func New(opts ...Option) *Batcher {
	b := &Batcher{}
	for _, opt := range opts {
		opt.apply(&b.opts)
	}
	b.opts.check()
	b.chans = make([]chan *msg, b.opts.worker)
	for i := 0; i < b.opts.worker; i++ {
		b.chans[i] = make(chan *msg, b.opts.buffer)
	}
	return b
}

// Start 启动所有工作协程（必须在Add前调用）
func (b *Batcher) Start() {
	if b.Do == nil {
		log.Fatal("Batcher: Do func is nil")
	}
	if b.Sharding == nil {
		log.Fatal("Batcher: Sharding func is nil")
	}
	b.wait.Add(len(b.chans))
	for i, ch := range b.chans {
		go b.merge(i, ch)
	}
}

// Add 向批量中添加一条消息（业务层调用的核心方法）
func (b *Batcher) Add(key string, val interface{}) error {
	return b.AddWithContext(context.Background(), key, val)
}

// AddWithContext 向批量中添加一条带上下文的消息
func (b *Batcher) AddWithContext(ctx context.Context, key string, val interface{}) error {
	// 1. 计算消息应进入的通道（调用add方法，基于Sharding规则）
	ch, msg := b.add(ctx, key, val)

	// 2. 非阻塞发送消息到通道：若缓冲区满，直接返回ErrFull
	select {
	case ch <- msg: // 通道有空闲，发送消息
	default: // 通道满，返回错误
		return ErrFull
	}

	return nil
}

// add 内部方法：计算分片通道，创建msg实例
func (b *Batcher) add(ctx context.Context, key string, val interface{}) (chan *msg, *msg) {
	// 1. 计算分片索引：Sharding(key) % 协程数（确保分片在[0, worker-1]范围内）
	sharding := b.Sharding(key) % b.opts.worker
	// 2. 获取对应的分片通道
	ch := b.chans[sharding]
	// 3. 创建msg实例（封装key和val）
	if ctx == nil {
		ctx = context.Background()
	}
	msg := &msg{key: key, val: val, ctx: contextx.ValueOnlyFrom(ctx)}

	return ch, msg
}

/*
merge 是 批量聚合的核心，运行在独立协程中，负责：
从通道接收消息；
按key聚合消息（同一key的消息放入同一列表）；
满足 数量阈值 或 时间间隔 时，调用Do处理批量消息。
*/
func (b *Batcher) merge(idx int, ch <-chan *msg) {
	defer b.wait.Done() // 协程退出时，等待组计数-1（优雅关闭）

	// 初始化局部变量
	var (
		msg        *msg              // 从通道接收的单个消息
		count      int               // 已聚合的消息总数
		closed     bool              // 通道是否已关闭（通过接收nil判断）
		lastTicker = true            // 标记是否为首次定时器触发（用于调整定时器间隔）
		interval   = b.opts.interval // 初始时间间隔（后续可能调整）
		batchCtx   = context.Background()
		// vals：聚合后的批量数据，key=分片键，val=同一key的消息列表
		vals = make(map[string][]interface{}, b.opts.size)
	)

	// 关键优化：错开不同协程的定时器触发时间（避免所有协程同时处理批量，导致下游压力突增）
	if idx > 0 {
		// 协程idx的初始间隔 = (idx * 总间隔) / 协程数（如10个协程，总间隔1秒，idx=1则间隔100ms）
		interval = time.Duration(int64(idx) * (int64(b.opts.interval) / int64(b.opts.worker)))
	}

	// 启动定时器（初始间隔为上面计算的错开间隔）
	ticker := time.NewTicker(interval)

	// 无限循环：接收消息或等待定时器
	for {
		select {
		// 分支1：从通道接收消息
		case msg = <-ch:
			// 若接收的是nil，说明通道已关闭（Close方法触发）
			if msg == nil {
				closed = true
				break // 跳出select，准备处理剩余聚合数据
			}

			// 聚合消息：将当前消息添加到对应key的列表中
			if count == 0 && msg.ctx != nil {
				batchCtx = msg.ctx
			}
			count++
			vals[msg.key] = append(vals[msg.key], msg.val)

			// 若聚合数量达到阈值（size），跳出select，触发批量处理
			if count >= b.opts.size {
				break
			}

			// 数量未达阈值，继续接收下一条消息
			continue

		// 分支2：定时器触发（时间间隔到）
		case <-ticker.C:
			// 首次定时器触发后，重置定时器为默认间隔（错开初始时间后，恢复正常间隔）
			if lastTicker {
				ticker.Stop()                            // 停止旧定时器
				ticker = time.NewTicker(b.opts.interval) // 启动新定时器（默认间隔）
				lastTicker = false                       // 标记为非首次
			}
		}

		// 处理聚合数据：若vals非空，调用Do函数批量处理
		if len(vals) > 0 {
			b.Do(batchCtx, vals)                               // 调用业务自定义的批量处理逻辑
			vals = make(map[string][]interface{}, b.opts.size) // 重置vals，准备下一轮聚合
			count = 0                                          // 重置计数
			batchCtx = context.Background()
		}

		// 若通道已关闭，停止定时器并退出协程
		if closed {
			ticker.Stop()
			return
		}
	}
}

func (b *Batcher) Close() {
	for _, ch := range b.chans {
		ch <- nil
	}
	b.wait.Wait()
}
