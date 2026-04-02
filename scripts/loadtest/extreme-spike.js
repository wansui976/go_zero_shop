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
    http_req_failed: ['rate<0.10'],
    http_req_duration: ['p(95)<3000'],
    checks: ['rate>0.90'],
  },
  scenarios: {
    hot_product_read: {
      executor: 'ramping-arrival-rate',
      exec: 'hotReadFlow',
      startRate: Number(__ENV.HOT_READ_START_RATE || '20'),
      timeUnit: '1s',
      preAllocatedVUs: Number(__ENV.HOT_READ_PRE_VUS || '50'),
      maxVUs: Number(__ENV.HOT_READ_MAX_VUS || '300'),
      stages: [
        { target: Number(__ENV.HOT_READ_PEAK_1 || '50'), duration: __ENV.HOT_READ_RAMP_1 || '30s' },
        { target: Number(__ENV.HOT_READ_PEAK_2 || '120'), duration: __ENV.HOT_READ_RAMP_2 || '45s' },
        { target: Number(__ENV.HOT_READ_PEAK_3 || '200'), duration: __ENV.HOT_READ_RAMP_3 || '1m' },
        { target: Number(__ENV.HOT_READ_RECOVER || '30'), duration: __ENV.HOT_READ_RAMP_DOWN || '45s' },
      ],
    },
    hot_checkout_write: {
      executor: 'ramping-arrival-rate',
      exec: 'hotCheckoutFlow',
      startRate: Number(__ENV.HOT_CHECKOUT_START_RATE || '5'),
      timeUnit: '1s',
      preAllocatedVUs: Number(__ENV.HOT_CHECKOUT_PRE_VUS || '30'),
      maxVUs: Number(__ENV.HOT_CHECKOUT_MAX_VUS || '150'),
      stages: [
        { target: Number(__ENV.HOT_CHECKOUT_PEAK_1 || '10'), duration: __ENV.HOT_CHECKOUT_RAMP_1 || '30s' },
        { target: Number(__ENV.HOT_CHECKOUT_PEAK_2 || '25'), duration: __ENV.HOT_CHECKOUT_RAMP_2 || '45s' },
        { target: Number(__ENV.HOT_CHECKOUT_PEAK_3 || '40'), duration: __ENV.HOT_CHECKOUT_RAMP_3 || '1m' },
        { target: Number(__ENV.HOT_CHECKOUT_RECOVER || '5'), duration: __ENV.HOT_CHECKOUT_RAMP_DOWN || '45s' },
      ],
      startTime: __ENV.HOT_CHECKOUT_START_TIME || '15s',
    },
  },
};

export function setup() {
  return setupShopper();
}

export function hotReadFlow(data) {
  var product = data.productPool && data.productPool.length ? data.productPool[0] : pickProduct(data);

  group('extreme hot read flow', function () {
    var response1 = http.get(data.baseUrl + '/v1/product/detail/' + product.id, {
      tags: { name: 'product.detail.hot' },
    });
    expectCommonSuccess('extreme.product.detail.1', response1);

    var response2 = http.get(data.baseUrl + '/v1/product/detail/' + product.id, {
      tags: { name: 'product.detail.hot' },
    });
    expectCommonSuccess('extreme.product.detail.2', response2);

    if ((__ITER + __VU) % 2 === 0) {
      var homeResponse = http.get(data.baseUrl + '/v1/home/index-infos', {
        tags: { name: 'home.indexInfos.hot' },
      });
      expectCommonSuccess('extreme.home.indexInfos', homeResponse);
    }
  });

  sleepRange(0.1, 0.5);
}

export function hotCheckoutFlow(data) {
  var product = data.productPool && data.productPool.length ? data.productPool[0] : pickProduct(data);
  var session = getSession(data, 'hot-checkout', true);
  if (!session) {
    return;
  }
  var headers = buildHeaders(session.token);
  var products = [product].concat(pickProducts(data, randInt(0, 1) + 1).filter(function (item) {
    return item.id !== product.id;
  }).slice(0, 1));

  group('extreme hot checkout flow', function () {
    var cartAddResponse = http.post(
      data.baseUrl + '/v1/cart/add',
      JSON.stringify({
        pid: product.id,
        quantity: 1,
        price: product.price,
      }),
      {
        headers: headers,
        tags: { name: 'cart.add.hot' },
      }
    );
    expectCommonSuccess('extreme.cart.add', cartAddResponse);

    var createOrderResponse = http.post(
      data.baseUrl + '/v1/order/add',
      JSON.stringify({
        request_id: requestId('extreme-order'),
        receiveAddressId: session.addressId,
        items: products.map(function (item, index) {
          return {
            id: item.id,
            count: index === 0 ? randInt(1, 2) : 1,
          };
        }),
        payment_type: (__ITER + __VU) % 2 === 0 ? 1 : 2,
      }),
      {
        headers: headers,
        tags: { name: 'order.add.hot' },
      }
    );
    expectCommonSuccess('extreme.order.add', createOrderResponse);
  });

  sleepRange(0.1, 0.4);
}
