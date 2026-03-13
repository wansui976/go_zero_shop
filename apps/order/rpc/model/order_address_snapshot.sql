CREATE TABLE `order_address_snapshot` (
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