package logic

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/seckill/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/seckill/rpc/seckill"
	"github.com/wansui976/go_zero_shop/pkg/bacher"
	//"github.com/zeromicro/go-zero/core/collection"
	"github.com/zeromicro/go-zero/core/limit"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SeckillOrderLogic struct {
	ctx     context.Context
	svcCtx  *svc.ServiceContext
	limiter *limit.PeriodLimit //基于 Redis 的分布式限流组件
	batcher *bacher.Batcher
	logx.Logger
}

type KafkaData struct {
	Uid int64 `json:"uid"`
	Pid int64 `json:"pid"`
}

const (
	limitPeriod       = 10               //控制 单个用户 10 秒内最多 1 次请求
	limitQuota        = 1                //周期内请求配额（1 次）	防止单个用户高频刷单
	seckillUserPrefix = "seckill#u#"     //edis 限流键前缀（seckill#u#）	区分不同业务的限流键，避免冲突
	localCacheExpire  = time.Second * 60 //本地缓存过期时间（60 秒）	暂存热点数据（当前未实际使用，预留）

	batcherSize     = 100         //批量聚合阈值	累计 100 条请求触发一次批量投递
	batcherBuffer   = 100         //批量通道缓冲区	应对流量峰值，避免批量处理阻塞请求
	batcherWorker   = 10          //批量处理协程数	并行处理不同分片的批量消息
	batcherInterval = time.Second //量时间触发间隔	即使不足 100 条，1 秒后也触发投递

	// 库存缓存键前缀
	seckillStockPrefix = "seckill#stock#"
	// 库存缓存过期时间（30秒，配合商品服务的主动更新）
	stockCacheExpire = time.Second * 30
)

func NewSeckillOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SeckillOrderLogic {

	s := &SeckillOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
		//localCache: localCache,
		limiter: limit.NewPeriodLimit(limitPeriod, limitQuota, svcCtx.BizRedis, seckillUserPrefix),
	}

	// 3. 初始化批量消息组件（batcher）
	b := bacher.New(
		bacher.WithSize(batcherSize),         // 100条触发批量
		bacher.WithBuffer(batcherBuffer),     // 100条缓冲区
		bacher.WithWorker(batcherWorker),     // 10个工作协程
		bacher.WithInterval(batcherInterval), // 1秒时间触发
	)

	//配置分片规则：按商品ID（Pid）分片，确保同一商品的请求进入同一批量
	b.Sharding = func(key string) int {
		pid, _ := strconv.ParseInt(key, 10, 64)
		//先对batcherWorker（int）转int64取模，再转int（确保计算不溢出）
		return int(pid % int64(batcherWorker))
	}

	//将 100 条左右的下单请求聚合成一条 Kafka 消息，减少 Kafka I/O 压力。
	// 3.2 配置批量处理逻辑：将聚合后的请求转为JSON，通过Kafka异步投递
	b.Do = func(ctx context.Context, val map[string][]interface{}) {
		var msgs []*KafkaData
		// 1. 遍历批量数据，转换为Kafka消息格式
		for _, vs := range val { // val是按key（商品ID）聚合的请求列表
			for _, v := range vs {
				msgs = append(msgs, v.(*KafkaData)) // v是单个下单请求（Uid+Pid）
			}
		}
		// 2. 将消息列表序列化为JSON字符串
		kd, err := json.Marshal(msgs)
		if err != nil {
			logx.Errorf("Batcher.Do json.Marshal msgs: %v error: %v", msgs, err)
			return
		}
		// 3. 投递到Kafka（异步处理，不阻塞当前批量逻辑）
		if err = s.svcCtx.KafkaPusher.Push(ctx, string(kd)); err != nil {
			logx.Errorf("KafkaPusher.Push kd: %s error: %v", string(kd), err)
		}
	}

	// 4. 启动批量组件：启动10个工作协程，开始监听批量通道
	b.Start()
	go func() {
		<-ctx.Done() // 监听上下文取消（如服务停止、超时）
		logx.Info("Batcher starting graceful close")
		b.Close() // 关闭batcher，确保未处理消息被消费
		logx.Info("Batcher closed successfully")
	}()

	// 5. 组装并返回逻辑实例
	return s
}

func (l *SeckillOrderLogic) SeckillOrder(in *seckill.SeckillOrderRequest) (*seckill.SeckillOrderResponse, error) {
	//步骤1：限流校验（防刷、防高频）
	// 生成用户限流键（如 "seckill#u#123"，123是用户ID）
	userIdStr := strconv.FormatInt(in.UserId, 10)
	// 尝试获取限流配额：返回 limit.Allowed（允许）或 limit.OverQuota（超限）
	code, _ := l.limiter.Take(userIdStr)
	if code == limit.OverQuota {
		// 限流触发：返回gRPC错误（前端提示请求过于频繁）
		return nil, status.Errorf(codes.OutOfRange, "Number of requests exceeded the limit")
	}

	//步骤2：库存预校验（快速拒绝无效请求）
	// 调用商品RPC，获取商品当前库存（预校验，非最终扣减）
	p, err := l.svcCtx.ProductRPC.Product(l.ctx, &product.ProductItemRequest{
		ProductId: in.ProductId,
	})
	if err != nil {
		return nil, err
	}
	if p.Stock <= 0 {
		//库存不足
		return nil, status.Errorf(codes.OutOfRange, "Insufficient stock")
	}

	//	请求入批量队列
	KafkaData := &KafkaData{
		Uid: in.UserId,
		Pid: in.ProductId,
	}
	// 将消息添加到批量组件（按商品ID分片，非阻塞，缓冲区满时返回错误）
	if err = l.batcher.Add(
		strconv.FormatInt(in.ProductId, 10),
		KafkaData,
	); err != nil {
		// 批量入队失败（如缓冲区满）：记录错误，但不返回给用户
		logx.Errorf("l.batcher.Add uid: %d pid: %d error: %v", in.UserId, in.ProductId, err)
	}

	//返回成功响应
	return &seckill.SeckillOrderResponse{}, nil
}
