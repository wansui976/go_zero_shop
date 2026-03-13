DROP TABLE IF EXISTS `product`;
CREATE TABLE `product` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '商品ID (SPU)',
  `name` VARCHAR(200) NOT NULL COMMENT '商品名称',
  `brief` VARCHAR(255) DEFAULT NULL COMMENT '商品简介',
  `keywords` VARCHAR(255) DEFAULT NULL COMMENT '关键词，用于搜索',
  `image_url` VARCHAR(255) DEFAULT NULL COMMENT '主图URL',

  `category_id` BIGINT DEFAULT NULL COMMENT '主分类ID',
  `category_name` VARCHAR(100) DEFAULT NULL COMMENT '主分类名称',
  `category_ids` VARCHAR(255) DEFAULT NULL COMMENT '所有分类ID集合（逗号分隔）',
  `brand_id` BIGINT DEFAULT NULL COMMENT '品牌ID',
  `brand_name` VARCHAR(100) DEFAULT NULL COMMENT '品牌名称',

  `price` INT NOT NULL DEFAULT 0 COMMENT '售价',
  `stock` BIGINT DEFAULT 0 COMMENT '当前库存',
  `low_stock` INT DEFAULT 0 COMMENT '库存预警值',
  `sales` BIGINT DEFAULT 0 COMMENT '销量',

  `unit` VARCHAR(20) DEFAULT NULL COMMENT '单位（件 / 台 / 箱）',
  `weight` FLOAT DEFAULT 0 COMMENT '重量(kg)',

  `detail_title` VARCHAR(255) DEFAULT NULL COMMENT '详情标题',
  `detail_desc` TEXT COMMENT '详情描述',
  `detail_html` MEDIUMTEXT COMMENT '富文本详情内容（HTML）',

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
  KEY `idx_create_time` (`create_time`),
  FULLTEXT KEY `idx_keywords` (`keywords`, `brief`, `name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品信息表（SPU）';