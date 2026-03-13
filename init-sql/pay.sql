-- =======================================
-- Payment Service Database Schema
-- =======================================

-- 创建支付数据库
CREATE DATABASE IF NOT EXISTS pay DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE pay;

-- 支付记录表
CREATE TABLE IF NOT EXISTS `payment` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `payment_id` VARCHAR(64) NOT NULL COMMENT '支付单号',
  `order_id` VARCHAR(64) NOT NULL COMMENT '订单号',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `amount` BIGINT NOT NULL COMMENT '支付金额（单位：分）',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '支付状态：0=待支付,1=已支付,2=支付失败,3=已取消',
  `payment_type` TINYINT NOT NULL DEFAULT 0 COMMENT '支付方式：1=微信支付,2=支付宝',
  `transaction_id` VARCHAR(64) DEFAULT NULL COMMENT '第三方交易号',
  `create_time` BIGINT NOT NULL COMMENT '创建时间戳',
  `pay_time` BIGINT DEFAULT 0 COMMENT '支付时间戳',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_payment_id` (`payment_id`),
  KEY `idx_order_id` (`order_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付记录表';

-- 退款记录表
CREATE TABLE IF NOT EXISTS `refund` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `refund_id` VARCHAR(64) NOT NULL COMMENT '退款单号',
  `order_id` VARCHAR(64) NOT NULL COMMENT '订单号',
  `payment_id` VARCHAR(64) NOT NULL COMMENT '支付单号',
  `refund_amount` BIGINT NOT NULL COMMENT '退款金额（单位：分）',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '退款状态：0=待退款,1=退款成功,2=退款失败',
  `reason` VARCHAR(256) DEFAULT NULL COMMENT '退款原因',
  `create_time` BIGINT NOT NULL COMMENT '创建时间戳',
  `refund_time` BIGINT DEFAULT 0 COMMENT '退款时间戳',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refund_id` (`refund_id`),
  KEY `idx_order_id` (`order_id`),
  KEY `idx_payment_id` (`payment_id`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='退款记录表';
