CREATE DATABASE IF NOT EXISTS `reply` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
USE `reply`;

CREATE TABLE IF NOT EXISTS `comment` (
  `id` bigint NOT NULL COMMENT '评论ID',
  `business` varchar(32) NOT NULL DEFAULT '' COMMENT '业务类型，如 product',
  `target_id` bigint NOT NULL COMMENT '业务对象ID',
  `reply_user_id` bigint NOT NULL COMMENT '评论用户ID',
  `be_reply_user_id` bigint NOT NULL DEFAULT 0 COMMENT '被回复用户ID',
  `parent_id` bigint NOT NULL DEFAULT 0 COMMENT '父评论ID，一级评论为0',
  `content` varchar(1000) NOT NULL DEFAULT '' COMMENT '评论内容',
  `image` varchar(2048) NOT NULL DEFAULT '' COMMENT '评论图片，多个时逗号分隔',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '状态：1正常，0删除',
  `create_time` bigint NOT NULL COMMENT '创建时间（毫秒）',
  `update_time` bigint NOT NULL COMMENT '更新时间（毫秒）',
  PRIMARY KEY (`id`),
  KEY `idx_business_target_status_id` (`business`, `target_id`, `status`, `id` DESC),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_reply_user_id` (`reply_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评论回复表';
