CREATE TABLE `order_request_mapping` (
  `id`          BIGINT       NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `request_id`  VARCHAR(128) NOT NULL                COMMENT '前端幂等请求ID',
  `order_id`    VARCHAR(64)  NOT NULL                COMMENT '订单唯一编号',
  `create_time` BIGINT       NOT NULL                COMMENT '创建时间（毫秒）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_order_id` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='幂等请求ID与订单ID映射表';
