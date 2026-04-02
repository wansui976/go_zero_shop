import http from 'k6/http';
import { group } from 'k6';
import {
  buildHeaders,
  expectCommonSuccess,
  getSession,
  pickProduct,
  pickProducts,
  requestId,
  randInt,
  setupShopper,
  sleepRange,
} from './lib.js';

export var options = {
  thresholds: {
    http_req_failed: ['rate<0.03'],
    http_req_duration: ['p(95)<1800'],
    checks: ['rate>0.97'],
  },
  scenarios: {
    browse_soak: {
      executor: 'constant-arrival-rate',
      exec: 'browseSoakFlow',
      rate: Number(__ENV.SOAK_BROWSE_RATE || '8'),
      timeUnit: '1s',
      duration: __ENV.SOAK_DURATION || '2h',
      preAllocatedVUs: Number(__ENV.SOAK_BROWSE_PRE_VUS || '30'),
      maxVUs: Number(__ENV.SOAK_BROWSE_MAX_VUS || '120'),
    },
    shopper_soak: {
      executor: 'constant-arrival-rate',
      exec: 'shopperSoakFlow',
      rate: Number(__ENV.SOAK_SHOPPER_RATE || '2'),
      timeUnit: '1s',
      duration: __ENV.SOAK_DURATION || '2h',
      preAllocatedVUs: Number(__ENV.SOAK_SHOPPER_PRE_VUS || '20'),
      maxVUs: Number(__ENV.SOAK_SHOPPER_MAX_VUS || '80'),
      startTime: __ENV.SOAK_SHOPPER_START_TIME || '15s',
    },
    order_soak: {
      executor: 'constant-arrival-rate',
      exec: 'orderSoakFlow',
      rate: Number(__ENV.SOAK_ORDER_RATE || '1'),
      timeUnit: '1s',
      duration: __ENV.SOAK_DURATION || '2h',
      preAllocatedVUs: Number(__ENV.SOAK_ORDER_PRE_VUS || '10'),
      maxVUs: Number(__ENV.SOAK_ORDER_MAX_VUS || '60'),
      startTime: __ENV.SOAK_ORDER_START_TIME || '30s',
    },
  },
};

export function setup() {
  return setupShopper();
}

export function browseSoakFlow(data) {
  var product = pickProduct(data);

  group('soak browse flow', function () {
    var homeResponse = http.get(data.baseUrl + '/v1/home/index-infos', {
      tags: { name: 'home.indexInfos.soak' },
    });
    expectCommonSuccess('soak.home.indexInfos', homeResponse);

    var detailResponse = http.get(data.baseUrl + '/v1/product/detail/' + product.id, {
      tags: { name: 'product.detail.soak' },
    });
    expectCommonSuccess('soak.product.detail', detailResponse);
  });

  sleepRange(1.0, 4.0);
}

export function shopperSoakFlow(data) {
  var product = pickProduct(data);
  var session = getSession(data, 'soak-shopper', true);
  if (!session) {
    return;
  }
  var headers = buildHeaders(session.token);

  group('soak shopper flow', function () {
    var infoResponse = http.get(data.baseUrl + '/v1/user/info', {
      headers: headers,
      tags: { name: 'user.info.soak' },
    });
    expectCommonSuccess('soak.user.info', infoResponse);

    var addCartResponse = http.post(
      data.baseUrl + '/v1/cart/add',
      JSON.stringify({
        pid: product.id,
        quantity: 1,
        price: product.price,
      }),
      {
        headers: headers,
        tags: { name: 'cart.add.soak' },
      }
    );
    expectCommonSuccess('soak.cart.add', addCartResponse);

    var cartListResponse = http.get(data.baseUrl + '/v1/cart/list', {
      headers: headers,
      tags: { name: 'cart.list.soak' },
    });
    expectCommonSuccess('soak.cart.list', cartListResponse);
  });

  sleepRange(2.0, 6.0);
}

export function orderSoakFlow(data) {
  var session = getSession(data, 'soak-order', true);
  if (!session) {
    return;
  }
  var headers = buildHeaders(session.token);
  var orderProducts = pickProducts(data, randInt(1, 3));

  group('soak order flow', function () {
    var createOrderResponse = http.post(
      data.baseUrl + '/v1/order/add',
      JSON.stringify({
        request_id: requestId('soak-order'),
        receiveAddressId: session.addressId,
        items: orderProducts.map(function (product, index) {
          return {
            id: product.id,
            count: 1 + ((index + __ITER) % 2),
          };
        }),
        payment_type: (__ITER + __VU) % 2 === 0 ? 1 : 2,
      }),
      {
        headers: headers,
        tags: { name: 'order.add.soak' },
      }
    );
    expectCommonSuccess('soak.order.add', createOrderResponse);

    if ((__ITER + __VU) % 3 === 0) {
      var listOrderResponse = http.get(data.baseUrl + '/v1/order/list', {
        headers: headers,
        tags: { name: 'order.list.soak' },
      });
      expectCommonSuccess('soak.order.list', listOrderResponse);
    }
  });

  sleepRange(3.0, 8.0);
}
