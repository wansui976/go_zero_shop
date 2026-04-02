CREATE DATABASE IF NOT EXISTS dtm DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS dtm_barrier DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE dtm;

CREATE TABLE IF NOT EXISTS dtm.trans_global (
  id bigint(22) NOT NULL AUTO_INCREMENT,
  gid varchar(128) NOT NULL,
  trans_type varchar(45) DEFAULT NULL,
  status varchar(45) DEFAULT NULL,
  query_prepared varchar(1024) DEFAULT NULL,
  protocol varchar(45) DEFAULT NULL,
  rollback_time datetime DEFAULT NULL,
  rollback_reason varchar(1024) DEFAULT NULL,
  options varchar(1024) DEFAULT NULL,
  ext_data varchar(1024) DEFAULT NULL,
  custom_data varchar(1024) DEFAULT NULL,
  result text DEFAULT NULL,
  create_time datetime DEFAULT NULL,
  update_time datetime DEFAULT NULL,
  finish_time datetime DEFAULT NULL,
  next_cron_interval int DEFAULT NULL,
  next_cron_time datetime DEFAULT NULL,
  owner varchar(64) DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_gid (gid),
  KEY idx_next_cron_time (next_cron_time),
  KEY idx_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS dtm.trans_branch_op (
  id bigint(22) NOT NULL AUTO_INCREMENT,
  gid varchar(128) NOT NULL,
  branch_id varchar(128) NOT NULL,
  op varchar(45) NOT NULL,
  url varchar(1024) DEFAULT NULL,
  data text DEFAULT NULL,
  bin_data mediumblob DEFAULT NULL,
  branch_headers text DEFAULT NULL,
  status varchar(45) DEFAULT NULL,
  finish_time datetime DEFAULT NULL,
  rollback_time datetime DEFAULT NULL,
  create_time datetime DEFAULT NULL,
  update_time datetime DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_gid_branch_op (gid, branch_id, op),
  KEY idx_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS dtm.kv (
  id bigint(22) NOT NULL AUTO_INCREMENT,
  cat varchar(128) NOT NULL,
  `k` varchar(128) NOT NULL,
  `v` text DEFAULT NULL,
  create_time datetime DEFAULT NULL,
  update_time datetime DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_cat_k (cat, `k`),
  KEY idx_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

USE dtm_barrier;

CREATE TABLE IF NOT EXISTS dtm_barrier.barrier (
  id bigint(22) NOT NULL AUTO_INCREMENT,
  trans_type varchar(45) DEFAULT NULL,
  gid varchar(128) DEFAULT NULL,
  branch_id varchar(128) DEFAULT NULL,
  op varchar(45) DEFAULT NULL,
  barrier_id varchar(45) DEFAULT NULL,
  reason varchar(45) DEFAULT NULL,
  create_time datetime DEFAULT NULL,
  update_time datetime DEFAULT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_barrier (gid, branch_id, op, barrier_id),
  KEY idx_update_time (update_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
