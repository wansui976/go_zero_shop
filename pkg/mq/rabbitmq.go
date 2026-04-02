package mq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

type RabbitMQConfig struct {
	URL string // MQ 连接地址（格式：amqp://user:pass@host:port/vhost）
	//ConnTimeout  time.Duration // 连接超时（默认推荐 5s）
	Heartbeat    time.Duration // 心跳间隔（默认推荐 60s，保持连接活跃）
	PoolSize     int           // 信道池大小（默认推荐 10，根据并发量调整）
	MaxReconnect int           // 最大重连次数（默认推荐 10，避免无限重试）
	ReconnectInt time.Duration // 重连间隔（默认推荐 3s）
}

type RabbitMQ struct {
	url          string
	conn         *amqp.Connection
	pool         chan *amqp.Channel // 信道池（解决线程安全问题）
	poolSize     int
	maxReconnect int
	reconnectInt time.Duration
	lock         sync.Mutex
	isClosed     bool
}

// 信道池
type channelPool struct {
	pool chan *amqp.Channel
	r    *RabbitMQ
}

// 初始化信道池
func (r *RabbitMQ) initChannelPool() error {
	r.pool = make(chan *amqp.Channel, r.poolSize)

	for i := 0; i < r.poolSize; i++ {
		ch, err := r.createChannel()
		if err != nil {
			// 部分信道创建失败，关闭已创建的信道和池
			close(r.pool)
			for ch := range r.pool {
				_ = ch.Close()
			}
			return fmt.Errorf("创建第 %d 个信道失败: %w", i+1, err)
		}
		r.pool <- ch
	}

	logx.Infof("信道池初始化完成，池大小: %d", r.poolSize)
	return nil
}

// createChannel 创建单个信道（加锁保护，避免并发创建冲突）
func (r *RabbitMQ) createChannel() (*amqp.Channel, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.isClosed || r.conn == nil || r.conn.IsClosed() {
		return nil, errors.New("连接已关闭，无法创建信道")
	}

	ch, err := r.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("创建信道失败: %w", err)
	}

	return ch, nil
}

func NewRabbitMQ(cfg RabbitMQConfig) (*RabbitMQ, error) {
	// 配置默认值（简化使用）
	// if cfg.ConnTimeout == 0 {
	// 	cfg.ConnTimeout = 5 * time.Second
	// }
	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = 60 * time.Second
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = 10
	}
	if cfg.MaxReconnect == 0 {
		cfg.MaxReconnect = 10
	}
	if cfg.ReconnectInt == 0 {
		cfg.ReconnectInt = 3 * time.Second
	}

	r := &RabbitMQ{
		url:          cfg.URL,
		poolSize:     cfg.PoolSize,
		maxReconnect: cfg.MaxReconnect,
		reconnectInt: cfg.ReconnectInt,
	}

	// 初始化连接
	if err := r.connect(cfg); err != nil {
		return nil, fmt.Errorf("初始化连接失败: %w", err)
	}

	// 初始化信道池
	if err := r.initChannelPool(); err != nil {
		_ = r.Close() // 信道池初始化失败，关闭连接
		return nil, fmt.Errorf("初始化信道池失败: %w", err)
	}

	// 启动自动重连协程
	go r.autoReconnect(cfg)

	logx.Info("RabbitMQ 客户端初始化成功")
	return r, nil
}

// connect 建立 MQ 连接（支持超时、心跳配置）
func (r *RabbitMQ) connect(cfg RabbitMQConfig) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.isClosed {
		return errors.New("客户端已关闭，无法建立连接")
	}

	// 使用带配置的连接方式（指定超时、心跳）
	conn, err := amqp.DialConfig(r.url, amqp.Config{
		//DialTimeout: cfg.ConnTimeout,
		Heartbeat: cfg.Heartbeat,
		Locale:    "en_US", // 固定 Locale，避免兼容性问题
	})
	if err != nil {
		return fmt.Errorf(" Dial MQ 失败: %w", err)
	}

	r.conn = conn
	return nil
}

// autoReconnect 自动重连逻辑（连接断开时触发，支持重试次数限制）
func (r *RabbitMQ) autoReconnect(cfg RabbitMQConfig) {
	reconnectCount := 0
	closeChan := make(chan *amqp.Error)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	// 初始监听连接关闭事件
	r.lock.Lock()
	if !r.isClosed && r.conn != nil {
		closeChan = r.conn.NotifyClose(make(chan *amqp.Error))
	}
	r.lock.Unlock()

	for {
		select {
		case err := <-closeChan:
			r.lock.Lock()
			if r.isClosed {
				r.lock.Unlock()
				logx.Info("自动重连协程退出（客户端已关闭）")
				return
			}
			r.lock.Unlock()

			if err != nil {
				logx.Errorf("MQ 连接断开: %v，开始重连（剩余次数: %d）", err, r.maxReconnect-reconnectCount)
			}

			// 重试重连
			for reconnectCount < r.maxReconnect {
				time.Sleep(r.reconnectInt)

				if err := r.connect(cfg); err != nil {
					reconnectCount++
					logx.Errorf("第 %d 次重连失败: %v", reconnectCount, err)
					continue
				}

				// 重连成功后，重建信道池并重新监听连接
				if err := r.initChannelPool(); err != nil {
					reconnectCount++
					logx.Errorf("重连后初始化信道池失败: %v", err)
					continue
				}

				r.lock.Lock()
				closeChan = r.conn.NotifyClose(make(chan *amqp.Error))
				r.lock.Unlock()

				logx.Info("MQ 重连成功")
				reconnectCount = 0 // 重置重试次数
				break
			}

			// 重连次数耗尽
			if reconnectCount >= r.maxReconnect {
				logx.Errorf("重连失败次数达到上限（%d 次），停止重连", r.maxReconnect)

				return
			}

		case <-ticker.C:
			// 每分钟检查一次连接状态（兜底监控）
			r.lock.Lock()
			connected := !r.isClosed && r.conn != nil && !r.conn.IsClosed()
			r.lock.Unlock()

			if !connected {
				logx.Errorf("MQ 连接状态异常，触发重连")
				closeChan <- &amqp.Error{Code: amqp.ConnectionForced, Reason: "连接状态异常"}
			}
		}
	}
}

// GetChannel 从信道池获取信道（线程安全，自动处理无效信道）
func (r *RabbitMQ) GetChannel() (*amqp.Channel, error) {
	r.lock.Lock()
	if r.isClosed {
		r.lock.Unlock()
		return nil, errors.New("客户端已关闭，无法获取信道")
	}
	r.lock.Unlock()

	select {
	case ch := <-r.pool:
		// 检查信道是否有效，无效则重新创建
		if ch.IsClosed() {
			logx.Info("获取到无效信道，重新创建")
			return r.createChannel()
		}
		return ch, nil

	default:
		// 信道池无空闲时，临时创建新信道（超出池大小，使用后自动关闭）
		logx.Debug("信道池无空闲，临时创建信道")
		return r.createChannel()
	}
}

// ReturnChannel 将信道归还到池（线程安全，自动过滤无效信道）
func (r *RabbitMQ) ReturnChannel(ch *amqp.Channel) {
	if ch == nil || ch.IsClosed() {
		logx.Info("跳过归还无效信道")
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if r.isClosed || len(r.pool) >= r.poolSize {
		// 客户端已关闭或池已满，直接关闭临时信道
		_ = ch.Close()
		return
	}

	select {
	case r.pool <- ch:
	default:
		// 池满时关闭信道（避免阻塞）
		_ = ch.Close()
	}
}

// DeclareQueue 声明队列（封装底层方法，降低使用成本）
func (r *RabbitMQ) DeclareQueue(queueName string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error {
	ch, err := r.GetChannel()
	if err != nil {
		return fmt.Errorf("获取信道失败: %w", err)
	}
	defer r.ReturnChannel(ch)

	_, err = ch.QueueDeclare(
		queueName,  // 队列名
		durable,    // 持久化（true=重启后队列不丢失）
		autoDelete, // 自动删除（无消费者时自动删除）
		exclusive,  // 排他性（仅当前连接可用）
		noWait,     // 不等待响应
		args,       // 额外参数（如 TTL、死信队列）
	)
	if err != nil {
		return fmt.Errorf("声明队列失败: %w", err)
	}

	logx.Infof("队列声明成功: %s", queueName)
	return nil
}

// BindQueue 绑定队列到交换机（支持路由键匹配）
func (r *RabbitMQ) BindQueue(queueName, exchange, routingKey string, noWait bool, args amqp.Table) error {
	ch, err := r.GetChannel()
	if err != nil {
		return fmt.Errorf("获取信道失败: %w", err)
	}
	defer r.ReturnChannel(ch)

	err = ch.QueueBind(
		queueName,  // 队列名
		routingKey, // 路由键（匹配规则）
		exchange,   // 交换机名
		noWait,     // 不等待响应
		args,       // 额外参数
	)
	if err != nil {
		return fmt.Errorf("绑定队列失败: %w", err)
	}

	logx.Infof("队列 %s 绑定交换机 %s 成功（路由键: %s）", queueName, exchange, routingKey)
	return nil
}

// Publish 发布消息（支持交换机、路由键、消息属性配置）
func (r *RabbitMQ) Publish(exchange, routingKey string, mandatory, immediate bool, msg amqp.Publishing) error {
	ch, err := r.GetChannel()
	if err != nil {
		return fmt.Errorf("获取信道失败: %w", err)
	}
	defer r.ReturnChannel(ch)

	// 消息发布超时控制（避免长期阻塞）
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = ch.PublishWithContext(
		ctx,
		exchange,   // 交换机名（空字符串使用默认交换机）
		routingKey, // 路由键
		mandatory,  // 强制路由（无匹配队列时返回消息）
		immediate,  // 立即投递（无消费者时返回消息）
		msg,        // 消息体（包含内容、属性）
	)
	if err != nil {
		return fmt.Errorf("发布消息失败: %w", err)
	}

	logx.Debugf("消息发布成功（交换机: %s，路由键: %s）", exchange, routingKey)
	return nil
}

// Consume 消费消息（返回消息通道，支持自动确认/手动确认）
func (r *RabbitMQ) Consume(queueName, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	// 消费消息的信道单独创建，不归还池（避免被其他协程复用导致冲突）
	ch, err := r.createChannel()
	if err != nil {
		return nil, fmt.Errorf("创建消费信道失败: %w", err)
	}

	// 声明队列（确保消费前队列已存在）
	if err := r.DeclareQueue(queueName, true, false, false, false, nil); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("消费前声明队列失败: %w", err)
	}

	deliveryChan, err := ch.Consume(
		queueName, // 队列名
		consumer,  // 消费者标签（用于标识消费者）
		autoAck,   // 自动确认（true=消费后自动确认，false=手动确认）
		exclusive, // 排他性消费者
		noLocal,   // 不接收本连接发布的消息
		noWait,    // 不等待响应
		args,      // 额外参数（如 QoS 配置）
	)
	if err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("启动消费失败: %w", err)
	}

	logx.Infof("消费者启动成功（队列: %s，消费者标签: %s）", queueName, consumer)
	return deliveryChan, nil
}

// Qos 设置消费限流（避免消费者过载）
func (r *RabbitMQ) Qos(prefetchCount, prefetchSize int, global bool) error {
	ch, err := r.GetChannel()
	if err != nil {
		return fmt.Errorf("获取信道失败: %w", err)
	}
	defer r.ReturnChannel(ch)

	err = ch.Qos(
		prefetchCount, // 每次预取消息数（限流核心参数）
		prefetchSize,  // 每次预取消息大小（0=无限制）
		global,        // 全局生效（true=所有信道，false=当前信道）
	)
	if err != nil {
		return fmt.Errorf("设置 QoS 失败: %w", err)
	}

	logx.Infof("QoS 配置成功（预取消息数: %d，全局生效: %t）", prefetchCount, global)
	return nil
}

// IsConnected 查询连接状态（用于监控/健康检查）
func (r *RabbitMQ) IsConnected() bool {
	r.lock.Lock()
	defer r.lock.Unlock()

	return !r.isClosed && r.conn != nil && !r.conn.IsClosed()
}

// Close 优雅关闭客户端（释放连接、信道池、协程资源）
func (r *RabbitMQ) Close() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.isClosed {
		logx.Error("RabbitMQ 客户端已关闭，无需重复操作")
		return nil
	}

	// 标记为已关闭（阻止重连和新操作）
	r.isClosed = true
	logx.Info("开始关闭 RabbitMQ 客户端...")

	// 1. 关闭信道池（等待所有信道归还）
	if r.pool != nil {
		close(r.pool)
		for ch := range r.pool {
			if !ch.IsClosed() {
				_ = ch.Close()
			}
		}
		logx.Info("信道池关闭完成")
	}

	// 2. 关闭连接（不再固定等待 5 秒）
	if r.conn != nil && !r.conn.IsClosed() {
		if err := r.conn.Close(); err != nil {
			logx.Errorf("关闭连接失败: %v", err)
			return fmt.Errorf("关闭连接失败: %w", err)
		}
		logx.Info("连接关闭完成")
	}

	logx.Info("RabbitMQ 客户端关闭成功")
	return nil
}
