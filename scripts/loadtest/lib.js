import http from 'k6/http';
import { check, fail, sleep } from 'k6';

export var BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8888';
export var USERNAME = __ENV.USERNAME || '';
export var PASSWORD = __ENV.PASSWORD || '123456';

export var USERNAME_PREFIX = __ENV.USERNAME_PREFIX || 'load_user_';
export var USER_ID_START = Number(__ENV.USER_ID_START || '1');
export var USER_ID_END = Number(__ENV.USER_ID_END || '60000');
export var USER_PAD_WIDTH = Number(__ENV.USER_PAD_WIDTH || '6');
export var USER_ROTATION_STEP = Number(__ENV.USER_ROTATION_STEP || '17');
export var USER_ROTATION_WINDOW = Number(__ENV.USER_ROTATION_WINDOW || '50');
export var LOGIN_RETRY_MAX = Number(__ENV.LOGIN_RETRY_MAX || '3');
export var LOGIN_RETRY_SLEEP = Number(__ENV.LOGIN_RETRY_SLEEP || '1');
export var FAIL_ON_SESSION_ERROR = String(__ENV.FAIL_ON_SESSION_ERROR || 'false') === 'true';

export var DEFAULT_PRODUCT_ID = Number(__ENV.PRODUCT_ID || '1001');
export var DEFAULT_PRODUCT_PRICE = Number(__ENV.PRODUCT_PRICE || '19900');
export var PRODUCT_POOL_SIZE = Number(__ENV.PRODUCT_POOL_SIZE || '60');
export var PRODUCT_ID_MIN = Number(__ENV.PRODUCT_ID_MIN || '0');
export var PRODUCT_ID_MAX = Number(__ENV.PRODUCT_ID_MAX || '0');

var sessionCache = {};

function nowMs() {
  return new Date().getTime();
}

export function randInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

export function sleepRange(minSeconds, maxSeconds) {
  sleep(minSeconds + Math.random() * (maxSeconds - minSeconds));
}

export function buildHeaders(token) {
  var headers = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers.Authorization = 'Bearer ' + token;
  }
  return headers;
}

export function safeJson(response) {
  try {
    return response.json();
  } catch (e) {
    return null;
  }
}

function truncate(text, maxLen) {
  if (!text) {
    return '';
  }
  if (text.length <= maxLen) {
    return text;
  }
  return text.slice(0, maxLen) + '...';
}

export function expectCommonSuccess(stepName, response) {
  var body = safeJson(response);
  var ok = check(response, {
    [stepName + ' status is 200']: function (r) {
      return r.status === 200;
    },
    [stepName + ' resultCode is 200']: function () {
      return body && body.resultCode === 200;
    },
  });

  if (!ok) {
    console.error(
      '[' +
        stepName +
        '] failed, status=' +
        response.status +
        ', body=' +
        truncate(response.body, 400)
    );
  }

  return {
    ok: ok,
    body: body,
    data: body ? body.data : null,
  };
}

export function pickRandom(list, fallbackValue) {
  if (!list || !list.length) {
    return fallbackValue;
  }
  return list[randInt(0, list.length - 1)];
}

export function requestId(prefix) {
  return (
    prefix +
    '-' +
    nowMs() +
    '-vu' +
    __VU +
    '-iter' +
    __ITER +
    '-' +
    randInt(1000, 9999)
  );
}

function padNumber(value, width) {
  var raw = String(value);
  while (raw.length < width) {
    raw = '0' + raw;
  }
  return raw;
}

function hashString(text) {
  var hash = 0;
  var source = String(text || '');
  for (var i = 0; i < source.length; i++) {
    hash = (hash * 31 + source.charCodeAt(i)) % 2147483647;
  }
  return hash;
}

function uniqueNumbers(list) {
  var seen = {};
  var result = [];
  for (var i = 0; i < list.length; i++) {
    var value = Number(list[i]);
    if (!value || seen[value]) {
      continue;
    }
    seen[value] = true;
    result.push(value);
  }
  return result;
}

function envProductIds() {
  if (!__ENV.PRODUCT_IDS) {
    return [];
  }

  var parts = __ENV.PRODUCT_IDS.split(',');
  var ids = [];
  for (var i = 0; i < parts.length; i++) {
    var value = Number(String(parts[i]).trim());
    if (value > 0) {
      ids.push(value);
    }
  }
  return uniqueNumbers(ids);
}

function sampleProductIdsFromRange(minId, maxId, count) {
  if (minId <= 0 || maxId <= 0 || maxId < minId) {
    return [];
  }

  var total = maxId - minId + 1;
  var size = Math.min(count, total);
  var seen = {};
  var ids = [];

  while (ids.length < size) {
    var candidate = randInt(minId, maxId);
    if (!seen[candidate]) {
      seen[candidate] = true;
      ids.push(candidate);
    }
  }
  return ids;
}

function collectHomeProductIds(homeData) {
  var ids = [];
  var sections = [
    homeData ? homeData.carousel_list : [],
    homeData ? homeData.new_goodses : [],
    homeData ? homeData.hot_goodses : [],
  ];

  for (var i = 0; i < sections.length; i++) {
    var section = sections[i] || [];
    for (var j = 0; j < section.length; j++) {
      var item = section[j];
      if (item && item.goods_id) {
        ids.push(Number(item.goods_id));
      }
    }
  }

  return uniqueNumbers(ids);
}

export function fetchHomeCatalog(baseUrl) {
  var response = http.get(baseUrl + '/v1/home/index-infos');
  var parsed = expectCommonSuccess('home.indexInfos', response);
  if (!parsed.ok || !parsed.data) {
    return [];
  }
  return collectHomeProductIds(parsed.data);
}

export function fetchProductDetail(baseUrl, productId) {
  var response = http.get(baseUrl + '/v1/product/detail/' + productId);
  var parsed = expectCommonSuccess('product.detail.' + productId, response);
  if (!parsed.ok || !parsed.data || !parsed.data.product) {
    return null;
  }
  return parsed.data.product;
}

function hasSingleUserOverride() {
  return !!USERNAME;
}

function accountCount() {
  if (hasSingleUserOverride()) {
    return 1;
  }
  return Math.max(1, USER_ID_END - USER_ID_START + 1);
}

function accountUsernameByPosition(position) {
  if (hasSingleUserOverride()) {
    return USERNAME;
  }

  var count = accountCount();
  var normalized = ((position % count) + count) % count;
  var userId = USER_ID_START + normalized;
  return USERNAME_PREFIX + padNumber(userId, USER_PAD_WIDTH);
}

export function login(baseUrl, username) {
  var account = username || USERNAME;

  for (var attempt = 1; attempt <= LOGIN_RETRY_MAX; attempt++) {
    var response = http.post(
      baseUrl + '/v1/user/login',
      JSON.stringify({
        username: account,
        password: PASSWORD,
      }),
      {
        headers: buildHeaders(),
        tags: { name: 'user.login' },
      }
    );

    var parsed = expectCommonSuccess('user.login.' + account + '.attempt' + attempt, response);
    if (parsed.ok && parsed.data && parsed.data.accessToken) {
      return parsed.data.accessToken;
    }

    if (attempt < LOGIN_RETRY_MAX) {
      sleep(LOGIN_RETRY_SLEEP * attempt);
    }
  }

  if (FAIL_ON_SESSION_ERROR) {
    fail('登录失败，请检查压测账号、密码与服务状态');
  }

  console.error('[user.login.' + account + '] exhausted retries, skip this iteration');
  return '';
}

function fetchAddressList(baseUrl, token) {
  var response = http.get(baseUrl + '/v1/user/get-receive-address-list', {
    headers: buildHeaders(token),
    tags: { name: 'user.address.list' },
  });
  var parsed = expectCommonSuccess('user.address.list', response);
  if (!parsed.ok) {
    return [];
  }

  // k6/JS 对超大整数（例如雪花 ID）会丢精度。
  // 地址 ID 被当成 number 解析后会四舍五入，后续下单就会带着错误的 receiveAddressId。
  // 这里直接从原始 body 做一次轻量规范化，把超大 id 包成字符串再解析，避免精度损失。
  var raw = response && response.body ? String(response.body) : '';
  if (raw) {
    try {
      var normalized = raw
        .replace(/"id":(\d{16,})/g, '"id":"$1"')
        .replace(/"Id":(\d{16,})/g, '"Id":"$1"');
      var body = JSON.parse(normalized);
      var data = body ? body.data : null;
      if (data && Array.isArray(data.list)) {
        return data.list;
      }
      if (data && Array.isArray(data.List)) {
        return data.List;
      }
    } catch (e) {
      // ignore and fallback to parsed.data below
    }
  }

  if (!parsed.data) {
    return [];
  }
  if (Array.isArray(parsed.data.list)) {
    return parsed.data.list;
  }
  if (Array.isArray(parsed.data.List)) {
    return parsed.data.List;
  }
  return [];
}

function createAddress(baseUrl, token, username) {
  var suffix = String(randInt(100, 999));
  var response = http.post(
    baseUrl + '/v1/user/add-receive-address',
    JSON.stringify({
      name: __ENV.RECEIVER_NAME || ('压测用户-' + (username || 'default')),
      phone: '139' + padNumber(randInt(0, 99999999), 8),
      is_default: 1,
      province: __ENV.RECEIVER_PROVINCE || '上海市',
      city: __ENV.RECEIVER_CITY || '上海市',
      region: __ENV.RECEIVER_REGION || '浦东新区',
      detail_address: (__ENV.RECEIVER_DETAIL || '世纪大道') + suffix + '号',
    }),
    {
      headers: buildHeaders(token),
      tags: { name: 'user.address.add' },
    }
  );

  expectCommonSuccess('user.address.add', response);
}

export function ensureAddress(baseUrl, token, username) {
  if (__ENV.ADDRESS_ID) {
    return String(__ENV.ADDRESS_ID);
  }

  var addresses = fetchAddressList(baseUrl, token);
  if (!addresses.length) {
    createAddress(baseUrl, token, username);
    addresses = fetchAddressList(baseUrl, token);
  }

  if (!addresses.length) {
    if (FAIL_ON_SESSION_ERROR) {
      fail('未能获取收货地址，请先准备测试账号的地址数据');
    }
    console.error('[user.address.' + (username || 'unknown') + '] no address available, skip this iteration');
    return '';
  }

  var first = addresses[0];
  var addressId = first && (first.id || first.Id);
  if (!addressId) {
    if (FAIL_ON_SESSION_ERROR) {
      fail('收货地址列表返回成功，但未找到地址 ID');
    }
    console.error('[user.address.' + (username || 'unknown') + '] address id missing, skip this iteration');
    return '';
  }

  return String(addressId);
}

export function buildProductPool(baseUrl) {
  var ids = envProductIds();

  if (!ids.length && PRODUCT_ID_MIN > 0 && PRODUCT_ID_MAX >= PRODUCT_ID_MIN) {
    ids = sampleProductIdsFromRange(PRODUCT_ID_MIN, PRODUCT_ID_MAX, PRODUCT_POOL_SIZE);
  }

  if (!ids.length) {
    ids = fetchHomeCatalog(baseUrl);
  }

  if (!ids.length) {
    ids = [DEFAULT_PRODUCT_ID];
  }

  var pool = [];
  for (var i = 0; i < ids.length; i++) {
    var detail = fetchProductDetail(baseUrl, ids[i]);
    if (detail) {
      pool.push({
        id: Number(detail.id || ids[i]),
        price: Number(detail.price || DEFAULT_PRODUCT_PRICE),
        name: detail.name || 'product-' + ids[i],
      });
    }
  }

  if (!pool.length) {
    pool.push({
      id: DEFAULT_PRODUCT_ID,
      price: DEFAULT_PRODUCT_PRICE,
      name: 'fallback-product',
    });
  }

  return pool;
}

export function setupShopper() {
  var productPool = buildProductPool(BASE_URL);

  console.log(
    'loadtest setup ready: users=' +
      (hasSingleUserOverride() ? USERNAME : USERNAME_PREFIX + padNumber(USER_ID_START, USER_PAD_WIDTH) + '..' + USERNAME_PREFIX + padNumber(USER_ID_END, USER_PAD_WIDTH)) +
      ', products=' +
      productPool.length
  );

  return {
    baseUrl: BASE_URL,
    productPool: productPool,
    singleUser: hasSingleUserOverride(),
    userIdStart: USER_ID_START,
    userIdEnd: USER_ID_END,
  };
}

export function pickProduct(data) {
  var fallbackProduct = {
    id: DEFAULT_PRODUCT_ID,
    price: DEFAULT_PRODUCT_PRICE,
    name: 'fallback-product',
  };
  return pickRandom(data && data.productPool ? data.productPool : [], fallbackProduct);
}

export function pickProducts(data, count) {
  var source = data && data.productPool ? data.productPool : [];
  if (!source.length) {
    return [pickProduct(data)];
  }

  var size = Math.min(count, source.length);
  var used = {};
  var products = [];
  while (products.length < size) {
    var item = source[randInt(0, source.length - 1)];
    if (!used[item.id]) {
      used[item.id] = true;
      products.push(item);
    }
  }
  return products;
}

function resolveAccountPosition(hint, rotate) {
  if (hasSingleUserOverride()) {
    return 0;
  }

  var base = __VU - 1 + hashString(hint || 'default');
  if (!rotate) {
    return base;
  }

  var window = Math.max(1, USER_ROTATION_WINDOW);
  var rotationRound = Math.floor(__ITER / window);
  return base + rotationRound * USER_ROTATION_STEP;
}

export function getSession(data, hint, rotate) {
  var position = resolveAccountPosition(hint, rotate);
  var username = accountUsernameByPosition(position);

  if (sessionCache[username]) {
    return sessionCache[username];
  }

  var token = login((data && data.baseUrl) || BASE_URL, username);
  if (!token) {
    return null;
  }
  var addressId = ensureAddress((data && data.baseUrl) || BASE_URL, token, username);
  if (!addressId) {
    return null;
  }

  sessionCache[username] = {
    username: username,
    token: token,
    addressId: addressId,
  };
  return sessionCache[username];
}
