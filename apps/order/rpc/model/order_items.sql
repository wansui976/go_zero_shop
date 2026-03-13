CREATE TABLE `order_items` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_id` VARCHAR(64) NOT NULL COMMENT '订单编号',
  `product_id` BIGINT NOT NULL COMMENT '商品 ID',
  `quantity` INT NOT NULL COMMENT '数量',
  `unit_price` INT NOT NULL DEFAULT 0  COMMENT '下单时单价',
  `total_price`INT NOT NULL DEFAULT 0 COMMENT '该商品总价（=单价*数量）',

  `create_time` BIGINT NOT NULL,
  `update_time` BIGINT NOT NULL,

  PRIMARY KEY (`id`),
  KEY `idx_order_id` (`order_id`),
  KEY `idx_product_id` (`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单商品明细表';

