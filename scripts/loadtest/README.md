# 压测脚本

这里放了 4 个基于 **k6** 的压测脚本：

- `daily-normal.js`：日常混合流量
- `extreme-spike.js`：热点商品极端突发流量
- `daily-soak.js`：长时间日常流量模拟（Soak Test）
- `multi-user-order-mix.js`：**多用户并发下单 + 购物车 + 查询混合场景**

## 这次更新了什么

当前脚本已经从“单账号压测”升级为“**多用户 + 多商品 + 多操作混合压测**”：

- 支持账号池轮转登录
- 支持大商品池随机取商品
- 支持多用户同时下单
- 支持并发购物车增删改查
- 支持订单列表、用户信息、地址查询等混合操作

如果你已经用 `cmd/mockdata` 造了数据，默认推荐直接用账号池：

```bash
load_user_000001 ~ load_user_060000
密码统一为 123456
```

---

## 前置条件

1. API 网关可访问，默认地址：

   ```bash
   http://127.0.0.1:8888
   ```

2. 已安装 k6

3. 如果使用多用户压测，建议先完成基础造数：
   - 6 万+ 用户
   - 10 万+ 商品
   - 100 万+ 历史订单

---

## 默认多用户配置

如果你**不传 `USERNAME`**，脚本默认使用账号池：

```bash
USERNAME_PREFIX=load_user_
USER_ID_START=1
USER_ID_END=60000
USER_PAD_WIDTH=6
PASSWORD=123456
```

也就是说，脚本会自动使用：

```bash
load_user_000001
load_user_000002
...
load_user_060000
```

如果你只想退回单用户压测，可以显式指定：

```bash
USERNAME=testuser
PASSWORD=123456
```

---

## 常用环境变量

### 服务与登录

```bash
BASE_URL=http://127.0.0.1:8888
USERNAME=
PASSWORD=123456
USERNAME_PREFIX=load_user_
USER_ID_START=1
USER_ID_END=60000
USER_PAD_WIDTH=6
USER_ROTATION_STEP=17
USER_ROTATION_WINDOW=50
LOGIN_RETRY_MAX=3
LOGIN_RETRY_SLEEP=1
FAIL_ON_SESSION_ERROR=false
```

说明：

- `USERNAME` 有值：走单用户
- `USERNAME` 为空：走用户池
- `USER_ROTATION_WINDOW`：同一个 VU 连续用多少轮后再切换账号
- `USER_ROTATION_STEP`：切换账号时的跳步，避免热点都打到同一批账号
- `LOGIN_RETRY_MAX`：登录失败自动重试次数
- `FAIL_ON_SESSION_ERROR=false`：登录或地址失败时跳过本次迭代，不直接打断压测

### 商品池

```bash
PRODUCT_ID=1001
PRODUCT_PRICE=19900
PRODUCT_IDS=
PRODUCT_ID_MIN=1
PRODUCT_ID_MAX=120000
PRODUCT_POOL_SIZE=60
```

优先级：

1. `PRODUCT_IDS`
2. `PRODUCT_ID_MIN + PRODUCT_ID_MAX`
3. `/v1/home/index-infos`
4. `PRODUCT_ID`

推荐在大数据量场景下显式指定范围：

```bash
PRODUCT_ID_MIN=1
PRODUCT_ID_MAX=120000
PRODUCT_POOL_SIZE=80
```

### 地址

```bash
ADDRESS_ID=
```

不传时，脚本会自动读取用户地址；如果没有地址，会自动创建一条默认地址。

---

## 运行示例

## 1）日常混合场景

```bash
BASE_URL=http://127.0.0.1:8888 \
PRODUCT_ID_MIN=1 \
PRODUCT_ID_MAX=120000 \
k6 run scripts/loadtest/daily-normal.js
```

加强版：

```bash
BASE_URL=http://127.0.0.1:8888 \
PASSWORD=123456 \
USER_ID_START=1 \
USER_ID_END=30000 \
PRODUCT_ID_MIN=1 \
PRODUCT_ID_MAX=120000 \
BROWSE_STEADY_RATE=30 \
SHOPPER_VUS=20 \
CHECKOUT_RATE=8 \
k6 run scripts/loadtest/daily-normal.js
```

## 2）极端热点场景

```bash
BASE_URL=http://127.0.0.1:8888 \
PRODUCT_ID_MIN=1 \
PRODUCT_ID_MAX=120000 \
k6 run scripts/loadtest/extreme-spike.js
```

大促峰值示例：

```bash
HOT_READ_PEAK_3=300 \
HOT_CHECKOUT_PEAK_3=80 \
USER_ID_END=50000 \
k6 run scripts/loadtest/extreme-spike.js
```

## 3）长时间 Soak 场景

默认持续 2 小时：

```bash
BASE_URL=http://127.0.0.1:8888 \
PRODUCT_ID_MIN=1 \
PRODUCT_ID_MAX=120000 \
k6 run scripts/loadtest/daily-soak.js
```

先做 10 分钟验证：

```bash
SOAK_DURATION=10m \
SOAK_BROWSE_RATE=5 \
SOAK_SHOPPER_RATE=2 \
SOAK_ORDER_RATE=2 \
PRODUCT_ID_MIN=1 \
PRODUCT_ID_MAX=120000 \
k6 run scripts/loadtest/daily-soak.js
```

## 4）多用户并发下单混合场景

这是当前最适合你现状的脚本。

```bash
BASE_URL=http://127.0.0.1:8888 \
PASSWORD=123456 \
USER_ID_START=1 \
USER_ID_END=60000 \
PRODUCT_ID_MIN=1 \
PRODUCT_ID_MAX=120000 \
PRODUCT_POOL_SIZE=80 \
k6 run scripts/loadtest/multi-user-order-mix.js
```

更激进的示例：

```bash
BASE_URL=http://127.0.0.1:8888 \
PASSWORD=123456 \
USER_ID_START=1 \
USER_ID_END=60000 \
PRODUCT_ID_MIN=1 \
PRODUCT_ID_MAX=120000 \
PRODUCT_POOL_SIZE=100 \
MIX_DURATION=20m \
MIX_BROWSE_RATE=40 \
MIX_CART_RATE=25 \
MIX_ORDER_RATE=15 \
MIX_AFTERSALE_RATE=10 \
k6 run scripts/loadtest/multi-user-order-mix.js
```

---

## 场景说明

### daily-normal.js

覆盖：

- 首页浏览
- 商品详情访问
- 多用户信息查询
- 收货地址查询
- 购物车增删改查
- 多用户下单与订单列表查询

适合：

- 日常容量评估
- 多用户基础回归压测

### extreme-spike.js

覆盖：

- 单热点商品详情高并发访问
- 多用户同时加购
- 多用户同时对热点商品下单

适合：

- 大促前热点链路摸底
- 验证商品、购物车、订单链路的突发承压能力

### daily-soak.js

覆盖：

- 中低强度持续浏览
- 中低强度持续购物行为
- 中低强度持续下单

适合：

- 观察长时间运行下 RT 抖动
- 排查连接泄漏、缓存衰退、内存增长

### multi-user-order-mix.js

覆盖：

- 浏览流量
- 用户查询
- 购物车操作
- 多商品下单
- 订单查询
- 地址查询

适合：

- 大数据量下的真实业务混合流量压测
- 验证“多用户同时下单 + 各类读写操作交织”的场景

---

## 注意事项

1. 当前仓库部分接口仍未全部实现，因此脚本主要压以下较完整接口：
   - `/v1/home/index-infos`
   - `/v1/product/detail/:productId`
   - `/v1/user/login`
   - `/v1/user/info`
   - `/v1/user/get-receive-address-list`
   - `/v1/user/add-receive-address`
   - `/v1/cart/*`
   - `/v1/order/*`

2. 下单会持续写入订单、购物车、事务相关数据，建议在独立测试环境执行。

3. 如果你已经导入了 100 万+ 历史订单，推荐优先跑：

   ```bash
   k6 run scripts/loadtest/multi-user-order-mix.js
   ```

4. 如果你还想继续增强，我下一步可以再帮你补：
   - 秒杀专用脚本
   - 登录风暴脚本
   - 纯订单洪峰脚本
   - 导出 HTML/JSON 压测报告
