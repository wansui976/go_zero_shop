DROP TABLE IF EXISTS `product_operation`;
CREATE TABLE `product_operation` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '操作记录ID',
  `product_id` BIGINT NOT NULL COMMENT '商品ID',
  `operator_id` BIGINT DEFAULT NULL COMMENT '操作人ID',
  `operator_name` VARCHAR(100) DEFAULT NULL COMMENT '操作人名称',
  `operation_type` VARCHAR(50) NOT NULL COMMENT '操作类型（create/update/online/offline/adjust_stock等）',
  `before_data` TEXT COMMENT '操作前数据（JSON快照）',
  `after_data` TEXT COMMENT '操作后数据（JSON快照）',
  `remark` VARCHAR(255) DEFAULT NULL COMMENT '备注',
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '操作时间',
  PRIMARY KEY (`id`),
  KEY `idx_product_id` (`product_id`),
  KEY `idx_operation_type` (`operation_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品操作日志表';