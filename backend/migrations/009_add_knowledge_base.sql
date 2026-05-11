-- ============================================================
-- AutoTestFlow RAG 知识库模块
-- 数据库: MySQL 8.0+
-- ============================================================

SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

USE auto_test_flow;

CREATE TABLE IF NOT EXISTS `knowledge_bases` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '关联项目ID，强绑定',
  `name` VARCHAR(200) NOT NULL,
  `description` TEXT,
  `status` TINYINT DEFAULT 1 COMMENT '0=disabled 1=active',
  `chunk_size` INT DEFAULT 500,
  `chunk_overlap` INT DEFAULT 50,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX `idx_project_id` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `knowledge_documents` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `kb_id` BIGINT UNSIGNED NOT NULL,
  `source_type` VARCHAR(50) NOT NULL COMMENT 'manual/markdown/code/url/gitlab/zentao',
  `source_path` VARCHAR(500),
  `title` VARCHAR(300),
  `content` LONGTEXT,
  `content_size` INT DEFAULT 0,
  `chunk_count` INT DEFAULT 0,
  `status` VARCHAR(20) DEFAULT 'pending' COMMENT 'pending/parsing/indexed/failed',
  `error_msg` TEXT,
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX `idx_kb_id` (`kb_id`),
  INDEX `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `knowledge_chunks` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `doc_id` BIGINT UNSIGNED NOT NULL,
  `chunk_index` INT NOT NULL COMMENT 'chunk顺序',
  `chunk_text` TEXT NOT NULL,
  `metadata` JSON COMMENT '位置/行号/标签等',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  INDEX `idx_doc_id` (`doc_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `chunk_relations` (
  `id` BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  `source_chunk_id` BIGINT UNSIGNED NOT NULL,
  `target_chunk_id` BIGINT UNSIGNED NOT NULL,
  `relation_type` VARCHAR(30) NOT NULL COMMENT 'similar/reference/tag',
  `score` DECIMAL(5,4) COMMENT '相似度/权重',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY `uk_source_target_type` (`source_chunk_id`, `target_chunk_id`, `relation_type`),
  INDEX `idx_source` (`source_chunk_id`),
  INDEX `idx_target` (`target_chunk_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 兼容已写入的旧配置：category=knowledge_base，key=knowledge_base.xxx。
-- 正确格式应为 category=knowledge_base，key=xxx。
INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'enabled', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.enabled'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'vector_store.type', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.vector_store.type'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'vector_store.host', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.vector_store.host'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'vector_store.port', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.vector_store.port'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'vector_store.collection', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.vector_store.collection'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'embedding.model', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.embedding.model'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'embedding.dimension', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.embedding.dimension'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'embedding.batch_size', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.embedding.batch_size'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'chunk_size', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.chunk_size'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'chunk_overlap', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.chunk_overlap'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'top_k', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.top_k'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`)
SELECT 'knowledge_base', 'similarity_threshold', `value`, `encrypted`, `description`
FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` = 'knowledge_base.similarity_threshold'
ON DUPLICATE KEY UPDATE `value` = VALUES(`value`), `encrypted` = VALUES(`encrypted`), `description` = VALUES(`description`);

INSERT INTO `system_setting` (`category`, `key`, `value`, `encrypted`, `description`) VALUES
  ('knowledge_base', 'enabled', 'false', 0, '是否启用 RAG 知识库'),
  ('knowledge_base', 'vector_store.type', 'milvus', 0, '向量存储类型'),
  ('knowledge_base', 'vector_store.host', 'milvus-standalone', 0, 'Milvus 地址'),
  ('knowledge_base', 'vector_store.port', '19530', 0, 'Milvus 端口'),
  ('knowledge_base', 'vector_store.collection', 'autotestflow_knowledge', 0, 'Milvus Collection'),
  ('knowledge_base', 'embedding.provider', 'openai_compatible', 0, 'Embedding 服务类型'),
  ('knowledge_base', 'embedding.api_key', '', 1, 'Embedding API Key'),
  ('knowledge_base', 'embedding.base_url', 'https://api.openai.com/v1', 0, 'Embedding OpenAI 兼容 Base URL'),
  ('knowledge_base', 'embedding.model', 'text-embedding-3-small', 0, 'Embedding 模型名'),
  ('knowledge_base', 'embedding.dimension', '1536', 0, 'Embedding 向量维度'),
  ('knowledge_base', 'embedding.batch_size', '16', 0, 'Embedding 批大小'),
  ('knowledge_base', 'chunk_size', '500', 0, '默认 chunk 大小'),
  ('knowledge_base', 'chunk_overlap', '50', 0, '默认 chunk 重叠'),
  ('knowledge_base', 'top_k', '5', 0, '默认检索数量'),
  ('knowledge_base', 'similarity_threshold', '0.75', 0, '相似度阈值')
ON DUPLICATE KEY UPDATE `description` = VALUES(`description`);

DELETE FROM `system_setting`
WHERE `category` = 'knowledge_base' AND `key` LIKE 'knowledge_base.%';

INSERT INTO `permission` (`code`, `name`, `module`) VALUES
  ('knowledge:list', '查看知识库', 'knowledge'),
  ('knowledge:manage', '管理知识库', 'knowledge')
ON DUPLICATE KEY UPDATE `name` = VALUES(`name`), `module` = VALUES(`module`);

INSERT IGNORE INTO `role_permission` (`role_id`, `permission_id`)
SELECT r.id, p.id FROM `role` r JOIN `permission` p
WHERE r.code = 'admin' AND p.code IN ('knowledge:list', 'knowledge:manage');

INSERT IGNORE INTO `role_permission` (`role_id`, `permission_id`)
SELECT r.id, p.id FROM `role` r JOIN `permission` p
WHERE r.code = 'test_lead' AND p.code IN ('knowledge:list', 'knowledge:manage');

INSERT IGNORE INTO `role_permission` (`role_id`, `permission_id`)
SELECT r.id, p.id FROM `role` r JOIN `permission` p
WHERE r.code IN ('tester', 'dev_lead', 'viewer') AND p.code = 'knowledge:list';
