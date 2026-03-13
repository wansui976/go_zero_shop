CREATE TABLE `cart` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '购物车ID',
  `user_id` bigint NOT NULL COMMENT '用户ID',
  `product_id` bigint NOT NULL COMMENT '商品ID',
  `price` INT NOT NULL COMMENT '加入时商品售价',
  `quantity` int NOT NULL DEFAULT 1 COMMENT '商品数量',
  `selected` tinyint NOT NULL DEFAULT 1 COMMENT '是否选中（0=否，1=是）',
  `delete_status` tinyint NOT NULL DEFAULT 0 COMMENT '删除状态（0=正常，1=删除）',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_product_specs` (`user_id`,`product_id`,`delete_status`),
  KEY `idx_product_id` (`product_id`),
  KEY `idx_user_selected` (`user_id`,`selected`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='购物车表';

