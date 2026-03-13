CREATE TABLE `orders` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `order_id` VARCHAR(64) NOT NULL COMMENT '订单唯一编号（UUID）',
  `user_id` BIGINT NOT NULL COMMENT '用户 ID',

  -- 价格相关
  `total_price` INT NOT NULL DEFAULT 0 COMMENT '订单总金额（分）',
  `payment_type` TINYINT NOT NULL DEFAULT 0 COMMENT '支付方式（1微信 2支付宝等）',
  

  -- 状态映射：0=已取消,1=待支付,2=已支付,3=待发货,4=已发货,5=待收货,6=已完成,7=退款中,8=已退款
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '订单状态',

  -- 地址快照
  `receiver_name`   VARCHAR(64) NOT NULL COMMENT '收件人姓名',
  `receiver_phone`  VARCHAR(32) NOT NULL COMMENT '收件人手机号',
  `receiver_address` VARCHAR(255) NOT NULL COMMENT '收货地址',

  -- 分布式事务
  `gid` VARCHAR(128) DEFAULT NULL COMMENT '全局事务 ID（DTM 用）',

  `create_time` BIGINT NOT NULL COMMENT '创建时间（毫秒）',
  `update_time` BIGINT NOT NULL COMMENT '更新时间（毫秒）',

  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_id` (`order_id`),
  KEY `idx_user_status_ctime` (`user_id`, `status`, `create_time` DESC),
  KEY `idx_gid` (`gid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单主表';

