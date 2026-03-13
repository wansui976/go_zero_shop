-- 订单状态变更日志
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

