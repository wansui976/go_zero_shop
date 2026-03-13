# Go Zero Shop Architecture State

_Last updated: 2026-03-12 (Asia/Shanghai)_

This document captures the repository's current architecture shape as observed from the codebase and deployment files. It is intended as a durable snapshot so future analysis can start from the same baseline instead of re-deriving the system from scratch.

## 1. System Positioning

`go_zero_shop` is a Go-based microservice e-commerce system built primarily on `go-zero` and `gRPC`, with REST exposed through an API gateway layer.

Core characteristics observed in the repo:

- Multi-service domain split: user, product, order, cart, pay, search, reply, seckill, recommend.
- REST + gRPC hybrid architecture.
- Synchronous service-to-service calls via `zrpc`/gRPC.
- Asynchronous event processing via RabbitMQ in several flows.
- Distributed transaction support via DTM in the order path.
- Multiple caching layers: go-zero cache, Redis business cache, local cache, bloom filter, and singleflight.
- Observable deployment intent: Prometheus, Grafana, Jaeger.
- Two deployment surfaces: local/docker-compose and Kubernetes.

## 2. Top-Level Layout

The repository currently contains these important top-level areas:

- `apps/`: primary business services and service entrypoints.
- `pkg/`: shared infrastructure and utility packages.
- `docker-compose.yml`: local multi-service deployment definition.
- `k8s/`: Kubernetes manifests and configs.
- `Dockerfile/`: per-service container build files.
- `start_services.sh` / `stop_services.sh`: local process-based start/stop scripts.
- `internal/`, `pay.go`, `payservice/`, `etc/pay.yaml`: a root-level payment service skeleton that appears separate from the `apps/pay/rpc` structure.

Repository hygiene note:

- The repo also contains `.gocache`, `bin`, `logs`, `.idea`, `.vscode`, and generated runtime artifacts. This means the repository is acting partly as a working directory, not just a clean source tree.

## 3. Runtime Entry Surfaces

### 3.1 External HTTP entry

The main external HTTP entrypoint is:

- `apps/app/api/api.go`

It loads `etc/api-api.yaml` and starts a `go-zero` REST server on port `8888`.

### 3.2 RPC services

Observed RPC service entrypoints:

- `apps/user/rpc/user.go`
- `apps/product/rpc/product.go`
- `apps/order/rpc/order.go`
- `apps/cart/rpc/cart.go`
- `apps/pay/rpc/pay.go`
- `apps/search/rpc/search.go`
- `apps/reply/rpc/reply.go`
- `apps/seckill/rpc/seckill.go`

These are standard `go-zero` zrpc servers that expose gRPC/proto services.

### 3.3 Additional admin surface

There is a separate admin HTTP surface for reply/comment management:

- `apps/reply/admin/admin.go`

### 3.4 Async consumers / publishers

Observed message-driven entrypoints:

- `apps/order/rmq/consumer/delay/main.go`
- `apps/order/rmq/consumer/notify/main.go`
- `apps/order/rmq/consumer/order/main.go`
- `apps/order/rmq/consumer/stock/main.go`
- `apps/pay/rmq/consumer/main.go`
- `apps/pay/rmq/publisher/main.go`
- `apps/reply/rmq/main.go`

## 4. Service Map

Based on the current repo layout, the system is organized approximately as follows.

### 4.1 API layer

`apps/app/api`

Responsibility:

- Public REST API surface.
- JWT authentication boundary.
- Calls downstream RPC services for business execution.
- Provides the mobile/app style HTTP contract defined in `api.api`.

Current downstream dependencies declared in config/service context:

- Order RPC
- Product RPC
- User RPC
- Search RPC
- Cart RPC
- Redis for idempotency/business use

Notably present in code config but not fully wired in current service context:

- Reply RPC config exists, but reply client creation is commented out.

### 4.2 User service

`apps/user/rpc`

Responsibility:

- Login
- User info
- Shipping addresses
- Product collections/favorites

Storage/dependencies:

- MySQL via go-zero model layer
- go-zero cache
- Redis business versioning / address-related business cache helper

### 4.3 Product service

`apps/product/rpc`

Responsibility:

- Product detail
- Product list / full list
- Operation/recommended products
- Stock checking and deduction
- Product catalog data backing search/recommend/homepage usage

Storage/dependencies:

- MySQL
- go-zero cache
- Redis business cache
- Local in-process cache
- `singleflight` deduplication
- Bloom filter for product existence / hot-path optimization
- Asynq client backed by Redis
- Gorm-based ORM path alongside go-zero/sqlx model path

This is one of the most infrastructure-heavy services in the repo.

### 4.4 Order service

`apps/order/rpc`

Responsibility:

- Order creation
- Order validation/check
- Order listing and order detail
- Rollback path for failed order creation
- DTM try/confirm/cancel transaction flow

Storage/dependencies:

- MySQL
- go-zero cache
- Product RPC
- User RPC
- RabbitMQ
- Redis for gid/request mapping
- Snowflake ID generation

This service appears to be one of the orchestration centers of the business flow.

### 4.5 Cart service

`apps/cart/rpc`

Responsibility:

- Shopping cart operations

The repo layout confirms its presence, but this analysis pass did not yet inspect its full internal logic.

### 4.6 Pay service

`apps/pay/rpc`

Responsibility:

- Payment creation
- Payment callback
- Refund and refund lookup

Storage/dependencies:

- MySQL
- go-zero cache
- Order RPC reference in config
- Payment provider config blocks for WeChat Pay and Alipay

### 4.7 Search service

`apps/search/rpc`

Responsibility:

- Product search capabilities

Signals in repo:

- Dedicated search proto/service
- Elasticsearch dependency exists in `go.mod`
- Shared `pkg/es` package exists

### 4.8 Reply service

`apps/reply/rpc` and `apps/reply/admin`

Responsibility:

- Product comments / reply domain
- Separate admin-facing surface

### 4.9 Seckill service

`apps/seckill/rpc`

Responsibility:

- Flash-sale / seckill flow

Dependencies seen in code:

- Product RPC
- Redis
- Kafka pusher

This is important because it does not match all deployment artifacts; see inconsistencies section.

### 4.10 Recommend service

`apps/recommend/rpc`

Responsibility:

- Recommended products

The repo layout shows the service exists, but this pass did not inspect its internal implementation yet.

## 5. Interface Design

### 5.1 REST contract

The primary REST contract is defined in:

- `apps/app/api/api.api`

Observed API groups:

- Home
- Product
- Product stock
- Cart
- User
- Order

The API contract uses a unified response envelope:

- `resultCode`
- `msg`
- `data`

This suggests a frontend-oriented API design with a consistent app/mobile response format.

### 5.2 RPC contracts

Observed key proto contracts:

- `apps/user/rpc/user.proto`
- `apps/product/rpc/product.proto`
- `apps/order/rpc/order.proto`
- `apps/cart/rpc/cart.proto`
- `apps/pay/rpc/pay.proto`
- `apps/search/rpc/search.proto`
- `apps/reply/rpc/reply.proto`
- `apps/seckill/rpc/seckill.proto`

The order proto is especially important because it exposes both:

- normal order lifecycle RPCs
- DTM-based try/confirm/cancel RPCs

This indicates the repo intends to support both simpler synchronous order flow and distributed transaction-based order flow.

## 6. Cross-Service Dependency Shape

From currently inspected files, the dependency pattern looks like this:

- API -> Order RPC, Product RPC, User RPC, Search RPC, Cart RPC
- Order RPC -> Product RPC, User RPC, RabbitMQ, Redis, MySQL
- Seckill RPC -> Product RPC, Redis, Kafka
- Pay RPC -> Order domain + payment persistence
- Product RPC -> MySQL, Redis, Asynq, bloom/local cache
- User RPC -> MySQL, Redis

A simplified business graph:

1. Client requests hit API on `8888`.
2. API authenticates (where needed) and dispatches to downstream RPC services.
3. Order creation coordinates user/address data, product stock state, and transactional/order event flow.
4. Async consumers handle delayed, stock, notify, pay, or reply side effects.
5. Metrics/tracing stack is intended to observe the service mesh.

## 7. State Management and Reliability Mechanisms

Several reliability/performance mechanisms are already present.

### 7.1 Idempotency

`pkg/idempotent/idempotment.go`

Pattern:

- Redis `SETNX` with TTL
- explicit delete on failure path supported

This is suitable for message re-consumption protection and request deduplication.

### 7.2 Message queue abstraction

`pkg/mq/rabbitmq.go`

Capabilities observed:

- pooled channels
- auto reconnect
- queue declaration
- queue binding
- publish
- consume
- QoS setting

This is more than a thin wrapper; it is an in-house RabbitMQ client abstraction with resilience behavior.

### 7.3 Product-side hot-path optimization

Product service includes:

- Redis business cache
- local cache
- `singleflight`
- bloom filter
- async warm-up of top products at startup

This suggests product read traffic is treated as a performance-sensitive hotspot.

### 7.4 Distributed transactions

Order service includes DTM-facing RPC methods:

- `CreateOrderDTM`
- `CreateOrderDTMConfirm`
- `CreateOrderDTMRevert`

This indicates the architecture is trying to solve cross-service order/stock/payment consistency beyond simple local transactions.

## 8. Deployment Surfaces

### 8.1 docker-compose

`docker-compose.yml` defines a fairly complete local environment:

Infrastructure:

- MySQL
- Redis
- Etcd
- RabbitMQ
- Jaeger
- Prometheus
- Grafana

Services:

- user-rpc
- product-rpc
- order-rpc
- seckill-rpc
- cart-rpc
- pay-rpc
- search-rpc
- recommend-rpc
- reply-rpc
- api-gateway
- seckill-rmq
- order-rmq
- pay-rmq
- reply-rmq

This indicates the intended local environment is close to a full-stack microservice environment, not a partial mock setup.

### 8.2 Kubernetes

The `k8s/` directory and `K8S-README.md` describe Kubernetes deployment, with manifests for:

- infrastructure
- configs
- deployments
- services
- base resources

However, the Kubernetes description does not perfectly match the currently observed application/runtime code. See section 10.

### 8.3 Local script-based startup

`start_services.sh` starts only a subset of services directly with `go run`:

- user-rpc
- product-rpc
- order-rpc
- search-rpc
- cart-rpc
- seckill-rpc
- api-gateway

Not started by this script despite existing in repo:

- pay-rpc
- reply-rpc
- recommend-rpc
- RMQ consumers

This means the script describes a reduced operational slice, not the whole system.

## 9. Shared Package Roles

Observed shared packages likely serve these roles:

- `pkg/es`: Elasticsearch integration
- `pkg/idempotent`: deduplication / retry safety
- `pkg/mq`: RabbitMQ abstraction
- `pkg/orm`: gorm/mysql helper
- `pkg/result`: HTTP response wrappers
- `pkg/snowflake`: ID generation
- `pkg/tool`: encryption/utility helpers
- `pkg/xerr`: error code and mapping layer
- `pkg/bacher`: batching helper (name suggests batch processing utility)

## 10. Current Architectural Inconsistencies and Drift

This is the most important section for future work. The repo contains multiple signs of architecture drift.

### 10.1 Seckill messaging path is inconsistent

- `apps/seckill/rpc/internal/svc/servicecontext.go` uses Kafka via `kq.NewPusher(...)`.
- `docker-compose.yml` provisions RabbitMQ for seckill-related runtime.
- `K8S-README.md` shows seckill -> Kafka.

Interpretation:

- The code and deployment docs are not fully aligned.
- Seckill likely migrated, partially migrated, or was designed against Kafka while local deployment moved to RabbitMQ elsewhere.

### 10.2 Gateway naming / layering is inconsistent

- Active API surface is in `apps/app/api`.
- There is also `apps/gateway/`, but it currently appears skeletal/incomplete.
- `docker-compose.yml` names the external service `api-gateway`.

Interpretation:

- The project may have been moving from one gateway layout to another, or `apps/gateway` is an abandoned replacement.

### 10.3 Root-level payment service duplicates `apps/pay/rpc`

At the repo root there is:

- `pay.go`
- `payservice/`
- `internal/`
- `etc/pay.yaml`

Separately, the main service tree contains:

- `apps/pay/rpc`

Interpretation:

- There is likely an older payment implementation or an extracted standalone experiment that was not fully removed after moving to `apps/pay/rpc`.

### 10.4 Compose, K8s, and startup script cover different service subsets

- Compose includes pay/recommend/reply and multiple RMQ workers.
- `start_services.sh` does not start all of those.
- K8s docs describe an architecture that is similar but not identical.

Interpretation:

- There is no single fully trusted source of truth yet for the runnable production topology.

### 10.5 Security/configuration posture is still development-grade

Observed examples:

- hardcoded JWT secret in API config
- hardcoded MySQL root password in compose
- hardcoded Redis password in compose
- hardcoded RabbitMQ admin password in compose
- admin password for Grafana in compose

Interpretation:

- Current config state is suitable for local/dev only.
- The repo should not be treated as production-ready without config extraction and secret management.

### 10.6 Generated/runtime artifacts are committed or kept in-tree

Examples:

- `.gocache`
- `logs/`
- `bin/`

Interpretation:

- The repo is carrying local runtime state, which increases noise and makes architecture reading less clean.

## 11. Current Best-Guess Architecture Summary

As of this snapshot, the safest working mental model is:

- The true primary public interface is `apps/app/api`.
- The system is centered around domain RPC services under `apps/*/rpc`.
- Orders are the main orchestration domain and touch product, user, MQ, Redis, and DTM.
- Product is the main read-heavy optimization domain.
- Async processing is a first-class design concern, but the exact transport choice is drifting across modules.
- There are signs of at least one architectural refactor in progress or abandoned mid-way.

## 12. Recommended Next Investigation Passes

To continue architecture clarification, the next highest-value passes are:

1. Trace each service's `etc/*.yaml` configs and build a definitive port/dependency matrix.
2. Inspect `apps/*/internal/logic` to map actual business call chains, especially order/pay/seckill.
3. Reconcile the messaging architecture: Kafka vs RabbitMQ vs Asynq by actual active call sites.
4. Decide whether `apps/gateway` and root-level `pay*` are active, transitional, or dead code.
5. Create a canonical runtime topology doc after verifying which services are truly required for a full order flow.

## 13. Main Business Chains (Current State)

This section captures the current end-to-end business flow for the three most important paths: order creation, payment, and seckill. These are based on code currently present in the repo, not on intended behavior alone.

### 13.1 Order creation chain

#### Entry

Primary entrypoint:

- `apps/app/api/internal/logic/order/addorderlogic.go`

Observed flow:

1. API extracts authenticated user ID from JWT context.
2. API performs request-level idempotency using Redis via `request_id` if provided.
3. API validates item count, duplicate product IDs, quantity ranges, payment type, and address ID.
4. API generates a DTM global transaction ID (`gid`).
5. API transforms request items into:
   - product stock deduction items
   - order item payloads
6. API builds and submits a DTM Saga.

#### Saga composition

The API-side Saga is currently built in this order:

1. `product.Product/DecrStock`
2. `order.OrderService/CreateOrderDTM`

Compensation endpoints:

1. `product.Product/DecrStockRevert`
2. `order.OrderService/CreateOrderDTMRevert`

Important note:

- The API code hardcodes service targets as `host.docker.internal:8081` and `host.docker.internal:8082` instead of using discovered/configured endpoints. This is an architecture coupling risk and may break outside specific local/container setups.

#### Order service Try stage

Primary implementation:

- `apps/order/rpc/internal/logic/createorderdtmlogic.go`

Observed responsibilities in Try stage:

1. Validate `gid`, `user_id`, and item list.
2. Parallel pre-check:
   - fetch user info from User RPC
   - fetch shipping address from User RPC
   - fetch each product and verify stock from Product RPC
3. Generate order ID via Snowflake.
4. Enter DTM barrier + DB transaction.
5. Pre-lock stock in Redis per `gid + productId` using Lua scripts.
6. Insert shipping snapshot.
7. Insert order items.
8. Insert main order record with status `1` (pending/payable state) and bind `gid`.
9. After transaction success, publish `OrderCreated` event to RabbitMQ queue `order_create_queue`.

A notable design detail:

- The order service is not directly deducting durable product stock in MySQL here. It is maintaining a Redis-side pre-lock lifecycle keyed by `gid` and relying on DTM stage transitions to confirm or revert the pre-lock.

#### Confirm stage

Primary implementation:

- `apps/order/rpc/internal/logic/createorderdtmconfirmlogic.go`

Observed responsibilities:

1. Load order by `order_id` inside barrier-protected DB transaction.
2. Verify that order exists and belongs to the current `gid`.
3. Verify status is still pending state.
4. For each order item, convert Redis stock state from `pre_locked` to `confirmed`.
5. Persist `request_id -> order_id` mapping if the API layer had stored `gid -> request_id` in Redis.

Important note:

- Confirm does not visibly advance the business order status beyond the initial pending state. It mainly finalizes the stock pre-lock lifecycle and request mapping.

#### Revert stage

Primary implementation:

- `apps/order/rpc/internal/logic/createorderdtmrevertlogic.go`

Observed responsibilities:

1. Use `gid` to locate the order.
2. If no order exists, still attempt to release Redis-side pre-locked stock using request payload items.
3. If order exists, set order status to `0` (cancelled).
4. Revert Redis-side stock pre-locks per item.
5. Handle repeated execution idempotently.

#### Async side effects after order creation

Primary consumer:

- `apps/order/rmq/consumer/order/main.go`

Observed side effects of `OrderCreated` event:

- idempotent event consumption using Redis
- cache order item snapshot to Redis for later delayed cancel compensation
- placeholder notification send
- placeholder statistics update
- placeholder inventory reserve hook

Important note:

- These consumers are partially scaffolded. They establish the event-driven shape, but some side effects are still TODO/stub implementations.

#### Delayed order follow-up

Primary consumer:

- `apps/order/rmq/consumer/delay/main.go`

Observed intended delayed actions:

- delayed cancel of unpaid orders
- payment reminder after delay
- on cancel: publish stock rollback events to stock exchange and publish order cancellation notification event

Important note:

- The delay consumer is architecturally significant, but the current order creation path does not yet show the actual publishing of delay messages after order creation. So the intended delayed cancellation architecture exists, but the hook-in point appears incomplete or located elsewhere not yet inspected.

#### Current order-chain summary

Best current reading of the order chain:

1. Client -> API `AddOrder`
2. API idempotency + validation
3. API submits DTM Saga
4. Product branch handles stock deduction/pre-deduction (exact product-side implementation not inspected in this pass)
5. Order Try creates pending order + shipping snapshot + order items + Redis stock pre-lock state
6. Order Confirm finalizes Redis stock pre-lock state
7. Order service emits `OrderCreated` RabbitMQ event
8. Order async consumers perform side work and preserve compensation metadata
9. Delayed cancellation/reminder architecture exists, but its producer path may still be incomplete

### 13.2 Payment chain

#### Core RPC surface

Primary contract:

- `apps/pay/rpc/pay.proto`

Exposed operations:

- create payment
- query payment by order ID
- payment callback
- refund
- query refund by ID

#### Create payment flow

Primary implementation:

- `apps/pay/rpc/internal/logic/createpaymentlogic.go`

Observed flow:

1. Generate `payment_id` using Snowflake.
2. Insert payment record into payment table with status `Pending`.
3. Generate mock payment URL based on payment type.
4. Return payment URL and expiry time.

Important notes:

- This is currently a local/internal payment order creation flow.
- The pay URL generation is mocked, not an actual provider integration.
- No inspected API-layer file yet shows where this RPC is called from the main user-facing API.

#### Payment callback flow

Primary implementation:

- `apps/pay/rpc/internal/logic/paymentcallbacklogic.go`

Observed flow:

1. Load payment by `payment_id`.
2. Signature verification is commented as TODO.
3. Update payment record with callback status, transaction ID, and pay time.
4. Comment notes that notifying order service is optional, but no actual RPC call is implemented.

This means:

- Payment state can be marked paid in the payment service.
- Order status advancement after successful payment is not yet wired in the inspected code.

#### Refund flow

Primary implementation:

- `apps/pay/rpc/internal/logic/refundlogic.go`

Observed flow:

1. Validate refund amount.
2. Load payment by `payment_id`.
3. Require payment status to already be `Paid`.
4. Validate payment/order match.
5. Check cumulative refund amount does not exceed paid amount.
6. Insert refund record in pending state.
7. Simulate external refund API success.
8. Update refund record to success.

Important note:

- Refund provider integration is also currently mocked.

#### Async payment callback path

Additional message-driven payment path exists:

- publisher: `apps/pay/rmq/publisher/main.go`
- consumer: `apps/pay/rmq/consumer/main.go`

Observed behavior:

- Publisher emits `pay.callback` events to RabbitMQ.
- Consumer reads the callback queue and updates payment records.

This creates a second callback path alongside direct RPC callback handling.

Interpretation:

- The payment domain currently supports both direct RPC callback handling and MQ-driven callback processing.
- This is potentially flexible, but also introduces duplication unless one path is canonicalized.

#### Current payment-chain summary

Best current reading of the payment chain:

1. Some caller creates payment through Pay RPC.
2. Pay service stores pending payment record and returns mock payment link.
3. Provider callback is expected either:
   - directly via `PaymentCallback` RPC, or
   - indirectly via RabbitMQ callback event
4. Pay service marks payment success/failure.
5. Order status synchronization after payment is not yet fully implemented in inspected code.
6. Refund flow is internally complete at record/state level but uses mocked provider success.

So the payment chain exists structurally, but its integration into the order lifecycle is currently weaker than the order creation chain.

### 13.3 Seckill chain

#### RPC entry

Primary contract:

- `apps/seckill/rpc/seckill.proto`

Primary request path:

- `apps/seckill/rpc/internal/logic/seckillorderlogic.go`

Observed synchronous front-path:

1. User submits seckill request with `user_id` and `product_id`.
2. Service performs per-user distributed rate limiting via Redis-backed `PeriodLimit`.
3. Service fetches product through Product RPC and checks visible stock > 0.
4. Service pushes request into an in-process batcher.
5. Batcher aggregates requests and sends JSON payloads to Kafka using `KafkaPusher`.

Key design intent:

- front-path is lightweight and admission-controlled
- actual order/stock work is deferred asynchronously
- batching reduces MQ write pressure under burst traffic

#### Seckill async consumer path

Primary consumer service:

- `apps/seckill/rmq/internal/service/service.go`

Observed intended flow:

1. Consume Kafka messages (single or batched).
2. Shard them by product ID into multiple in-process channels to serialize same-product contention.
3. For each request, start a DTM TCC global transaction.
4. TCC branch 1: product stock reservation / confirmation / cancellation
5. TCC branch 2: order pre-create / confirm / cancel

This is architecturally important because it shows the intended seckill model is:

- RPC admission -> Kafka batching -> per-product sharded consumer -> DTM TCC transaction

#### Major inconsistency in seckill path

The inspected seckill consumer references RPC endpoints that do not match the currently inspected order/product contracts:

Expected by seckill consumer:

- `product.Product/CheckAndReserveStock`
- `product.Product/ConfirmStockDeduct`
- `product.Product/CancelStockReserve`
- `order.Order/TryCreateOrder`
- `order.Order/ConfirmOrder`
- `order.Order/CancelOrder`

But currently inspected contracts expose different names such as:

- product: `DecrStock`, `DecrStockRevert`, `CheckAndUpdateStock`, etc.
- order: `CreateOrderDTM`, `CreateOrderDTMConfirm`, `CreateOrderDTMRevert`

Interpretation:

- The seckill RMQ/TCC path appears to target an older or planned interface, not the current live one.
- This chain is therefore architecturally designed but very likely not runnable without refactoring/alignment.

#### Current seckill-chain summary

Best current reading of the seckill chain:

1. Client -> Seckill RPC
2. Redis rate limit + lightweight product stock check
3. Batch and publish to Kafka
4. Kafka consumer should drive DTM TCC stock+order flow
5. But the consumer currently targets RPC methods that do not align with inspected current service contracts

So seckill is the least coherent of the three major chains at this snapshot: conceptually advanced, but contract-drifted.

## 14. Mainline Repair Status (2026-03-12)

This section records the repository changes made after the initial architecture read, using the order flow as the primary baseline.

### 14.1 Build and dependency repairs already completed

The following repository blockers were fixed so the order baseline and its core dependencies can build together:

- `apps/product/rpc/internal/cache/cache.go`: fixed `ExpireCtx` call shape mismatch.
- `apps/product/rpc/internal/model/categoryModel.go`: added the missing public category model wrapper expected by `ServiceContext`.

After these repairs, the following package groups were verified together via `go test`:

- `./apps/product/rpc/...`
- `./apps/order/rpc/...`
- `./apps/app/api/...`
- `./apps/pay/rpc/...`
- `./apps/order/rmq/consumer/...`

### 14.2 Order Saga/service-address cleanup

The order creation API originally hardcoded DTM branch targets using `host.docker.internal:8081/8082`.

This was corrected so the API now resolves actual branch targets from configured `zrpc` client configs:

- Product target from `Config.ProductRPC.BuildTarget()`
- Order target from `Config.OrderRPC.BuildTarget()`

This makes the order baseline less coupled to one local/container topology.

### 14.3 Order event contract cleanup

The order service originally published `order_create_queue` events using a generic map and serialized `order_id` as a string.

This was changed to:

- publish a typed event structure
- send numeric `order_id` consistently as `int64`

This removes a real producer/consumer contract mismatch because the order consumer expects numeric order IDs.

### 14.4 Delay queue is now attached to the real order flow

The order creation path now publishes delay messages for:

- payment reminder
- unpaid order cancellation

The order service startup path also declares the required RabbitMQ pieces for this flow:

- `dlx.exchange`
- `order.delay.queue`
- `order.dlq.queue`
- `order.notification.queue`

This means the delay consumer is no longer only an isolated scaffold; it is connected to the main order path.

### 14.5 Payment callback now advances the order state

Previously, payment callback handling updated only the payment record and did not complete the order lifecycle.

This was repaired so that on successful payment callback the pay service now:

- updates the payment record
- marks the order paid through the order model transition path
- writes a Redis paid marker keyed by the numeric order primary ID

This change is important because the delay cancellation path checks this paid marker before cancelling unpaid orders.

### 14.6 Delay cancellation has been partially domainized

The delay consumer no longer directly mutates the order table as its primary path.

Current behavior:

1. query order status through Order RPC
2. if status is still pending, call the formal `CancelOrder` RPC
3. only then proceed with stock compensation event publishing and notification event publishing

This is architecturally cleaner than the earlier direct-model update approach.

### 14.7 Formal cancel RPC added to order domain

A dedicated order cancellation RPC was added:

- `CancelOrder(CancelOrderRequest) returns (CancelOrderResponse)`

Purpose:

- provide a stable domain boundary for timeout-driven cancellation
- allow delay consumer and future callers to cancel through the order service instead of touching the order model directly

Implementation notes:

- the cancel logic reuses the order status transition/update path
- intended target transition is `Pending -> Canceled`

### 14.8 `CreateOrderCheck` removed from the public service contract

`CreateOrderCheck` was exposed in the order service contract but remained unimplemented.

That method has now been removed from the public order RPC contract so the surface area better reflects real behavior.

Important implication:

- future order pre-check logic should be reintroduced only after a real implementation is ready, not as a placeholder contract.

### 14.9 Payment creation is now part of the internal order chain

The order service no longer treats payment creation as an external follow-up guess.

After successful order creation, the order service now:

- calls Pay RPC `CreatePayment`
- stores the returned payment metadata in Redis keyed by `order_id`

Persisted payment metadata currently includes:

- `order_id`
- `payment_id`
- `pay_url`
- `expire_time`

This lets downstream callers read the exact pay creation result produced inside the order domain flow instead of reconstructing it later.

### 14.10 API order creation now returns payment info from internal chain output

The API layer no longer relies on a second Pay RPC query to synthesize payment info after the order saga completes.

Current API behavior:

1. submit DTM saga
2. read `gid -> order_id` mapping
3. read `order_id -> payment_info` payload from Redis
4. return `order_id`, `payment_id`, `pay_url`, `expire_time` together with `gid`

This is closer to the intended behavior: payment info comes from the internal order->pay chain, and the API just returns the produced result.

### 14.11 Current mainline state after repairs

As of this snapshot, the practical working model of the mainline is:

1. API receives order request.
2. API validates and submits DTM saga.
3. Product stock branch and order branch complete.
4. Order service creates order record, emits order event, emits delay messages, and creates payment.
5. Order service stores payment result for API retrieval.
6. API returns `gid + order_id + payment_id + pay_url + expire_time` when available.
7. Payment callback updates payment and advances order to paid.
8. Delay consumer checks whether order is still unpaid; if so, it cancels through Order RPC and triggers compensation/notification flow.

### 14.12 Remaining mainline gaps

Even after these repairs, the following gaps remain:

- Delay cancellation still uses Redis paid markers as a shortcut safety signal; a fully authoritative order-status check is still the stronger source of truth.
- Compensation and notification flows are structurally wired, but some downstream consumers still include placeholder/TODO behaviors.
- The repo still contains generated duplicate package trees under `apps/order/rpc/apps/order/rpc`, which should be normalized/cleaned later.
- Seckill remains drifted from the repaired order baseline and should not yet be treated as a production-quality extension path.

## 15. Files Used for This Snapshot

Main files inspected during this pass included:

- `go.mod`
- `docker-compose.yml`
- `K8S-README.md`
- `apps/app/api/api.go`
- `apps/app/api/api.api`
- `apps/app/api/etc/api-api.yaml`
- `apps/app/api/internal/config/config.go`
- `apps/app/api/internal/svc/servicecontext.go`
- `apps/order/rpc/order.go`
- `apps/order/rpc/order.proto`
- `apps/order/rpc/internal/config/config.go`
- `apps/order/rpc/internal/svc/servicecontext.go`
- `apps/product/rpc/product.go`
- `apps/product/rpc/product.proto`
- `apps/product/rpc/internal/config/config.go`
- `apps/product/rpc/internal/svc/servicecontext.go`
- `apps/user/rpc/user.proto`
- `apps/user/rpc/internal/config/config.go`
- `apps/user/rpc/internal/svc/servicecontext.go`
- `apps/seckill/rpc/internal/config/config.go`
- `apps/seckill/rpc/internal/svc/servicecontext.go`
- `apps/pay/rpc/internal/config/config.go`
- `apps/pay/rpc/internal/svc/servicecontext.go`
- `apps/reply/admin/admin.go`
- `pkg/mq/rabbitmq.go`
- `pkg/idempotent/idempotment.go`
- `start_services.sh`
- `stop_services.sh`

---

If future code changes make this snapshot stale, update this document rather than replacing it with tribal memory.
