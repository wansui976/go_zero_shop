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
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<1500'],
    checks: ['rate>0.95'],
  },
  scenarios: {
    browse: {
      executor: 'ramping-arrival-rate',
      exec: 'browseFlow',
      startRate: Number(__ENV.BROWSE_START_RATE || '5'),
      timeUnit: '1s',
      preAllocatedVUs: Number(__ENV.BROWSE_PRE_VUS || '20'),
      maxVUs: Number(__ENV.BROWSE_MAX_VUS || '80'),
      stages: [
        { target: Number(__ENV.BROWSE_WARM_RATE || '10'), duration: __ENV.BROWSE_WARM_DURATION || '1m' },
        { target: Number(__ENV.BROWSE_STEADY_RATE || '20'), duration: __ENV.BROWSE_STEADY_DURATION || '3m' },
        { target: Number(__ENV.BROWSE_COOL_RATE || '8'), duration: __ENV.BROWSE_COOL_DURATION || '1m' },
      ],
    },
    shopper: {
      executor: 'constant-vus',
      exec: 'shopperFlow',
      vus: Number(__ENV.SHOPPER_VUS || '5'),
      duration: __ENV.SHOPPER_DURATION || '5m',
      startTime: __ENV.SHOPPER_START_TIME || '10s',
    },
    checkout: {
      executor: 'constant-arrival-rate',
      exec: 'checkoutFlow',
      rate: Number(__ENV.CHECKOUT_RATE || '1'),
      timeUnit: '1s',
      duration: __ENV.CHECKOUT_DURATION || '4m',
      startTime: __ENV.CHECKOUT_START_TIME || '20s',
      preAllocatedVUs: Number(__ENV.CHECKOUT_PRE_VUS || '10'),
      maxVUs: Number(__ENV.CHECKOUT_MAX_VUS || '40'),
    },
  },
};

export function setup() {
  return setupShopper();
}

export function browseFlow(data) {
  var product = pickProduct(data);

  group('daily browse flow', function () {
    var homeResponse = http.get(data.baseUrl + '/v1/home/index-infos', {
      tags: { name: 'home.indexInfos' },
    });
    expectCommonSuccess('daily.home.indexInfos', homeResponse);

    var detailResponse = http.get(data.baseUrl + '/v1/product/detail/' + product.id, {
      tags: { name: 'product.detail' },
    });
    expectCommonSuccess('daily.product.detail', detailResponse);

    if ((__ITER + __VU) % 3 === 0) {
      var secondDetail = http.get(data.baseUrl + '/v1/product/detail/' + product.id, {
        tags: { name: 'product.detail.repeat' },
      });
      expectCommonSuccess('daily.product.detail.repeat', secondDetail);
    }
  });

  sleepRange(0.5, 2.0);
}

export function shopperFlow(data) {
  var product = pickProduct(data);
  var session = getSession(data, 'daily-shopper', true);
  if (!session) {
    return;
  }
  var tokenHeaders = buildHeaders(session.token);

  group('daily shopper flow', function () {
    var infoResponse = http.get(data.baseUrl + '/v1/user/info', {
      headers: tokenHeaders,
      tags: { name: 'user.info' },
    });
    expectCommonSuccess('daily.user.info', infoResponse);

    var addressResponse = http.get(data.baseUrl + '/v1/user/get-receive-address-list', {
      headers: tokenHeaders,
      tags: { name: 'user.address.list' },
    });
    expectCommonSuccess('daily.user.address.list', addressResponse);

    var addCartResponse = http.post(
      data.baseUrl + '/v1/cart/add',
      JSON.stringify({
        pid: product.id,
        quantity: 1,
        price: product.price,
      }),
      {
        headers: tokenHeaders,
        tags: { name: 'cart.add' },
      }
    );
    expectCommonSuccess('daily.cart.add', addCartResponse);

    var listCartResponse = http.get(data.baseUrl + '/v1/cart/list', {
      headers: tokenHeaders,
      tags: { name: 'cart.list' },
    });
    expectCommonSuccess('daily.cart.list', listCartResponse);

    if ((__ITER + __VU) % 2 === 0) {
      var updateCartResponse = http.post(
        data.baseUrl + '/v1/cart/update',
        JSON.stringify({
          pid: product.id,
          quantity: 2,
        }),
        {
          headers: tokenHeaders,
          tags: { name: 'cart.update' },
        }
      );
      expectCommonSuccess('daily.cart.update', updateCartResponse);
    }

    if ((__ITER + __VU) % 4 === 0) {
      var deleteCartResponse = http.post(
        data.baseUrl + '/v1/cart/delete',
        JSON.stringify({
          pid: product.id,
        }),
        {
          headers: tokenHeaders,
          tags: { name: 'cart.delete' },
        }
      );
      expectCommonSuccess('daily.cart.delete', deleteCartResponse);
    }
  });

  sleepRange(1.0, 3.0);
}

export function checkoutFlow(data) {
  var session = getSession(data, 'daily-checkout', true);
  if (!session) {
    return;
  }
  var orderProducts = pickProducts(data, randInt(1, 2));

  group('daily checkout flow', function () {
    var createOrderResponse = http.post(
      data.baseUrl + '/v1/order/add',
      JSON.stringify({
        request_id: requestId('daily-order'),
        receiveAddressId: session.addressId,
        items: orderProducts.map(function (product, index) {
          return {
            id: product.id,
            count: index === 0 ? 1 : 2,
          };
        }),
        payment_type: (__ITER + __VU) % 2 === 0 ? 1 : 2,
      }),
      {
        headers: buildHeaders(session.token),
        tags: { name: 'order.add' },
      }
    );
    expectCommonSuccess('daily.order.add', createOrderResponse);

    var listOrderResponse = http.get(data.baseUrl + '/v1/order/list', {
      headers: buildHeaders(session.token),
      tags: { name: 'order.list' },
    });
    expectCommonSuccess('daily.order.list', listOrderResponse);
  });

  sleepRange(1.0, 2.5);
}
