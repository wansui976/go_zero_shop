DROP TABLE IF EXISTS `category`;
CREATE TABLE `category` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '分类ID',
  `parent_id` BIGINT DEFAULT 0 COMMENT '父分类ID，0表示顶级分类',
  `name` VARCHAR(64) NOT NULL COMMENT '分类名称',
  `level` INT DEFAULT 1 COMMENT '分类层级（1: 一级分类，2: 二级分类...）',
  `sort` INT DEFAULT 0 COMMENT '排序值',
  `icon_url` VARCHAR(255) DEFAULT NULL COMMENT '分类图标URL',
  `keywords` VARCHAR(255) DEFAULT NULL COMMENT '关键词，用于搜索',
  `description` TEXT COMMENT '分类描述',
  `status` TINYINT DEFAULT 1 COMMENT '状态：0-禁用，1-启用',
  `create_time` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `update_time` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_level` (`level`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品分类表';