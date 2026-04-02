CREATE DATABASE IF NOT EXISTS user DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS product DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS orders DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS cart DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS pay DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS reply DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE user;
CREATE TABLE IF NOT EXISTS `user` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  `username` varchar(50) NOT NULL DEFAULT '' COMMENT '用户名',
  `password` varchar(50) NOT NULL DEFAULT '' COMMENT '用户密码，MD5加密',
  `phone` varchar(20) NOT NULL DEFAULT '' COMMENT '手机号',
  `question` varchar(100) NOT NULL DEFAULT '' COMMENT '找回密码问题',
  `answer` varchar(100) NOT NULL DEFAULT '' COMMENT '找回密码答案',
  `introduce_sign` varchar(100) NOT NULL DEFAULT '' COMMENT '个性签名',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uniq_phone` (`phone`),
  UNIQUE KEY `uniq_username` (`username`),
  KEY `ix_update_time` (`update_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';

CREATE TABLE IF NOT EXISTS `user_collection` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '收藏Id',
  `uid` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '用户id',
  `product_id` bigint(20) unsigned NOT NULL DEFAULT '0' COMMENT '商品id',
  `is_delete` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否删除',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '数据创建时间',
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '数据更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `UN_collection_uid_product_id` (`uid`,`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户收藏表';

CREATE TABLE IF NOT EXISTS `user_receive_address` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `uid` bigint(20) NOT NULL DEFAULT '0' COMMENT '用户id',
  `name` varchar(64) NOT NULL DEFAULT '' COMMENT '收货人名称',
  `phone` varchar(20) NOT NULL DEFAULT '' COMMENT '手机号',
  `is_default` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否为默认地址',
  `province` varchar(100) NOT NULL DEFAULT '' COMMENT '省份/直辖市',
  `city` varchar(100) NOT NULL DEFAULT '' COMMENT '城市',
  `region` varchar(100) NOT NULL DEFAULT '' COMMENT '区',
  `detail_address` varchar(128) NOT NULL DEFAULT '' COMMENT '详细地址(街道)',
  `is_delete` tinyint(1) unsigned NOT NULL DEFAULT '0' COMMENT '是否删除',
  `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '数据创建时间',
  `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '数据更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_uid` (`uid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户收货地址表';

INSERT IGNORE INTO `user` (`id`, `username`, `password`, `phone`, `question`, `answer`, `introduce_sign`)
VALUES (1, 'testuser', '123456', '13800000001', '', '', 'docker compose seeded user');

INSERT IGNORE INTO `user_receive_address` (`id`, `uid`, `name`, `phone`, `is_default`, `province`, `city`, `region`, `detail_address`, `is_delete`)
VALUES (1, 1, 'Tester', '13800000001', 1, 'Shanghai', 'Shanghai', 'Pudong', 'No.1 Demo Road', 0);

USE product;
CREATE TABLE IF NOT EXISTS `category` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '分类ID',
  `parent_id` BIGINT DEFAULT 0 COMMENT '父分类ID，0表示顶级分类',
  `name` VARCHAR(64) NOT NULL COMMENT '分类名称',
  `level` INT DEFAULT 1 COMMENT '分类层级',
  `sort` INT DEFAULT 0 COMMENT '排序值',
  `icon_url` VARCHAR(255) DEFAULT NULL COMMENT '分类图标URL',
  `keywords` VARCHAR(255) DEFAULT NULL COMMENT '关键词',
  `description` TEXT COMMENT '分类描述',
  `status` TINYINT DEFAULT 1 COMMENT '状态：0-禁用，1-启用',
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_level` (`level`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品分类表';

CREATE TABLE IF NOT EXISTS `product` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '商品ID (SPU)',
  `name` VARCHAR(200) NOT NULL COMMENT '商品名称',
  `brief` VARCHAR(255) DEFAULT NULL COMMENT '商品简介',
  `keywords` VARCHAR(255) DEFAULT NULL COMMENT '关键词',
  `image_url` VARCHAR(255) DEFAULT NULL COMMENT '主图URL',
  `category_id` BIGINT DEFAULT NULL COMMENT '主分类ID',
  `category_name` VARCHAR(100) DEFAULT NULL COMMENT '主分类名称',
  `category_ids` VARCHAR(255) DEFAULT NULL COMMENT '所有分类ID集合',
  `brand_id` BIGINT DEFAULT NULL COMMENT '品牌ID',
  `brand_name` VARCHAR(100) DEFAULT NULL COMMENT '品牌名称',
  `price` INT NOT NULL DEFAULT 0 COMMENT '售价',
  `stock` BIGINT DEFAULT 0 COMMENT '当前库存',
  `low_stock` INT DEFAULT 0 COMMENT '库存预警值',
  `sales` BIGINT DEFAULT 0 COMMENT '销量',
  `unit` VARCHAR(20) DEFAULT NULL COMMENT '单位',
  `weight` FLOAT DEFAULT 0 COMMENT '重量',
  `detail_title` VARCHAR(255) DEFAULT NULL COMMENT '详情标题',
  `detail_desc` TEXT COMMENT '详情描述',
  `detail_html` MEDIUMTEXT COMMENT '富文本详情内容',
  `sort` INT DEFAULT 0 COMMENT '排序值',
  `new_status_sort` INT DEFAULT 0 COMMENT '新品排序',
  `recommend_status_sort` INT DEFAULT 0 COMMENT '推荐排序',
  `status` TINYINT DEFAULT 1 COMMENT '商品状态：0-下架，1-上架',
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_category_id` (`category_id`),
  KEY `idx_brand_id` (`brand_id`),
  KEY `idx_status` (`status`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品信息表';

CREATE TABLE IF NOT EXISTS `product_operation` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '操作记录ID',
  `product_id` BIGINT NOT NULL COMMENT '商品ID',
  `operator_id` BIGINT DEFAULT NULL COMMENT '操作人ID',
  `operator_name` VARCHAR(100) DEFAULT NULL COMMENT '操作人名称',
  `operation_type` VARCHAR(50) NOT NULL COMMENT '操作类型',
  `before_data` TEXT COMMENT '操作前数据',
  `after_data` TEXT COMMENT '操作后数据',
  `remark` VARCHAR(255) DEFAULT NULL COMMENT '备注',
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',
  PRIMARY KEY (`id`),
  KEY `idx_product_id` (`product_id`),
  KEY `idx_operation_type` (`operation_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品操作日志表';

INSERT IGNORE INTO `category` (`id`, `parent_id`, `name`, `level`, `sort`, `status`)
VALUES (10, 0, '首页轮播', 1, 10, 1),
       (11, 0, '新品专区', 1, 20, 1),
       (12, 0, '热门推荐', 1, 30, 1);

INSERT IGNORE INTO `product` (`id`, `name`, `brief`, `keywords`, `image_url`, `category_id`, `category_name`, `brand_id`, `brand_name`, `price`, `stock`, `sales`, `detail_desc`, `detail_html`, `new_status_sort`, `recommend_status_sort`, `status`)
VALUES
  (1001, '轮播测试商品 A', '首页轮播测试商品 A', '轮播,测试,首页', 'https://example.com/p/1001.jpg', 10, '首页轮播', 1, 'OpenAI Demo', 19900, 100, 20, '用于首页轮播接口联调', '<p>轮播商品 A 详情</p>', 10, 10, 1),
  (1002, '轮播测试商品 B', '首页轮播测试商品 B', '轮播,测试,首页', 'https://example.com/p/1002.jpg', 10, '首页轮播', 1, 'OpenAI Demo', 29900, 100, 15, '用于首页轮播接口联调', '<p>轮播商品 B 详情</p>', 9, 9, 1),
  (1003, '轮播测试商品 C', '首页轮播测试商品 C', '轮播,测试,首页', 'https://example.com/p/1003.jpg', 10, '首页轮播', 1, 'OpenAI Demo', 39900, 100, 12, '用于首页轮播接口联调', '<p>轮播商品 C 详情</p>', 8, 8, 1),
  (1101, '新品测试商品 A', '新品测试商品 A', '新品,测试', 'https://example.com/p/1101.jpg', 11, '新品专区', 2, 'Launch Lab', 45900, 100, 8, '用于首页新品接口联调', '<p>新品商品 A 详情</p>', 20, 6, 1),
  (1102, '新品测试商品 B', '新品测试商品 B', '新品,测试', 'https://example.com/p/1102.jpg', 11, '新品专区', 2, 'Launch Lab', 55900, 100, 6, '用于首页新品接口联调', '<p>新品商品 B 详情</p>', 19, 5, 1),
  (1201, '热门测试商品 A', '热门测试商品 A', '热门,测试', 'https://example.com/p/1201.jpg', 12, '热门推荐', 3, 'Hot Pick', 69900, 100, 88, '用于首页热门接口联调', '<p>热门商品 A 详情</p>', 7, 30, 1),
  (1202, '热门测试商品 B', '热门测试商品 B', '热门,测试', 'https://example.com/p/1202.jpg', 12, '热门推荐', 3, 'Hot Pick', 79900, 100, 66, '用于首页热门接口联调', '<p>热门商品 B 详情</p>', 6, 29, 1);

USE cart;
CREATE TABLE IF NOT EXISTS `cart` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '购物车ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `product_id` bigint NOT NULL COMMENT '商品ID',
  `price` INT NOT NULL COMMENT '加入时商品售价',
  `quantity` int NOT NULL DEFAULT 1 COMMENT '商品数量',
  `selected` tinyint NOT NULL DEFAULT 1 COMMENT '是否选中',
  `delete_status` tinyint NOT NULL DEFAULT 0 COMMENT '删除状态',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_product_specs` (`user_id`,`product_id`,`delete_status`),
  KEY `idx_product_id` (`product_id`),
  KEY `idx_user_selected` (`user_id`,`selected`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='购物车表';

USE orders;
CREATE TABLE IF NOT EXISTS `orders` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `order_id` VARCHAR(64) NOT NULL COMMENT '订单唯一编号',
  `user_id` BIGINT NOT NULL COMMENT '用户 ID',
  `total_price` INT NOT NULL DEFAULT 0 COMMENT '订单总金额（分）',
  `payment_type` TINYINT NOT NULL DEFAULT 0 COMMENT '支付方式',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '订单状态',
  `receiver_name` VARCHAR(64) NOT NULL COMMENT '收件人姓名',
  `receiver_phone` VARCHAR(32) NOT NULL COMMENT '收件人手机号',
  `receiver_address` VARCHAR(255) NOT NULL COMMENT '收货地址',
  `gid` VARCHAR(128) DEFAULT NULL COMMENT '全局事务 ID（DTM 用）',
  `create_time` BIGINT NOT NULL COMMENT '创建时间（毫秒）',
  `update_time` BIGINT NOT NULL COMMENT '更新时间（毫秒）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_id` (`order_id`),
  KEY `idx_user_status_ctime` (`user_id`, `status`, `create_time` DESC),
  KEY `idx_gid` (`gid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单主表';

CREATE TABLE IF NOT EXISTS `order_items` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_id` VARCHAR(64) NOT NULL COMMENT '订单编号',
  `product_id` BIGINT NOT NULL COMMENT '商品 ID',
  `quantity` INT NOT NULL COMMENT '数量',
  `unit_price` INT NOT NULL DEFAULT 0 COMMENT '下单时单价',
  `total_price` INT NOT NULL DEFAULT 0 COMMENT '该商品总价',
  `create_time` BIGINT NOT NULL,
  `update_time` BIGINT NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_order_id` (`order_id`),
  KEY `idx_product_id` (`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单商品明细表';

CREATE TABLE IF NOT EXISTS `order_address_snapshot` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_id` VARCHAR(64) NOT NULL,
  `user_id` BIGINT NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `phone` VARCHAR(32) NOT NULL,
  `province` VARCHAR(64) DEFAULT '',
  `city` VARCHAR(64) DEFAULT '',
  `district` VARCHAR(64) DEFAULT '',
  `detail` VARCHAR(255) NOT NULL,
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_order_id` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单地址快照';

CREATE TABLE IF NOT EXISTS `order_status_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_id` BIGINT NOT NULL,
  `old_status` TINYINT NOT NULL,
  `new_status` TINYINT NOT NULL,
  `operator` VARCHAR(128) DEFAULT NULL,
  `reason` VARCHAR(255) DEFAULT NULL,
  `create_time` BIGINT NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_order_id` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单状态变更日志';
