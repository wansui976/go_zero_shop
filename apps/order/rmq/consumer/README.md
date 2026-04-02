# Order RMQ Consumers

本目录包含订单系统的RabbitMQ消息消费者，用于异步处理各种订单相关事件。

## 消费者列表

### 1. order consumer - 订单事件消费者
**功能**: 监听订单创建事件，处理订单确认通知、统计更新和库存预留。

**配置**: `etc/order-consumer.yaml`

**启动命令**:
```bash
go run ./order -f ./etc/order-consumer.yaml
```

### 2. delay consumer - 延迟队列消费者
**功能**: 处理订单延迟任务，如取消未支付订单、发送支付提醒等。

**配置**: `etc/delay-consumer.yaml`

**启动命令**:
```bash
go run ./delay -f ./etc/delay-consumer.yaml
```

### 3. stock consumer - 库存变更消费者
**功能**: 监听库存变更事件，实现库存的统一变更处理和幂等性保证。

**配置**: `etc/stock-consumer.yaml`

**启动命令**:
```bash
go run ./stock -f ./etc/stock-consumer.yaml
```

### 4. notify consumer - 通知事件消费者
**功能**: 监听 `order.notification.queue`，异步处理订单取消通知和支付提醒通知。

**配置**: `etc/notify-consumer.yaml`

**启动命令**:
```bash
go run ./notify -f ./etc/notify-consumer.yaml
```

## 修复的问题

### 1. 配置文件缺失
- **问题**: 消费者程序尝试加载配置文件但 `etc/` 目录不存在
- **修复**: 创建了 `etc/` 目录并添加了相应的配置文件

### 2. 类型断言panic
- **问题**: `stock_change_consumer.go` 中对消息头 `x-retry-count` 的类型断言可能导致panic
- **修复**: 改进了类型检查逻辑，避免了panic

## 依赖

- RabbitMQ 服务
- Redis 服务（stock_change_consumer 与 notify_consumer 都使用）

## 编译

```bash
# 编译所有消费者
go build -o order_consumer ./order
go build -o delay_queue_consumer ./delay
go build -o stock_change_consumer ./stock
go build -o notify_consumer ./notify
```

## 架构说明

这些消费者共同构成了订单系统的异步处理架构：

1. **order_consumer**: 处理实时订单事件
2. **delay_queue_consumer**: 处理定时任务（如订单超时）
3. **stock_change_consumer**: 处理库存变更，实现最终一致性
4. **notify_consumer**: 处理通知事件，实现取消/提醒异步解耦
