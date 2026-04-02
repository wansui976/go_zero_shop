# go-zero-shop

基于 **go-zero** 微服务框架构建的电商后端项目，涵盖商品、订单、用户、购物车、搜索、秒杀、支付等核心业务模块，重点实践分布式事务、异步消息、高并发库存控制等电商场景下的关键技术。

---

## 技术栈

| 类别 | 技术选型 |
|------|---------|
| 微服务框架 | [go-zero](https://github.com/zeromicro/go-zero) v1.9 |
| 分布式事务 | [DTM](https://github.com/dtm-labs/dtm)（Saga 模式） |
| 服务注册发现 | etcd |
| 消息队列 | RabbitMQ（业务事件） / Kafka（日志流） |
| 缓存 | Redis（库存预扣、幂等、排行榜） |
| 数据库 | MySQL 8.0 |
| 搜索引擎 | Elasticsearch 9 |
| 异步任务 | [asynq](https://github.com/hibiken/asynq)（库存 DB 同步） |
| 链路追踪 | OpenTelemetry + Jaeger |
| 监控 | Prometheus + Grafana |
| 容器化 | Docker / Kubernetes |
| 语言 | Go 1.24 |

---

## 项目结构

```
go_zero_shop/
├── apps/
│   ├── app/api/        # API 网关（HTTP，端口 8888）
│   ├── user/rpc/       # 用户服务（gRPC，端口 8080）
│   ├── product/rpc/    # 商品服务（gRPC，端口 8081）
│   ├── order/          # 订单服务
│   │   ├── rpc/        # 订单 RPC（端口 8082）
│   │   └── rmq/        # 订单消费者（延迟取消/库存回补/通知）
│   ├── cart/rpc/       # 购物车服务（gRPC，端口 8084）
│   ├── search/rpc/     # 搜索服务（gRPC，端口 8083）
│   ├── pay/            # 支付服务
│   │   ├── rpc/        # 支付 RPC（端口 8085）
│   │   └── rmq/        # 支付回调消费者
│   ├── reply/          # 评论服务
│   │   ├── rpc/        # 评论 RPC（端口 8086）
│   │   ├── admin/      # 评论后台 API
│   │   └── rmq/        # 评论异步处理
│   └── seckill/        # 秒杀服务
│       ├── rpc/        # 秒杀 RPC
│       └── rmq/        # 秒杀消费者
├── pkg/                # 公共包（雪花ID、幂等、MQ、加密等）
├── k8s/                # Kubernetes 部署配置
├── Dockerfile/         # 各服务 Dockerfile
├── docker-compose.yml  # 本地一键启动
├── prometheus.yml      # Prometheus 配置
└── grafana/            # Grafana Dashboard
```

---

## 核心功能

### 下单链路（DTM Saga）

```
客户端 → API 网关 → DTM Saga 事务
                      ├── Step 1: 扣减商品库存（product-rpc/DecrStock）
                      │          补偿: 回补库存（DecrStockRevert）
                      └── Step 2: 创建订单（order-rpc/CreateOrderDTM）
                                 补偿: 取消订单（CreateOrderDTMRevert）
```

- 基于 DTM Saga 保证分布式事务最终一致性
- 订单创建后发布 `order_create_queue` 事件，触发异步通知与延迟取消
- 支持前端 `request_id` 幂等防重，避免网络重试导致重复下单

### 库存控制（双层保护）

| 层 | 机制 | 适用场景 |
|----|------|---------|
| DB 层 | DTM Saga + 乐观锁 | 普通下单 |
| Redis 层 | Lua 原子预扣（`available`/`pre_locked`） | DTM Try 阶段额外预扣 |
| Redis 层 | Lua 原子扣减（`total`/`used`/`frozen`） | 秒杀 / 高并发场景 |

Redis 库存未初始化时自动降级为 DB 层兜底，不影响正常下单。

### 秒杀

- Redis 预热商品库存，Lua 脚本原子扣减防超卖
- 异步队列削峰，消费者写库保证最终一致
- 基于 Bloom Filter 防缓存穿透

### 搜索

- 商品数据同步至 Elasticsearch
- 支持关键词搜索、相关推荐、热度排行

### 延迟取消

- RabbitMQ 死信队列（30 分钟 TTL）实现待支付订单自动取消
- 取消时向库存回补 Topic 发布事件，异步消费者负责还原库存

---

## 快速启动

### 前置依赖

- Docker & Docker Compose
- Go 1.24+

### 一键启动基础设施

```bash
docker-compose up -d
```

启动以下服务：MySQL、Redis、etcd、RabbitMQ、DTM、Elasticsearch、Jaeger、Prometheus、Grafana

### 启动各微服务

```bash
# 用户服务
go run apps/user/rpc/user.go -f apps/user/rpc/etc/user.yaml

# 商品服务
go run apps/product/rpc/product.go -f apps/product/rpc/etc/product.yaml

# 订单服务
go run apps/order/rpc/order.go -f apps/order/rpc/etc/order.yaml

# 购物车服务
go run apps/cart/rpc/cart.go -f apps/cart/rpc/etc/cart.yaml

# 搜索服务
go run apps/search/rpc/search.go -f apps/search/rpc/etc/search.yaml

# 支付服务
go run apps/pay/rpc/pay.go -f apps/pay/rpc/etc/pay.yaml

# API 网关
go run apps/app/api/api.go -f apps/app/api/etc/api-api.yaml
```

### Kubernetes 部署

```bash
chmod +x k8s/deploy.sh && ./k8s/deploy.sh
```

---

## 主要 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/user/login` | 用户登录，返回 JWT |
| GET  | `/user/info` | 获取用户信息 |
| GET  | `/product/list` | 商品列表（分页） |
| GET  | `/product/:productId` | 商品详情 |
| POST | `/order/add` | 创建订单（DTM Saga） |
| GET  | `/order/list` | 订单列表 |
| POST | `/cart/add` | 加入购物车 |
| GET  | `/cart/list` | 购物车列表 |
| GET  | `/home/flashsale` | 限时抢购商品 |

> 除登录外，所有接口需在 Header 携带 `Authorization: Bearer <token>`

---

## 监控与追踪

| 工具 | 地址 | 说明 |
|------|------|------|
| Grafana | http://localhost:3000 | 服务指标看板 |
| Prometheus | http://localhost:9090 | 指标采集 |
| Jaeger | http://localhost:16686 | 分布式链路追踪 |
| RabbitMQ | http://localhost:15672 | 消息队列管理 |

---

## 数据库初始化

各服务 SQL 文件位于对应模块的 `model/` 目录下：

```
apps/user/rpc/model/user.sql
apps/user/rpc/model/user_receive_address.sql
apps/product/rpc/internal/model/product.sql
apps/order/rpc/model/orders.sql
apps/order/rpc/model/order_items.sql
apps/order/rpc/model/order_address_snapshot.sql
apps/order/rpc/model/order_request_mapping.sql
apps/cart/rpc/model/cart.sql
init-sql/pay.sql
```

---

## 项目亮点

- **分布式事务**：DTM Saga 保证跨服务（库存 + 订单）的原子性，含幂等 Barrier 防重复执行
- **高并发库存**：Redis Lua 原子脚本防超卖，支持预扣→确认→回滚全生命周期管理
- **异步解耦**：RabbitMQ 承载订单事件、延迟取消、库存回补、支付回调等异步流程
- **可观测性**：OpenTelemetry 全链路追踪 + Prometheus 指标 + Grafana 可视化
- **幂等设计**：下单接口支持 `request_id` 前端去重，消费者基于 Redis SETNX 防重复消费
- **优雅降级**：Redis 库存未初始化时自动降级至 DB 层，不影响核心下单链路
