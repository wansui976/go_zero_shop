import http from 'k6/http';
import { group } from 'k6';
import {
  buildHeaders,
  expectCommonSuccess,
  safeJson,
  getSession,
  pickProduct,
  pickProducts,
  randInt,
  requestId,
  setupShopper,
  sleepRange,
} from './lib.js';

/**
 * 混合压测脚本：
 * 1. 使用多个 scenario 同时向系统施压，而不是只测单一接口。
 * 2. 通过“浏览 -> 购物车 -> 下单 -> 查询/售后”四类流量组合，
 *    模拟电商站点更真实的读写比例。
 * 3. 所有场景共享 setup() 生成的测试数据，以降低测试准备成本，
 *    并保证不同 flow 间的数据结构一致。
 */
export var options = {
  // 全局阈值：用于在压测结束后快速判断本轮是否达标。
  thresholds: {
    // 整体失败率控制在 5% 以内。
    http_req_failed: ['rate<0.05'],
    // 95 分位耗时控制在 2 秒以内，避免长尾过高。
    http_req_duration: ['p(95)<2000'],
    // checks（例如 expectCommonSuccess 内部校验）通过率需高于 95%。
    checks: ['rate>0.95'],
  },
  scenarios: {
    // 浏览读流量：
    // - 节奏最快
    // - 不依赖登录
    // - 主要压首页和商品详情等读接口
    browse_read: {
      // constant-arrival-rate 表示以固定到达速率发请求，
      // 更接近“每秒进来多少用户动作”的建模方式。
      executor: 'constant-arrival-rate',
      // 指定场景执行的函数名。
      exec: 'browseFlow',
      // 每秒触发多少次 browseFlow，可通过环境变量覆盖。
      rate: Number(__ENV.MIX_BROWSE_RATE || '20'),
      timeUnit: '1s',
      // 整个场景持续时间；所有场景默认复用同一时长配置。
      duration: __ENV.MIX_DURATION || '15m',
      // 预分配的 VU 数，避免运行中频繁临时扩容。
      preAllocatedVUs: Number(__ENV.MIX_BROWSE_PRE_VUS || '40'),
      // 当预分配不足以支撑速率时，允许扩容到的上限。
      maxVUs: Number(__ENV.MIX_BROWSE_MAX_VUS || '150'),
    },
    // 购物车流量：
    // - 需要登录
    // - 混合“查用户信息、加购、改购、查购物车”操作
    // - 启动时间稍晚于 browse_read，模拟先有浏览后有加购
    cart_ops: {
      executor: 'constant-arrival-rate',
      exec: 'cartFlow',
      rate: Number(__ENV.MIX_CART_RATE || '12'),
      timeUnit: '1s',
      duration: __ENV.MIX_DURATION || '15m',
      preAllocatedVUs: Number(__ENV.MIX_CART_PRE_VUS || '30'),
      maxVUs: Number(__ENV.MIX_CART_MAX_VUS || '120'),
      // 延迟启动，避免所有场景在第 0 秒同时打满系统。
      startTime: __ENV.MIX_CART_START_TIME || '5s',
    },
    // 下单流量：
    // - 写操作更重
    // - QPS 低于浏览/加购
    // - 再次延迟启动，模拟用户经过浏览和加购后的结算行为
    checkout_ops: {
      executor: 'constant-arrival-rate',
      exec: 'checkoutFlow',
      rate: Number(__ENV.MIX_ORDER_RATE || '6'),
      timeUnit: '1s',
      duration: __ENV.MIX_DURATION || '15m',
      preAllocatedVUs: Number(__ENV.MIX_ORDER_PRE_VUS || '25'),
      maxVUs: Number(__ENV.MIX_ORDER_MAX_VUS || '100'),
      startTime: __ENV.MIX_ORDER_START_TIME || '10s',
    },
    // 售后/查询流量：
    // - 偏查询
    // - 少量夹带删除购物车动作
    // - 最后启动，用于拉开各业务阶段的流量层次
    aftersale_query: {
      executor: 'constant-arrival-rate',
      exec: 'afterSaleFlow',
      rate: Number(__ENV.MIX_AFTERSALE_RATE || '5'),
      timeUnit: '1s',
      duration: __ENV.MIX_DURATION || '15m',
      preAllocatedVUs: Number(__ENV.MIX_AFTERSALE_PRE_VUS || '20'),
      maxVUs: Number(__ENV.MIX_AFTERSALE_MAX_VUS || '80'),
      startTime: __ENV.MIX_AFTERSALE_START_TIME || '15s',
    },
  },
};

export function setup() {
  /**
   * setup() 在压测开始前执行一次。
   * 返回值会作为 data 传给各个 VU 执行的 flow 函数。
   *
   * setupShopper() 通常会准备：
   * - baseUrl：目标服务地址
   * - 登录态用户 session 列表
   * - 默认收货地址
   * - 商品池
   *
   * 这样各 flow 内无需重复初始化，大幅减少无意义开销。
   */
  return setupShopper();
}

export function browseFlow(data) {
  /**
   * 浏览流：
   * - 不需要 token
   * - 一次迭代只做两件事：首页 + 商品详情
   * - 主要用于压读请求链路，例如网关、缓存、商品查询接口
   */

  // 从 setup 准备好的商品池中随机选一个商品，避免所有请求集中到同一商品。
  var product = pickProduct(data);

  group('mix browse flow', function () {
    // 首页接口通常会聚合多个模块数据（轮播图、推荐商品、分类等），
    // 是典型的“高频读取”入口。
    var homeResponse = http.get(data.baseUrl + '/v1/home/index-infos', {
      tags: { name: 'mix.home.indexInfos' },
    });
    // 统一校验 HTTP 状态码和业务返回码。
    expectCommonSuccess('mix.home.indexInfos', homeResponse);

    // 查看商品详情，进一步覆盖商品服务读取链路。
    // 此处直接拼接 product.id，确保请求目标真实存在。
    var detailResponse = http.get(data.baseUrl + '/v1/product/detail/' + product.id, {
      tags: { name: 'mix.product.detail' },
    });
    expectCommonSuccess('mix.product.detail', detailResponse);
  });

  // 停顿一小段随机时间（think time）：
  // - 模拟真实用户浏览节奏
  // - 避免单个 VU 以“机器速度”连续轰炸接口
  sleepRange(0.2, 1.0);
}

export function cartFlow(data) {
  /**
   * 购物车流：
   * - 需要登录态
   * - 同时覆盖读写接口
   * - 用于模拟“已登录用户浏览后开始加购”的阶段
   */

  // 从共享数据中取一个可用登录 session。
  // 第三个参数为 true，表示需要严格拿到已登录用户。
  var session = getSession(data, 'mix-cart', true);
  if (!session) {
    // 如果当前没有可用用户，直接结束本次迭代，避免发送无效请求。
    return;
  }

  // 生成携带 token 的通用请求头，后续所有登录接口复用。
  var headers = buildHeaders(session.token);
  // 随机选择一个商品作为加购目标。
  var product = pickProduct(data);

  group('mix cart flow', function () {
    // 先请求用户信息：
    // - 验证 token 是否可用
    // - 覆盖用户中心常见接口
    // - 模拟用户进入个人页/购物车页前的基础查询
    var infoResponse = http.get(data.baseUrl + '/v1/user/info', {
      headers: headers,
      tags: { name: 'mix.user.info' },
    });
    expectCommonSuccess('mix.user.info', infoResponse);

    // 加购请求：
    // - pid：商品 ID
    // - quantity：随机数量，避免所有请求参数完全相同
    // - price：直接使用商品池价格，模拟前端回传单价
    var addCartResponse = http.post(
      data.baseUrl + '/v1/cart/add',
      JSON.stringify({
        pid: product.id,
        quantity: randInt(1, 3),
        price: product.price,
      }),
      {
        headers: headers,
        tags: { name: 'mix.cart.add' },
      }
    );
    expectCommonSuccess('mix.cart.add', addCartResponse);

    // 使用 (__ITER + __VU) % 2 做简单流量切分：
    // - 一半迭代只加购
    // - 一半迭代加购后再改数量
    //
    // 这样能在不引入复杂随机控制的前提下，稳定覆盖 update 接口。
    if ((__ITER + __VU) % 2 === 0) {
      var updateCartResponse = http.post(
        data.baseUrl + '/v1/cart/update',
        JSON.stringify({
          pid: product.id,
          // 改购时把数量扩大到 1~5，模拟用户反复调整购买数。
          quantity: randInt(1, 5),
        }),
        {
          headers: headers,
          tags: { name: 'mix.cart.update' },
        }
      );
      expectCommonSuccess('mix.cart.update', updateCartResponse);
    }

    // 最后查询购物车列表：
    // - 读取当前购物车状态
    // - 覆盖“写后读”的典型场景
    // - 有助于观察写操作后缓存/数据库的一致性表现
    var cartListResponse = http.get(data.baseUrl + '/v1/cart/list', {
      headers: headers,
      tags: { name: 'mix.cart.list' },
    });
    expectCommonSuccess('mix.cart.list', cartListResponse);
  });

  // 购物车操作比浏览略重，因此给更长一点的停顿区间。
  sleepRange(0.3, 1.2);
}

export function checkoutFlow(data) {
  /**
   * 下单流：
   * - 覆盖购物车写入 + 订单创建
   * - 是该脚本中相对更“重”的业务操作
   * - 更适合暴露库存、订单、事务、消息等链路问题
   */

  // 取一个可用下单用户；若不存在则跳过本轮。
  var session = getSession(data, 'mix-checkout', true);
  if (!session) {
    return;
  }
  var headers = buildHeaders(session.token);

  // 一次订单选择 1~3 个商品：
  // - 覆盖单商品订单
  // - 覆盖多商品订单
  // - 避免每一单结构都完全一致
  var items = pickProducts(data, randInt(1, 3));

  group('mix checkout flow', function () {
    // 先将商品逐个加入购物车，再提交订单。
    // 这样虽然会增加一次请求数，但对业务路径模拟更真实。
    for (var i = 0; i < items.length; i++) {
      var item = items[i];
      var addCartResponse = http.post(
        data.baseUrl + '/v1/cart/add',
        JSON.stringify({
          pid: item.id,
          // 这里用 1 / 2 交替，而不是纯随机，
          // 使压测数据既有变化，又更可复现。
          quantity: 1 + (i % 2),
          price: item.price,
        }),
        {
          headers: headers,
          tags: { name: 'mix.checkout.cart.add' },
        }
      );
      // 将商品 ID 带进校验名，便于报错时快速定位是哪一个商品加购失败。
      expectCommonSuccess('mix.checkout.cart.add.' + item.id, addCartResponse);
    }

    // 创建订单：
    // - request_id 保证每次请求有唯一幂等标识
    // - count / payment_type 通过简单扰动提高场景覆盖度
    var createOrderResponse = http.post(
      data.baseUrl + '/v1/order/add',
      JSON.stringify({
        // request_id 一般用于服务端幂等控制，防止重试造成重复下单。
        request_id: requestId('mix-order'),
        // setup 阶段会为测试用户准备地址，这里直接复用。
        receiveAddressId: session.addressId,
        // items 按接口要求组织为订单项数组。
        items: items.map(function (item, index) {
          return {
            id: item.id,
            // 订单项购买数与迭代号关联，避免长期固定不变。
            count: 1 + ((index + __ITER) % 2),
          };
        }),
        // 两种支付方式交替出现，提高支付类型分支的覆盖率。
        payment_type: (__ITER + __VU) % 2 === 0 ? 1 : 2,
      }),
      {
        headers: headers,
        tags: { name: 'mix.order.add' },
      }
    );
    expectCommonSuccess('mix.order.add', createOrderResponse);
  });

  // 下单操作通常更复杂，用户停顿时间也更长。
  sleepRange(0.5, 1.5);
}

export function afterSaleFlow(data) {
  /**
   * 售后/查询流：
   * - 以查询接口为主
   * - 夹带少量购物车删除
   * - 用于模拟“下单后查看订单、管理地址、清理购物车”的后置行为
   */

  // 取一个已登录用户，用于访问订单与地址相关接口。
  var session = getSession(data, 'mix-aftersale', true);
  if (!session) {
    return;
  }
  var headers = buildHeaders(session.token);
  // 删除购物车时需要一个商品 ID，这里随机选择。
  var product = pickProduct(data);

  group('mix aftersale flow', function () {
    // 查询订单列表：
    // - 覆盖订单读取接口
    // - 模拟用户查看历史订单、确认状态、准备申请售后等动作
    var orderListResponse = http.get(data.baseUrl + '/v1/order/list', {
      headers: headers,
      tags: { name: 'mix.order.list' },
    });
    expectCommonSuccess('mix.order.list', orderListResponse);

    // 查询地址列表：
    // - 覆盖用户地址管理接口
    // - 模拟用户下单后检查或维护收货地址
    var addressListResponse = http.get(data.baseUrl + '/v1/user/get-receive-address-list', {
      headers: headers,
      tags: { name: 'mix.user.address.list' },
    });
    expectCommonSuccess('mix.user.address.list', addressListResponse);

    // 只有约 1/3 的迭代会删购物车，避免该接口流量占比过高。
    if ((__ITER + __VU) % 3 === 0) {
      var deleteCartResponse = http.post(
        data.baseUrl + '/v1/cart/delete',
        JSON.stringify({
          // 删除随机商品，模拟用户清理不再需要的商品。
          pid: product.id,
        }),
        {
          headers: headers,
          tags: { name: 'mix.cart.delete' },
        }
      );

      // 尝试解析返回体，便于识别“业务已处理但语义不是完全成功”的情况。
      var deleteResult = safeJson(deleteCartResponse);
      // 删除不存在商品时，业务层可能返回 500；这里将其视作可接受结果，
      // 避免把幂等删除的场景误判为压测失败。
      if (
        deleteCartResponse.status === 200 &&
        deleteResult &&
        (deleteResult.resultCode === 200 || deleteResult.resultCode === 500)
      ) {
        // 这里直接 return 当前 group 回调，相当于结束本次删除分支，
        // 不再走 expectCommonSuccess 的严格校验。
        return;
      }

      // 对于其他返回情况，仍按正常成功标准校验，确保真正异常不会被吞掉。
      expectCommonSuccess('mix.cart.delete', deleteCartResponse);
    }
  });

  // 查询类操作停顿介于浏览和下单之间。
  sleepRange(0.3, 1.0);
}
